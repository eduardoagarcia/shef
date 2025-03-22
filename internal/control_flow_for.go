package internal

import (
	"fmt"
	"strconv"
	"time"
)

type ForFlow struct {
	Type         string `yaml:"type"`
	Count        string `yaml:"count"`
	Variable     string `yaml:"variable"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
}

func (op *Operation) GetForFlow() (*ForFlow, error) {
	if op.ControlFlow == nil {
		return nil, fmt.Errorf("operation does not have control_flow")
	}

	flowMap, ok := op.ControlFlow.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid control_flow structure")
	}

	typeVal, ok := flowMap["type"].(string)
	if !ok || typeVal != "for" {
		return nil, fmt.Errorf("not a for control flow")
	}

	countVal, ok := flowMap["count"]
	if !ok {
		return nil, fmt.Errorf("for requires a 'count' field")
	}
	count := fmt.Sprintf("%v", countVal)

	variable, ok := flowMap["variable"].(string)
	if !ok || variable == "" {
		variable = "i"
	}

	progressMode, _ := flowMap["progress_mode"].(bool)

	return &ForFlow{
		Type:         "for",
		Count:        count,
		Variable:     variable,
		ProgressMode: progressMode,
	}, nil
}

func ExecuteFor(op Operation, forFlow *ForFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	startTime := time.Now()

	originalProgressMode := ctx.ProgressMode
	useProgressMode := forFlow.ProgressMode
	if useProgressMode {
		ctx.ProgressMode = true
	}

	countStr, err := renderTemplate(forFlow.Count, ctx.templateVars())
	if err != nil {
		ctx.ProgressMode = originalProgressMode
		return false, fmt.Errorf("failed to render count template: %w", err)
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		ctx.ProgressMode = originalProgressMode
		return false, fmt.Errorf("invalid count value after rendering: %s", countStr)
	}

	if debug {
		fmt.Printf("For loop with %d iterations\n", count)
	}

	breakLoop := false
	for i := 0; i < count && !breakLoop; i++ {
		updateDurationVars(ctx, startTime)

		if debug {
			fmt.Printf("For iteration %d/%d: %s = %d (elapsed: %s)\n", i+1, count, forFlow.Variable, i, ctx.Vars["duration_fmt"])
		}

		ctx.Vars[forFlow.Variable] = i
		ctx.Vars["iteration"] = i + 1

		for _, subOp := range op.Operations {
			if subOp.Condition != "" {
				condResult, err := evaluateCondition(subOp.Condition, ctx)
				if err != nil {
					ctx.ProgressMode = originalProgressMode
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
					fmt.Printf("Breaking out of for loop due to break flag in '%s'\n", subOp.Name)
				}
				breakLoop = true
				break
			}
		}
	}

	updateDurationVars(ctx, startTime)

	delete(ctx.Vars, forFlow.Variable)
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
