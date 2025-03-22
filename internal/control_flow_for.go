package internal

import (
	"fmt"
	"strconv"
	"time"
)

// ForFlow defines the structure for a for loop control flow
type ForFlow struct {
	Type         string `yaml:"type"`
	Count        string `yaml:"count"`
	Variable     string `yaml:"variable"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
}

// GetForFlow extracts for loop configuration from an operation
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

// ExecuteFor runs a for loop with the given parameters
func ExecuteFor(op Operation, forFlow *ForFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	startTime := time.Now()

	originalMode := setupProgressMode(ctx, forFlow.ProgressMode)
	defer func() {
		ctx.ProgressMode = originalMode
		if forFlow.ProgressMode {
			fmt.Println()
		}
	}()

	count, err := getIterationCount(forFlow, ctx)
	if err != nil {
		return false, err
	}

	if debug {
		fmt.Printf("For loop with %d iterations\n", count)
	}

	for i := 0; i < count; i++ {
		updateDurationVars(ctx, startTime)
		ctx.Vars[forFlow.Variable] = i
		ctx.Vars["iteration"] = i + 1

		if debug {
			fmt.Printf("For iteration %d/%d: %s = %d (elapsed: %s)\n",
				i+1, count, forFlow.Variable, i, ctx.Vars["duration_fmt"])
		}

		exit, breakLoop := executeLoopOperations(op.Operations, ctx, depth, executeOp, debug)
		if exit {
			return true, nil
		}
		if breakLoop {
			break
		}
	}

	updateDurationVars(ctx, startTime)
	cleanupLoopState(ctx, op.ID, forFlow.Variable)

	return false, nil
}

// getIterationCount resolves the number of iterations for a for loop
func getIterationCount(forFlow *ForFlow, ctx *ExecutionContext) (int, error) {
	countStr, err := renderTemplate(forFlow.Count, ctx.templateVars())
	if err != nil {
		return 0, fmt.Errorf("failed to render count template: %w", err)
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0, fmt.Errorf("invalid count value after rendering: %s", countStr)
	}

	return count, nil
}
