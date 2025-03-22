package internal

import (
	"fmt"
	"time"
)

type ForEachFlow struct {
	Type         string `yaml:"type"`
	Collection   string `yaml:"collection"`
	As           string `yaml:"as"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
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

	progressMode, _ := flowMap["progress_mode"].(bool)

	return &ForEachFlow{
		Type:         "foreach",
		Collection:   collection,
		As:           as,
		ProgressMode: progressMode,
	}, nil
}

func ExecuteForEach(op Operation, forEach *ForEachFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	startTime := time.Now()

	originalProgressMode := ctx.ProgressMode
	useProgressMode := forEach.ProgressMode
	if useProgressMode {
		ctx.ProgressMode = true
	}

	collectionExpr, err := renderTemplate(forEach.Collection, ctx.templateVars())
	if err != nil {
		ctx.ProgressMode = originalProgressMode
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

		updateDurationVars(ctx, startTime)

		if debug {
			fmt.Printf("Foreach iteration %d/%d: %s = %s (elapsed: %s)\n", idx+1, len(items), forEach.As, item, ctx.Vars["duration_fmt"])
		}

		ctx.Vars[forEach.As] = item
		ctx.Vars["iteration"] = idx + 1

		for _, subOp := range op.Operations {
			if subOp.Condition != "" {
				condResult, err := evaluateCondition(subOp.Condition, ctx)
				if err != nil {
					ctx.ProgressMode = originalProgressMode
					if useProgressMode {
						fmt.Println()
					}

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
				ctx.ProgressMode = originalProgressMode
				if useProgressMode {
					fmt.Println()
				}

				return shouldExit, err
			}

			if shouldExit || subOp.Exit {
				if debug {
					fmt.Printf("Exiting entire recipe due to exit flag in '%s'\n", subOp.Name)
				}

				ctx.ProgressMode = originalProgressMode
				if useProgressMode {
					fmt.Println()
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

	updateDurationVars(ctx, startTime)

	delete(ctx.Vars, forEach.As)
	delete(ctx.Vars, "iteration")

	if op.ID != "" {
		ctx.OperationResults[op.ID] = true
	}

	ctx.ProgressMode = originalProgressMode
	if useProgressMode {
		fmt.Println()
	}

	return false, nil
}
