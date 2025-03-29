package internal

import (
	"fmt"
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
func LoadComponents(sources []string, debug bool) error {
	for _, source := range sources {
		file, err := loadFile(source)
		if err != nil {
			if debug {
				fmt.Printf("Warning: Failed to load components from %s: %v\n", source, err)
			}
			continue
		}

		for _, component := range file.Components {
			if component.ID == "" {
				if debug {
					fmt.Printf("Warning: Skipping component without ID in %s\n", source)
				}
				continue
			}
			globalComponentRegistry.Register(component)
		}
	}

	return nil
}

// ExpandComponentReferences recursively replaces component references with their operations
func ExpandComponentReferences(operations []Operation, opMap map[string]Operation, debug bool) ([]Operation, error) {
	var expanded []Operation

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

			if debug {
				fmt.Printf("Expanding component reference: %s\n", op.Uses)
			}

			clonedOps := make([]Operation, len(component.Operations))
			for i, compOp := range component.Operations {
				clonedOps[i] = compOp
				applyOperationProperties(&clonedOps[i], op)

				if clonedOps[i].ID != "" {
					opMap[clonedOps[i].ID] = clonedOps[i]
				}
			}

			componentOps, err := ExpandComponentReferences(clonedOps, opMap, debug)
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

			expanded = append(expanded, componentOps...)
		} else {
			if len(op.Operations) > 0 {
				expandedSubOps, err := ExpandComponentReferences(op.Operations, opMap, debug)
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
