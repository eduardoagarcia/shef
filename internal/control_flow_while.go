package internal

import (
	"fmt"
	"time"
)

// WhileFlow defines the structure for a while loop control flow
type WhileFlow struct {
	Type         string `yaml:"type"`
	Condition    string `yaml:"condition"`
	ProgressMode bool   `yaml:"progress_mode,omitempty"`
}

// GetType returns the control flow type
func (w *WhileFlow) GetType() string {
	return w.Type
}

// GetWhileFlow extracts while loop configuration from an operation
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

// ExecuteWhile runs a while loop with the given parameters
func ExecuteWhile(op Operation, whileFlow *WhileFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	startTime := time.Now()

	originalMode := setupProgressMode(ctx, whileFlow.ProgressMode)
	defer func() {
		ctx.ProgressMode = originalMode
		if whileFlow.ProgressMode {
			fmt.Println()
		}
	}()

	iterations := 0

	for {
		updateDurationVars(ctx, startTime)

		shouldContinue, err := evaluateWhileCondition(whileFlow.Condition, ctx)
		if err != nil {
			return false, err
		}

		if !shouldContinue {
			break
		}

		iterations++
		ctx.Vars["iteration"] = iterations

		if debug {
			fmt.Printf("While iteration %d, condition: %s (elapsed: %s)\n",
				iterations, whileFlow.Condition, ctx.Vars["duration_fmt"])
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
	cleanupLoopState(ctx, op.ID, "")

	return false, nil
}

// evaluateWhileCondition renders and evaluates the while loop condition
func evaluateWhileCondition(condition string, ctx *ExecutionContext) (bool, error) {
	renderedCondition, err := renderTemplate(condition, ctx.templateVars())
	if err != nil {
		return false, fmt.Errorf("failed to render while condition template: %w", err)
	}

	conditionResult, err := evaluateCondition(renderedCondition, ctx)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate while condition '%s': %w", renderedCondition, err)
	}

	return conditionResult, nil
}
