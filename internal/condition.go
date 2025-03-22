package internal

import (
	"fmt"
	"strconv"
	"strings"
)

func evaluateAndCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, "&&")
	for _, part := range parts {
		result, err := evaluateCondition(part, ctx)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func evaluateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	if condition == "" {
		return true, nil
	}

	if strings.Contains(condition, "{{") && strings.Contains(condition, "}}") {
		rendered, err := renderTemplate(condition, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render condition template: %w", err)
		}
		rendered = strings.ReplaceAll(rendered, "<no value>", "false")
		return evaluateCondition(rendered, ctx)
	}

	condition = strings.TrimSpace(condition)

	if strings.Contains(condition, "&&") {
		return evaluateAndCondition(condition, ctx)
	}

	if strings.Contains(condition, "||") {
		return evaluateOrCondition(condition, ctx)
	}

	if strings.HasPrefix(condition, "!") {
		return evaluateNotCondition(condition, ctx)
	}

	result, err := evaluateNumericComparison(condition, ctx)
	if err == nil {
		return result, nil
	}

	if strings.Contains(condition, ".success") || strings.Contains(condition, ".failure") {
		return evaluateOperationResult(condition, ctx)
	}

	if strings.Contains(condition, "==") || strings.Contains(condition, "!=") {
		return evaluateVariableComparison(condition, ctx)
	}

	if condition == "true" {
		return true, nil
	}
	if condition == "false" {
		return false, nil
	}

	return false, fmt.Errorf("unsupported condition format: %s", condition)
}

func evaluateOperationResult(condition string, ctx *ExecutionContext) (bool, error) {
	if strings.Contains(condition, ".success") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "success" {
			opID := strings.TrimSpace(parts[0])
			result, exists := ctx.OperationResults[opID]
			if exists {
				return result, nil
			}
			return false, nil
		}
	}

	if strings.Contains(condition, ".failure") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "failure" {
			opID := strings.TrimSpace(parts[0])
			result, exists := ctx.OperationResults[opID]
			if exists {
				return !result, nil
			}
			return true, nil
		}
	}

	return false, fmt.Errorf("invalid operation result condition: %s", condition)
}

func evaluateNotCondition(condition string, ctx *ExecutionContext) (bool, error) {
	subCondition := strings.TrimSpace(condition[1:])
	result, err := evaluateCondition(subCondition, ctx)
	if err != nil {
		return false, err
	}
	return !result, nil
}

func evaluateNumericComparison(condition string, ctx *ExecutionContext) (bool, error) {
	var op string
	var parts []string

	if strings.Contains(condition, ">=") {
		parts = strings.Split(condition, ">=")
		op = ">="
	} else if strings.Contains(condition, "<=") {
		parts = strings.Split(condition, "<=")
		op = "<="
	} else if strings.Contains(condition, ">") {
		parts = strings.Split(condition, ">")
		op = ">"
	} else if strings.Contains(condition, "<") {
		parts = strings.Split(condition, "<")
		op = "<"
	} else {
		return false, fmt.Errorf("not a numeric comparison")
	}

	if len(parts) != 2 {
		return false, fmt.Errorf("invalid numeric comparison format: %s", condition)
	}

	leftStr, err := resolveValue(strings.TrimSpace(parts[0]), ctx)
	if err != nil {
		return false, err
	}

	rightStr, err := resolveValue(strings.TrimSpace(parts[1]), ctx)
	if err != nil {
		return false, err
	}

	if leftStr == "false" {
		leftStr = "0"
	}
	if rightStr == "false" {
		rightStr = "0"
	}

	leftInt, leftErr := strconv.Atoi(leftStr)
	rightInt, rightErr := strconv.Atoi(rightStr)

	if leftErr == nil && rightErr == nil {
		switch op {
		case ">":
			return leftInt > rightInt, nil
		case "<":
			return leftInt < rightInt, nil
		case ">=":
			return leftInt >= rightInt, nil
		default:
			return leftInt <= rightInt, nil
		}
	}

	leftFloat, leftErr := strconv.ParseFloat(leftStr, 64)
	rightFloat, rightErr := strconv.ParseFloat(rightStr, 64)

	if leftErr != nil || rightErr != nil {
		return false, fmt.Errorf("numeric comparison requires numeric values, got '%s' and '%s'", leftStr, rightStr)
	}

	switch op {
	case ">":
		return leftFloat > rightFloat, nil
	case "<":
		return leftFloat < rightFloat, nil
	case ">=":
		return leftFloat >= rightFloat, nil
	default:
		return leftFloat <= rightFloat, nil
	}
}

func resolveValue(value string, ctx *ExecutionContext) (string, error) {
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		rendered, err := renderTemplate(value, ctx.templateVars())
		if err != nil {
			return "", err
		}
		return rendered, nil
	}

	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, ".") {
		varName := value
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:]
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:]
		}

		if val, exists := ctx.Vars[varName]; exists {
			return fmt.Sprintf("%v", val), nil
		}
		if val, exists := ctx.OperationOutputs[varName]; exists {
			return val, nil
		}
		return "false", nil
	}

	return value, nil
}

func evaluateOrCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, "||")
	for _, part := range parts {
		result, err := evaluateCondition(part, ctx)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

func evaluateVariableComparison(condition string, ctx *ExecutionContext) (bool, error) {
	var op string
	var parts []string

	if strings.Contains(condition, "==") {
		parts = strings.Split(condition, "==")
		op = "=="
	} else if strings.Contains(condition, "!=") {
		parts = strings.Split(condition, "!=")
		op = "!="
	} else {
		return false, fmt.Errorf("not a variable comparison")
	}

	if len(parts) != 2 {
		return false, fmt.Errorf("invalid variable comparison format: %s", condition)
	}

	leftPart := strings.TrimSpace(parts[0])
	rightPart := strings.TrimSpace(parts[1])
	rightPart = strings.Trim(rightPart, "\"'")

	varName := leftPart
	if strings.HasPrefix(varName, "$") {
		varName = varName[1:]
	}
	if strings.HasPrefix(varName, ".") {
		varName = varName[1:]
	}

	var actualValue string
	var exists bool

	if value, ok := ctx.Vars[varName]; ok {
		actualValue = fmt.Sprintf("%v", value)
		exists = true
	}

	if !exists {
		if value, ok := ctx.OperationOutputs[varName]; ok {
			actualValue = value
			exists = true
		}
	}

	if !exists {
		actualValue = "false"
	}

	var result bool
	if op == "==" {
		result = actualValue == rightPart
	} else {
		result = actualValue != rightPart
	}

	return result, nil
}
