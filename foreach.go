package main

import "fmt"

type ForEachFlow struct {
	Type       string `yaml:"type"`
	Collection string `yaml:"collection"`
	As         string `yaml:"as"`
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

	return &ForEachFlow{
		Type:       "foreach",
		Collection: collection,
		As:         as,
	}, nil
}

func ExecuteForEach(op Operation, forEach *ForEachFlow, ctx *ExecutionContext, depth int, executeOp func(Operation, int) error, debug bool) error {
	collectionExpr, err := renderTemplate(forEach.Collection, ctx.templateVars())
	if err != nil {
		return fmt.Errorf("failed to render collection template: %w", err)
	}

	items := parseOptionsFromOutput(collectionExpr)

	if debug {
		fmt.Printf("Foreach loop over %d items\n", len(items))
	}

	for idx, item := range items {
		if debug {
			fmt.Printf("Foreach iteration %d/%d: %s = %s\n", idx+1, len(items), forEach.As, item)
		}

		ctx.Vars[forEach.As] = item

		if len(op.Operations) > 0 {
			if err := executeOp(op.Operations[0], depth+1); err != nil {
				return err
			}
		}
	}

	delete(ctx.Vars, forEach.As)

	if op.ID != "" {
		ctx.OperationResults[op.ID] = true
	}

	return nil
}
