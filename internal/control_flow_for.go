package internal

import (
	"fmt"
	"strconv"
)

// ForFlow defines the structure for a for loop control flow
type ForFlow struct {
	Type            string              `yaml:"type"`
	Count           string              `yaml:"count"`
	Variable        string              `yaml:"variable"`
	ProgressMode    bool                `yaml:"progress_mode,omitempty"`
	ProgressBar     bool                `yaml:"progress_bar,omitempty"`
	ProgressBarOpts *ProgressBarOptions `yaml:"progress_bar_options,omitempty"`
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
	progressBar, _ := flowMap["progress_bar"].(bool)

	var progressBarOpts *ProgressBarOptions
	if optsVal, ok := flowMap["progress_bar_options"]; ok {
		if optsMap, ok := optsVal.(map[string]interface{}); ok {
			progressBarOpts = ParseProgressBarOptions(optsMap)
		}
	}

	return &ForFlow{
		Type:            "for",
		Count:           count,
		Variable:        variable,
		ProgressMode:    progressMode,
		ProgressBar:     progressBar,
		ProgressBarOpts: progressBarOpts,
	}, nil
}

// ExecuteFor runs a for loop with the given parameters
func ExecuteFor(op Operation, forFlow *ForFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	loopCtx := ctx.pushLoopContext("for", depth)
	defer ctx.popLoopContext()

	originalMode := setupProgressMode(ctx, forFlow.ProgressMode)
	defer func() {
		ctx.ProgressMode = originalMode
		if forFlow.ProgressMode && !forFlow.ProgressBar {
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

	var progressBar *ProgressBar
	if forFlow.ProgressBar {
		description := ""
		if forFlow.ProgressBarOpts != nil && forFlow.ProgressBarOpts.Description != "" {
			description = forFlow.ProgressBarOpts.Description
		}
		progressBar = CreateProgressBar(count, description, forFlow.ProgressBarOpts)
	}

	for i := 0; i < count; i++ {
		ctx.updateLoopDuration()
		ctx.Vars[forFlow.Variable] = i
		ctx.Vars["iteration"] = i + 1

		if debug {
			fmt.Printf("For iteration %d/%d: %s = %d (elapsed: %s)\n",
				i+1, count, forFlow.Variable, i, formatDuration(loopCtx.Duration))
		}

		if progressBar != nil && forFlow.ProgressBarOpts != nil && forFlow.ProgressBarOpts.MessageTemplate != "" {
			rendered, err := renderTemplate(forFlow.ProgressBarOpts.MessageTemplate, ctx.templateVars())
			if err == nil {
				progressBar.Update(rendered)
			}
		}

		exit, breakLoop := executeLoopOperations(op.Operations, ctx, depth, executeOp, debug)

		if progressBar != nil {
			progressBar.Increment()
		}

		if exit {
			if progressBar != nil {
				progressBar.Complete()
			}
			return true, nil
		}
		if breakLoop {
			break
		}
	}

	if progressBar != nil {
		progressBar.Complete()
	}

	ctx.updateLoopDuration()
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
