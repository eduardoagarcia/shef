package internal

import (
	"fmt"
	"time"
)

type WhileFlow struct {
	Type         string `yaml:"type"`
	Condition    string `yaml:"condition"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
}

func (w *WhileFlow) GetType() string {
	return w.Type
}

func (op *Operation) GetWhileFlow() (*WhileFlow, error) {
	if op.ControlFlow == nil {
		return nil, fmt.Errorf("operation does not have control_flow")
	}

	flowMap, ok := op.ControlFlow.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid control_flow structure")
	}

	typeVal, ok := flowMap["type"].(string)
	if !ok || typeVal != "while" {
		return nil, fmt.Errorf("not a while control flow")
	}

	condition, ok := flowMap["condition"].(string)
	if !ok {
		return nil, fmt.Errorf("while requires a 'condition' field")
	}

	progressMode, _ := flowMap["progress_mode"].(bool)

	return &WhileFlow{
		Type:         "while",
		Condition:    condition,
		ProgressMode: progressMode,
	}, nil
}

func ExecuteWhile(op Operation, whileFlow *WhileFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	startTime := time.Now()

	originalProgressMode := ctx.ProgressMode
	useProgressMode := whileFlow.ProgressMode
	if useProgressMode {
		ctx.ProgressMode = true
	}

	iterations := 0
	breakLoop := false

	for {
		if breakLoop {
			break
		}

		updateDurationVars(ctx, startTime)

		renderedCondition, err := renderTemplate(whileFlow.Condition, ctx.templateVars())
		if err != nil {
			ctx.ProgressMode = originalProgressMode
			if useProgressMode {
				fmt.Println()
			}

			return false, fmt.Errorf("failed to render while condition template: %w", err)
		}

		conditionResult, err := evaluateCondition(renderedCondition, ctx)
		if err != nil {
			ctx.ProgressMode = originalProgressMode
			if useProgressMode {
				fmt.Println()
			}

			return false, fmt.Errorf("failed to evaluate while condition '%s': %w", renderedCondition, err)
		}

		if !conditionResult {
			break
		}

		iterations++
		if debug {
			fmt.Printf("While iteration %d, condition: %s (elapsed: %s)\n", iterations, whileFlow.Condition, ctx.Vars["duration_fmt"])
		}
		ctx.Vars["iteration"] = iterations

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
					fmt.Printf("Breaking out of while loop due to break flag in '%s'\n", subOp.Name)
				}
				breakLoop = true
				break
			}
		}
	}

	updateDurationVars(ctx, startTime)

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
