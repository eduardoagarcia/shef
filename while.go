package main

import "fmt"

type WhileFlow struct {
	Type      string `yaml:"type"`
	Condition string `yaml:"condition"`
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

	return &WhileFlow{
		Type:      "while",
		Condition: condition,
	}, nil
}

func ExecuteWhile(op Operation, whileFlow *WhileFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) error {
	maxIterations := 1000
	iterations := 0

	for {
		renderedCondition, err := renderTemplate(whileFlow.Condition, ctx.templateVars())
		if err != nil {
			return fmt.Errorf("failed to render while condition template: %w", err)
		}

		conditionResult, err := evaluateCondition(renderedCondition, ctx)
		if err != nil {
			return fmt.Errorf("failed to evaluate while condition '%s': %w", renderedCondition, err)
		}

		if !conditionResult {
			break
		}

		iterations++
		if iterations > maxIterations {
			return fmt.Errorf("maximum while loop iterations exceeded (%d)", maxIterations)
		}

		if debug {
			fmt.Printf("While iteration %d, condition: %s\n", iterations, whileFlow.Condition)
		}

		ctx.Vars["iteration"] = iterations

		for _, subOp := range op.Operations {
			shouldExit, err := executeOp(subOp, depth+1)
			if err != nil {
				return err
			}

			if shouldExit {
				if debug {
					fmt.Printf("Exiting while loop early due to exit flag\n")
				}
				return nil
			}
		}
	}

	delete(ctx.Vars, "iteration")

	if op.ID != "" {
		ctx.OperationResults[op.ID] = true
	}

	return nil
}
