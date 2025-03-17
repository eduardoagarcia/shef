package main

import "fmt"

type ForEachFlow struct {
	Type       string `yaml:"type"`
	Collection string `yaml:"collection"`
	As         string `yaml:"as"`
}

func (f *ForEachFlow) GetType() string {
	return f.Type
}

func (op *Operation) GetForEachFlow() (*ForEachFlow, error) {
	if op.ControlFlow == nil {
		return nil, fmt.Errorf("operation does not have control_flow")
	}

	flowMap, ok := op.ControlFlow.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid control_flow structure")
	}

	typeVal, ok := flowMap["type"].(string)
	if !ok || typeVal != "foreach" {
		return nil, fmt.Errorf("not a foreach control flow")
	}

	collection, ok := flowMap["collection"].(string)
	if !ok {
		return nil, fmt.Errorf("foreach requires a 'collection' field")
	}

	as, ok := flowMap["as"].(string)
	if !ok {
		return nil, fmt.Errorf("foreach requires an 'as' field")
	}

	return &ForEachFlow{
		Type:       "foreach",
		Collection: collection,
		As:         as,
	}, nil
}

func ExecuteForEach(op Operation, forEach *ForEachFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	collectionExpr, err := renderTemplate(forEach.Collection, ctx.templateVars())
	if err != nil {
		return false, fmt.Errorf("failed to render collection template: %w", err)
	}

	items := parseOptionsFromOutput(collectionExpr)

	if debug {
		fmt.Printf("Foreach loop over %d items\n", len(items))
	}

	breakLoop := false
	for idx, item := range items {
		if breakLoop {
			break
		}

		if debug {
			fmt.Printf("Foreach iteration %d/%d: %s = %s\n", idx+1, len(items), forEach.As, item)
		}

		ctx.Vars[forEach.As] = item
		ctx.Vars["iteration"] = idx + 1

		for _, subOp := range op.Operations {
			if subOp.Condition != "" {
				condResult, err := evaluateCondition(subOp.Condition, ctx)
				if err != nil {
					return false, fmt.Errorf("condition evaluation failed: %w", err)
				}

				if !condResult {
					if debug {
						fmt.Printf("Skipping operation '%s' (condition not met)\n", subOp.Name)
					}
					continue
				}
			}

			shouldExit, err := executeOp(subOp, depth+1)
			if err != nil {
				return shouldExit, err
			}

			if shouldExit || subOp.Exit {
				if debug {
					fmt.Printf("Exiting entire recipe due to exit flag in '%s'\n", subOp.Name)
				}
				return true, nil
			}

			if subOp.Break {
				if debug {
					fmt.Printf("Breaking out of foreach loop due to break flag in '%s'\n", subOp.Name)
				}
				breakLoop = true
				break
			}
		}
	}

	delete(ctx.Vars, forEach.As)
	delete(ctx.Vars, "iteration")

	if op.ID != "" {
		ctx.OperationResults[op.ID] = true
	}

	return false, nil
}
