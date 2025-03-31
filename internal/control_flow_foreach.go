package internal

import (
	"fmt"
)

// ForEachFlow defines the structure for a foreach loop control flow
type ForEachFlow struct {
	Type            string              `yaml:"type"`
	Collection      string              `yaml:"collection"`
	As              string              `yaml:"as"`
	ProgressMode    bool                `yaml:"progress_mode,omitempty"`
	ProgressBar     bool                `yaml:"progress_bar,omitempty"`
	ProgressBarOpts *ProgressBarOptions `yaml:"progress_bar_options,omitempty"`
}

// GetType returns the control flow type
func (f *ForEachFlow) GetType() string {
	return f.Type
}

// GetForEachFlow extracts foreach loop configuration from an operation
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
	progressBar, _ := flowMap["progress_bar"].(bool)

	var progressBarOpts *ProgressBarOptions
	if optsVal, ok := flowMap["progress_bar_options"]; ok {
		if optsMap, ok := optsVal.(map[string]interface{}); ok {
			progressBarOpts = ParseProgressBarOptions(optsMap)
		}
	}

	return &ForEachFlow{
		Type:            "foreach",
		Collection:      collection,
		As:              as,
		ProgressMode:    progressMode,
		ProgressBar:     progressBar,
		ProgressBarOpts: progressBarOpts,
	}, nil
}

// ExecuteForEach runs a foreach loop with the given parameters
func ExecuteForEach(op Operation, forEach *ForEachFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error)) (bool, error) {
	loopCtx := ctx.pushLoopContext("foreach", depth)
	defer ctx.popLoopContext()

	originalMode := setupProgressMode(ctx, forEach.ProgressMode)
	defer func() {
		ctx.ProgressMode = originalMode
		if forEach.ProgressMode && !forEach.ProgressBar {
			fmt.Println()
		}
	}()

	collectionExpr, err := renderTemplate(forEach.Collection, ctx.templateVars())
	if err != nil {
		return false, fmt.Errorf("failed to render collection template: %w", err)
	}

	items := parseOptionsFromOutput(collectionExpr)

	Log(CategoryLoop, fmt.Sprintf("Foreach loop over %d items", len(items)))

	var progressBar *ProgressBar
	if forEach.ProgressBar {
		description := ""
		if forEach.ProgressBarOpts != nil && forEach.ProgressBarOpts.Description != "" {
			description = forEach.ProgressBarOpts.Description
		}
		progressBar = CreateProgressBar(len(items), description, forEach.ProgressBarOpts)
	}

	for idx, item := range items {
		ctx.updateLoopDuration()
		ctx.Vars[forEach.As] = item
		ctx.Vars["iteration"] = idx + 1

		LogLoopIteration("foreach", idx+1, len(items), map[string]interface{}{
			"variable": forEach.As,
			"value":    item,
			"duration": formatDuration(loopCtx.Duration),
		})

		if progressBar != nil && forEach.ProgressBarOpts != nil && forEach.ProgressBarOpts.MessageTemplate != "" {
			rendered, err := renderTemplate(forEach.ProgressBarOpts.MessageTemplate, ctx.templateVars())
			if err == nil {
				progressBar.Update(rendered)
			}
		}

		exit, breakLoop := executeLoopOperations(op.Operations, ctx, depth, executeOp)

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
	cleanupLoopState(ctx, op.ID, forEach.As)

	return false, nil
}
