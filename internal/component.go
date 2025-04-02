package internal

import (
	"fmt"
	"regexp"
	"sync"
)

// ComponentRegistry manages component loading and retrieval
type ComponentRegistry struct {
	components map[string]Component
	mutex      sync.RWMutex
}

// Global component registry
var globalComponentRegistry = &ComponentRegistry{
	components: make(map[string]Component),
}

// Register adds a component to the registry
func (cr *ComponentRegistry) Register(component Component) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	if component.ID != "" {
		cr.components[component.ID] = component
	}
}

// Get retrieves a component by ID
func (cr *ComponentRegistry) Get(id string) (Component, bool) {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()

	component, exists := cr.components[id]
	return component, exists
}

// Clear empties the registry
func (cr *ComponentRegistry) Clear() {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	cr.components = make(map[string]Component)
}

// LoadComponents loads components from all available sources
func LoadComponents(sources []string) error {
	for _, source := range sources {
		file, err := loadFile(source)
		if err != nil {
			LogError(fmt.Sprintf("Failed to load components from %s", source), err, nil)
			continue
		}

		for _, component := range file.Components {
			if component.ID == "" {
				Log(CategoryComponent, fmt.Sprintf("Skipping component without ID in %s", source))
				continue
			}
			globalComponentRegistry.Register(component)
		}
	}

	return nil
}

// ExpandComponentReferences recursively replaces component references with their operations
func ExpandComponentReferences(operations []Operation, opMap map[string]Operation) ([]Operation, error) {
	var expanded []Operation
	componentInstances := make(map[string]int)

	for _, op := range operations {
		if op.ID != "" {
			opMap[op.ID] = op
		}
	}

	for _, op := range operations {
		if op.Uses != "" {
			component, exists := globalComponentRegistry.Get(op.Uses)
			if !exists {
				return nil, fmt.Errorf("component not found: %s", op.Uses)
			}

			componentInstances[op.Uses]++
			instanceNum := componentInstances[op.Uses]

			var instanceID string
			if op.ID != "" {
				instanceID = fmt.Sprintf("%s_%s_%d", op.Uses, op.ID, instanceNum)
			} else {
				instanceID = fmt.Sprintf("%s_%d", op.Uses, instanceNum)
			}

			Log(CategoryComponent, fmt.Sprintf("Expanding component reference: %s (instance: %s)", op.Uses, instanceID))

			var inputOps []Operation
			if op.With != nil && len(op.With) > 0 {
				Log(CategoryComponent, fmt.Sprintf("Processing component inputs for: %s (instance: %s)", op.Uses, instanceID))

				if len(component.Inputs) > 0 {
					for _, input := range component.Inputs {
						if input.Required {
							if _, exists := op.With[input.ID]; !exists {
								return nil, fmt.Errorf("required input '%s' missing for component: %s", input.ID, op.Uses)
							}
						}
					}
				}

				for name, value := range op.With {
					var inputVar string = name
					for _, input := range component.Inputs {
						if input.ID == name {
							inputVar = input.ID
							break
						}
					}

					inputOp := Operation{
						Name:    fmt.Sprintf("Set component input: %s", name),
						ID:      fmt.Sprintf("comp_%s_%s", instanceID, inputVar),
						Command: fmt.Sprintf("echo '%v'", value),
						Silent:  true,
					}
					inputOps = append(inputOps, inputOp)

					opMap[inputOp.ID] = inputOp
				}
			}

			if len(component.Inputs) > 0 {
				for _, input := range component.Inputs {
					if op.With != nil {
						if _, exists := op.With[input.ID]; exists {
							continue
						}
					}

					if input.Default != nil {
						defaultOp := Operation{
							Name:    fmt.Sprintf("Set default input: %s", input.Name),
							ID:      fmt.Sprintf("comp_%s_%s", instanceID, input.ID),
							Command: fmt.Sprintf("echo '%v'", input.Default),
							Silent:  true,
						}
						inputOps = append(inputOps, defaultOp)

						opMap[defaultOp.ID] = defaultOp
					}
				}
			}

			clonedOps := make([]Operation, len(component.Operations))
			for i, compOp := range component.Operations {
				clonedOps[i] = compOp
				applyOperationProperties(&clonedOps[i], op)

				originalID := clonedOps[i].ID

				processComponentOperationVars(&clonedOps[i], instanceID)

				if clonedOps[i].ID != "" {
					opMap[originalID] = clonedOps[i]
					namespacedID := fmt.Sprintf("%s_%s", instanceID, originalID)
					opMap[namespacedID] = clonedOps[i]
				}
			}

			componentOps, err := ExpandComponentReferences(clonedOps, opMap)
			if err != nil {
				return nil, err
			}

			if op.ID != "" && len(componentOps) > 0 {
				lastOpIndex := len(componentOps) - 1
				origID := componentOps[lastOpIndex].ID
				componentOps[lastOpIndex].ID = op.ID

				if origID != "" && origID != op.ID {
					copyOp := Operation{
						Name:    fmt.Sprintf("Copy output from %s to %s", origID, op.ID),
						Command: fmt.Sprintf("echo \"{{ .%s }}\"", origID),
						ID:      op.ID,
						Silent:  true,
					}
					componentOps = append(componentOps, copyOp)
				}
			}

			expanded = append(expanded, inputOps...)
			expanded = append(expanded, componentOps...)
		} else {
			if len(op.Operations) > 0 {
				expandedSubOps, err := ExpandComponentReferences(op.Operations, opMap)
				if err != nil {
					return nil, err
				}
				op.Operations = expandedSubOps
			}

			expanded = append(expanded, op)
		}
	}

	return expanded, nil
}

// applyOperationProperties applies specific properties from source operation to target operation
func applyOperationProperties(target *Operation, source Operation) {
	if source.Condition != "" {
		if target.Condition != "" {
			target.Condition = "(" + target.Condition + ") && (" + source.Condition + ")"
		} else {
			target.Condition = source.Condition
		}
	}

	target.Silent = target.Silent || source.Silent
	target.Break = target.Break || source.Break
	target.Exit = target.Exit || source.Exit
}

// processComponentOperationVars updates operations within components to use component-scoped variables
func processComponentOperationVars(op *Operation, instanceID string) {
	if op.Command != "" {
		op.Command = updateTemplateVarsForComponent(op.Command, instanceID)
	}

	if op.Condition != "" {
		op.Condition = updateConditionVarsForComponent(op.Condition, instanceID)
	}

	if op.Transform != "" {
		op.Transform = updateTemplateVarsForComponent(op.Transform, instanceID)
	}

	for i := range op.Prompts {
		if op.Prompts[i].Default != "" {
			op.Prompts[i].Default = updateTemplateVarsForComponent(op.Prompts[i].Default, instanceID)
		}
		if op.Prompts[i].Message != "" {
			op.Prompts[i].Message = updateTemplateVarsForComponent(op.Prompts[i].Message, instanceID)
		}
	}

	for i := range op.Operations {
		processComponentOperationVars(&op.Operations[i], instanceID)
	}
}

// updateTemplateVarsForComponent replaces template variables in a template string with component-scoped versions
func updateTemplateVarsForComponent(templateStr string, instanceID string) string {
	regexPattern := regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)
	replacement := fmt.Sprintf(`{{ .comp_%s_$1 }}`, instanceID)

	result := regexPattern.ReplaceAllString(templateStr, replacement)

	Log(CategoryComponent, fmt.Sprintf("Updated component variable references in template: %s → %s", templateStr, result))

	return result
}

// updateConditionVarsForComponent replaces variable references in conditions with component-scoped versions
func updateConditionVarsForComponent(condition string, instanceID string) string {
	regexPattern := regexp.MustCompile(`\.(\w+)\b`)
	replacement := fmt.Sprintf(`.comp_%s_$1`, instanceID)

	result := regexPattern.ReplaceAllString(condition, replacement)

	Log(CategoryComponent, fmt.Sprintf("Updated component variable references in condition: %s → %s", condition, result))

	return result
}
