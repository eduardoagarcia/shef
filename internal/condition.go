package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// evaluateCondition determines if a condition is true based on the execution context
func evaluateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	result, err := evaluateConditionWrapper(condition, ctx)

	Log(CategoryCondition, fmt.Sprintf("Condition result: %v", result))

	return result, err
}

func evaluateConditionWrapper(condition string, ctx *ExecutionContext) (bool, error) {
	if condition == "" {
		return true, nil
	}

	// Log the condition we're about to evaluate
	Log(CategoryCondition, fmt.Sprintf("Evaluating raw condition: '%s'", condition))

	if isTemplateCondition(condition) {
		Log(CategoryCondition, "Detected template condition")
		return evaluateTemplateCondition(condition, ctx)
	}

	condition = strings.TrimSpace(condition)

	// Handle basic boolean values
	if condition == "true" {
		return true, nil
	}
	if condition == "false" {
		return false, nil
	}

	// Special case for complex parenthesized conditions
	if strings.HasPrefix(condition, "(") && strings.HasSuffix(condition, ")") {
		// First check if this is a single parenthesized condition with no operators
		innerCondition := condition[1 : len(condition)-1]
		if !strings.Contains(innerCondition, "&&") && !strings.Contains(innerCondition, "||") {
			Log(CategoryCondition, fmt.Sprintf("Evaluating inner condition: '%s'", innerCondition))
			return evaluateCondition(innerCondition, ctx)
		}
	}

	// Handle logical operators with proper parentheses handling
	if strings.Contains(condition, "&&") {
		Log(CategoryCondition, "Detected AND condition")
		return evaluateAndCondition(condition, ctx)
	}
	if strings.Contains(condition, "||") {
		Log(CategoryCondition, "Detected OR condition")
		return evaluateOrCondition(condition, ctx)
	}
	if strings.HasPrefix(condition, "!") {
		Log(CategoryCondition, "Detected NOT condition")
		return evaluateNotCondition(condition, ctx)
	}

	// Handle various comparison types
	if result, err := evaluateNumericComparison(condition, ctx); err == nil {
		Log(CategoryCondition, "Evaluated as numeric comparison")
		return result, nil
	}
	if isOperationResultCondition(condition) {
		Log(CategoryCondition, "Detected operation result condition")
		return evaluateOperationResult(condition, ctx)
	}
	if isVariableComparison(condition) {
		Log(CategoryCondition, "Detected variable comparison")
		return evaluateVariableComparison(condition, ctx)
	}

	Log(CategoryCondition, fmt.Sprintf("Unsupported condition format: '%s'", condition))
	return false, fmt.Errorf("unsupported condition format: %s", condition)
}

// isTemplateCondition checks if the condition contains Go template syntax
func isTemplateCondition(condition string) bool {
	return strings.Contains(condition, "{{") && strings.Contains(condition, "}}")
}

// evaluateTemplateCondition renders a template condition and evaluates the result
func evaluateTemplateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	rendered, err := renderTemplate(condition, ctx.templateVars())
	if err != nil {
		return false, fmt.Errorf("failed to render condition template: %w", err)
	}
	rendered = handleDefaultEmpty(rendered)
	return evaluateCondition(rendered, ctx)
}

// evaluateAndCondition evaluates a condition with AND operators (&&)
func evaluateAndCondition(condition string, ctx *ExecutionContext) (bool, error) {
	// Use regular expression to properly handle parentheses and AND conditions
	parts := strings.Split(condition, "&&")
	for _, part := range parts {
		// Trim spaces and remove surrounding parentheses if present
		trimmedPart := strings.TrimSpace(part)
		if strings.HasPrefix(trimmedPart, "(") && strings.HasSuffix(trimmedPart, ")") {
			trimmedPart = trimmedPart[1 : len(trimmedPart)-1]
		}

		result, err := evaluateCondition(trimmedPart, ctx)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

// evaluateOrCondition evaluates a condition with OR operators (||)
func evaluateOrCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, "||")
	for _, part := range parts {
		// Trim spaces and remove surrounding parentheses if present
		trimmedPart := strings.TrimSpace(part)
		if strings.HasPrefix(trimmedPart, "(") && strings.HasSuffix(trimmedPart, ")") {
			trimmedPart = trimmedPart[1 : len(trimmedPart)-1]
		}

		result, err := evaluateCondition(trimmedPart, ctx)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

// evaluateNotCondition evaluates a negated condition (!)
func evaluateNotCondition(condition string, ctx *ExecutionContext) (bool, error) {
	subCondition := strings.TrimSpace(condition[1:])
	result, err := evaluateCondition(subCondition, ctx)
	if err != nil {
		return false, err
	}
	return !result, nil
}

// isOperationResultCondition checks if a condition refers to an operation result
func isOperationResultCondition(condition string) bool {
	return strings.Contains(condition, ".success") || strings.Contains(condition, ".failure")
}

// evaluateOperationResult evaluates conditions based on operation success/failure
func evaluateOperationResult(condition string, ctx *ExecutionContext) (bool, error) {
	if strings.Contains(condition, ".success") {
		return evaluateSuccessCondition(condition, ctx)
	}
	if strings.Contains(condition, ".failure") {
		return evaluateFailureCondition(condition, ctx)
	}
	return false, fmt.Errorf("invalid operation result condition: %s", condition)
}

// evaluateSuccessCondition checks if an operation was successful
func evaluateSuccessCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, ".")
	if len(parts) == 2 && parts[1] == "success" {
		opID := strings.TrimSpace(parts[0])
		result, exists := ctx.OperationResults[opID]
		if exists {
			return result, nil
		}
		return false, nil
	}
	return false, fmt.Errorf("invalid success condition: %s", condition)
}

// evaluateFailureCondition checks if an operation failed
func evaluateFailureCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, ".")
	if len(parts) == 2 && parts[1] == "failure" {
		opID := strings.TrimSpace(parts[0])
		result, exists := ctx.OperationResults[opID]
		if exists {
			return !result, nil
		}
		return true, nil
	}
	return false, fmt.Errorf("invalid failure condition: %s", condition)
}

// isVariableComparison checks if a condition compares variables
func isVariableComparison(condition string) bool {
	return strings.Contains(condition, "==") || strings.Contains(condition, "!=")
}

// evaluateNumericComparison evaluates a numeric comparison condition
func evaluateNumericComparison(condition string, ctx *ExecutionContext) (bool, error) {
	operator, leftPart, rightPart, err := parseComparisonParts(condition, []string{">=", "<=", ">", "<"})
	if err != nil {
		return false, err
	}

	leftStr, err := resolveValue(leftPart, ctx)
	if err != nil {
		return false, err
	}

	rightStr, err := resolveValue(rightPart, ctx)
	if err != nil {
		return false, err
	}

	return compareNumericValues(leftStr, rightStr, operator)
}

// parseComparisonParts extracts the operator and operands from a comparison
func parseComparisonParts(condition string, operators []string) (string, string, string, error) {
	for _, op := range operators {
		if strings.Contains(condition, op) {
			parts := strings.Split(condition, op)
			if len(parts) != 2 {
				return "", "", "", fmt.Errorf("invalid comparison format: %s", condition)
			}
			return op, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
		}
	}
	return "", "", "", fmt.Errorf("no comparison operator found in: %s", condition)
}

// compareNumericValues performs a numeric comparison between two values
func compareNumericValues(leftStr, rightStr, operator string) (bool, error) {
	leftStr = normalizeBooleanValue(leftStr)
	rightStr = normalizeBooleanValue(rightStr)

	// Try comparing as integers first
	leftInt, leftErr := strconv.Atoi(leftStr)
	rightInt, rightErr := strconv.Atoi(rightStr)

	if leftErr == nil && rightErr == nil {
		return performIntComparison(leftInt, rightInt, operator)
	}

	// Fall back to float comparison
	leftFloat, leftErr := strconv.ParseFloat(leftStr, 64)
	rightFloat, rightErr := strconv.ParseFloat(rightStr, 64)

	if leftErr != nil || rightErr != nil {
		return false, fmt.Errorf("numeric comparison requires numeric values, got '%s' and '%s'", leftStr, rightStr)
	}

	return performFloatComparison(leftFloat, rightFloat, operator)
}

// normalizeBooleanValue converts boolean strings to numeric equivalents
func normalizeBooleanValue(value string) string {
	if value == "false" {
		return "0"
	}
	return value
}

// performIntComparison compares two integers using the specified operator
func performIntComparison(left, right int, operator string) (bool, error) {
	switch operator {
	case ">":
		return left > right, nil
	case "<":
		return left < right, nil
	case ">=":
		return left >= right, nil
	case "<=":
		return left <= right, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// performFloatComparison compares two floats using the specified operator
func performFloatComparison(left, right float64, operator string) (bool, error) {
	switch operator {
	case ">":
		return left > right, nil
	case "<":
		return left < right, nil
	case ">=":
		return left >= right, nil
	case "<=":
		return left <= right, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// evaluateVariableComparison evaluates equality conditions (== or !=)
func evaluateVariableComparison(condition string, ctx *ExecutionContext) (bool, error) {
	operator, leftPart, rightPart, err := parseComparisonParts(condition, []string{"==", "!="})
	if err != nil {
		return false, err
	}

	// Trim spaces around the parts
	leftPart = strings.TrimSpace(leftPart)
	rightPart = strings.TrimSpace(rightPart)

	// Remove quotes from right part (literal string)
	rightPart = strings.Trim(rightPart, "\"'")

	// Resolve the actual value from context
	actualValue := resolveVariableValue(leftPart, ctx)

	// Log comparison for debugging
	Log(CategoryCondition, fmt.Sprintf("Comparing: '%s' %s '%s'", actualValue, operator, rightPart))

	if operator == "==" {
		return actualValue == rightPart, nil
	}
	return actualValue != rightPart, nil
}

// resolveVariableValue gets a variable's value from the context
func resolveVariableValue(varRef string, ctx *ExecutionContext) string {
	varName := normalizeVariableName(varRef)

	if value, isDynamic := resolveDynamicVariable(varName, ctx); isDynamic {
		return value
	}

	if value, ok := ctx.Vars[varName]; ok {
		return fmt.Sprintf("%v", value)
	}

	ctx.OperationMutex.RLock()
	value, ok := ctx.OperationOutputs[varName]
	ctx.OperationMutex.RUnlock()

	if ok {
		return value
	}
	return "false"
}

// resolveDynamicVariable checks if a variable is a dynamic variable and returns its value
func resolveDynamicVariable(varName string, ctx *ExecutionContext) (string, bool) {
	switch varName {
	case "allTasksComplete":
		return ctx.allTasksComplete(), true
	case "anyTasksFailed":
		return ctx.anyTasksFailed(), true
	default:
		return "", false
	}
}

// normalizeVariableName removes $ or . prefixes from variable names
func normalizeVariableName(name string) string {
	if strings.HasPrefix(name, "$") {
		name = name[1:]
	}
	if strings.HasPrefix(name, ".") {
		name = name[1:]
	}
	return name
}

// resolveValue resolves a value from a template, variable, or literal
func resolveValue(value string, ctx *ExecutionContext) (string, error) {
	if isTemplateCondition(value) {
		rendered, err := renderTemplate(value, ctx.templateVars())
		if err != nil {
			return "", err
		}
		return rendered, nil
	}

	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, ".") {
		actualValue := resolveVariableValue(value, ctx)
		return actualValue, nil
	}

	return value, nil
}
