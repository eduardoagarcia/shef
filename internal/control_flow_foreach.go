package internal

import (
	"fmt"
)

// ForEachFlow defines the structure for a foreach loop control flow
type ForEachFlow struct {
	Type         string `yaml:"type"`
	Collection   string `yaml:"collection"`
	As           string `yaml:"as"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
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

	return &ForEachFlow{
		Type:         "foreach",
		Collection:   collection,
		As:           as,
		ProgressMode: progressMode,
	}, nil
}

// ExecuteForEach runs a foreach loop with the given parameters
func ExecuteForEach(op Operation, forEach *ForEachFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	loopCtx := ctx.pushLoopContext("foreach", depth)
	defer ctx.popLoopContext()

	originalMode := setupProgressMode(ctx, forEach.ProgressMode)
	defer func() {
		ctx.ProgressMode = originalMode
		if forEach.ProgressMode {
			fmt.Println()
		}
	}()

	collectionExpr, err := renderTemplate(forEach.Collection, ctx.templateVars())
	if err != nil {
		return false, fmt.Errorf("failed to render collection template: %w", err)
	}

	items := parseOptionsFromOutput(collectionExpr)

	if debug {
		fmt.Printf("Foreach loop over %d items\n", len(items))
	}

	for idx, item := range items {
		ctx.updateLoopDuration()
		ctx.Vars[forEach.As] = item
		ctx.Vars["iteration"] = idx + 1

		if debug {
			fmt.Printf("Foreach iteration %d/%d: %s = %s (elapsed: %s)\n",
				idx+1, len(items), forEach.As, item, formatDuration(loopCtx.Duration))
		}

		exit, breakLoop := executeLoopOperations(op.Operations, ctx, depth, executeOp, debug)
		if exit {
			return true, nil
		}
		if breakLoop {
			break
		}
	}

	ctx.updateLoopDuration()
	cleanupLoopState(ctx, op.ID, forEach.As)

	return false, nil
}
