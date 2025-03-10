package main

import (
	"fmt"
	"strconv"
)

type ForFlow struct {
	Type     string `yaml:"type"`
	Count    string `yaml:"count"`
	Variable string `yaml:"variable"`
}

func (f *ForFlow) GetType() string {
	return f.Type
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

	return &ForFlow{
		Type:     "for",
		Count:    count,
		Variable: variable,
	}, nil
}

func ExecuteFor(op Operation, forFlow *ForFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) error, debug bool) error {
	countStr, err := renderTemplate(forFlow.Count, ctx.templateVars())
	if err != nil {
		return fmt.Errorf("failed to render count template: %w", err)
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return fmt.Errorf("invalid count value after rendering: %s", countStr)
	}

	if debug {
		fmt.Printf("For loop with %d iterations\n", count)
	}

	for i := 0; i < count; i++ {
		if debug {
			fmt.Printf("For iteration %d/%d: %s = %d\n", i+1, count, forFlow.Variable, i)
		}

		ctx.Vars[forFlow.Variable] = i
		ctx.Vars["iteration"] = i + 1

		for _, subOp := range op.Operations {
			if err := executeOp(subOp, depth+1); err != nil {
				return err
			}
		}
	}

	delete(ctx.Vars, forFlow.Variable)
	delete(ctx.Vars, "iteration")

	if op.ID != "" {
		ctx.OperationResults[op.ID] = true
	}

	return nil
}
