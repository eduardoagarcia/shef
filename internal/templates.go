package internal

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

// templateVars creates a map of variables available for template rendering
func (ctx *ExecutionContext) templateVars() map[string]interface{} {
	vars := make(map[string]interface{})

	for k, v := range ctx.Vars {
		vars[k] = v
	}

	for opID, output := range ctx.OperationOutputs {
		vars[opID] = output
	}

	vars["context"] = ctx
	vars["operationOutputs"] = ctx.OperationOutputs
	vars["operationResults"] = ctx.OperationResults

	return vars
}

// Template functions are organized by category
var templateFuncs = buildTemplateFunctions()

// buildTemplateFunctions creates the complete template function map
func buildTemplateFunctions() template.FuncMap {
	funcs := template.FuncMap{}

	stringFunctions(funcs)
	mathFunctions(funcs)
	formattingFunctions(funcs)

	return funcs
}

// stringFunctions adds string manipulation functions to the template function map
func stringFunctions(funcs template.FuncMap) {
	funcs["split"] = strings.Split
	funcs["join"] = strings.Join
	funcs["joinArray"] = JoinArray
	funcs["trim"] = strings.TrimSpace
	funcs["trimPrefix"] = strings.TrimPrefix
	funcs["trimSuffix"] = strings.TrimSuffix
	funcs["contains"] = strings.Contains
	funcs["replace"] = strings.ReplaceAll
	funcs["filter"] = filterLines
	funcs["grep"] = filterLines
	funcs["cut"] = cutFields
	funcs["exec"] = execCommand
}

// mathFunctions adds mathematical functions to the template function map
func mathFunctions(funcs template.FuncMap) {
	funcs["atoi"] = func(s string) interface{} {
		return normalizeNumber(toFloat64(s))
	}
	funcs["add"] = func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) + toFloat64(b))
	}
	funcs["sub"] = func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) - toFloat64(b))
	}
	funcs["div"] = func(a, b interface{}) interface{} {
		bVal := toFloat64(b)
		if bVal == 0 {
			return 0
		}
		return normalizeNumber(toFloat64(a) / bVal)
	}
	funcs["mul"] = func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) * toFloat64(b))
	}
	funcs["mod"] = func(a, b interface{}) interface{} {
		bVal := toFloat64(b)
		if bVal == 0 {
			return 0
		}
		return normalizeNumber(math.Mod(toFloat64(a), bVal))
	}
	funcs["round"] = func(value interface{}) int {
		return int(math.Round(toFloat64(value)))
	}
	funcs["rand"] = func(min, max interface{}) int {
		minVal := int(toFloat64(min))
		maxVal := int(toFloat64(max))
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		return minVal + rand.Intn(maxVal-minVal+1)
	}
	funcs["percent"] = func(part, total interface{}) interface{} {
		totalVal := toFloat64(total)
		if totalVal == 0 {
			return 0
		}
		return normalizeNumber((toFloat64(part) / totalVal) * 100)
	}
	funcs["ceil"] = func(value interface{}) int {
		return int(math.Ceil(toFloat64(value)))
	}
	funcs["floor"] = func(value interface{}) int {
		return int(math.Floor(toFloat64(value)))
	}
	funcs["abs"] = func(value interface{}) interface{} {
		return normalizeNumber(math.Abs(toFloat64(value)))
	}
	funcs["max"] = func(a, b interface{}) interface{} {
		return normalizeNumber(math.Max(toFloat64(a), toFloat64(b)))
	}
	funcs["min"] = func(a, b interface{}) interface{} {
		return normalizeNumber(math.Min(toFloat64(a), toFloat64(b)))
	}
	funcs["pow"] = func(base, exponent interface{}) interface{} {
		return normalizeNumber(math.Pow(toFloat64(base), toFloat64(exponent)))
	}
	funcs["sqrt"] = func(value interface{}) interface{} {
		return normalizeNumber(math.Sqrt(toFloat64(value)))
	}
	funcs["log"] = func(value interface{}) interface{} {
		return normalizeNumber(math.Log(toFloat64(value)))
	}
	funcs["log10"] = func(value interface{}) interface{} {
		return normalizeNumber(math.Log10(toFloat64(value)))
	}
	funcs["roundTo"] = func(value interface{}, decimals interface{}) interface{} {
		precision := math.Pow(10, toFloat64(decimals))
		return normalizeNumber(math.Round(toFloat64(value)*precision) / precision)
	}
}

// formattingFunctions adds display formatting functions to the template function map
func formattingFunctions(funcs template.FuncMap) {
	funcs["formatPercent"] = func(value interface{}, decimals interface{}) string {
		return fmt.Sprintf("%.*f%%", int(toFloat64(decimals)), toFloat64(value))
	}
	funcs["formatNumber"] = func(format string, args ...interface{}) string {
		processedArgs := make([]interface{}, len(args))
		for i, arg := range args {
			switch arg.(type) {
			case int, int64, float32, float64, string:
				processedArgs[i] = normalizeNumber(toFloat64(arg))
			default:
				processedArgs[i] = arg
			}
		}
		return fmt.Sprintf(format, processedArgs...)
	}
	funcs["color"] = func(color string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := colorCodes[strings.ToLower(color)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + colorCodes["reset"]
	}
	funcs["style"] = func(styleType string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := styleCodes[strings.ToLower(styleType)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + styleCodes["reset"]
	}
	funcs["resetFormat"] = func() string {
		if os.Getenv("NO_COLOR") != "" {
			return ""
		}
		return colorCodes["reset"]
	}
}

// extendTemplateFuncs adds context-specific functions like background task status
func extendTemplateFuncs(baseFuncs template.FuncMap, ctx *ExecutionContext) template.FuncMap {
	newFuncs := make(template.FuncMap)
	for k, v := range baseFuncs {
		newFuncs[k] = v
	}

	newFuncs["bgTaskStatus"] = backgroundTaskStatusFunc(ctx)
	newFuncs["bgTaskComplete"] = backgroundTaskCompleteFunc(ctx)
	newFuncs["bgTaskFailed"] = backgroundTaskFailedFunc(ctx)

	return newFuncs
}

// backgroundTaskStatusFunc returns a function to check background task status
func backgroundTaskStatusFunc(ctx *ExecutionContext) func(string) string {
	return func(taskID string) string {
		ctx.BackgroundMutex.RLock()
		defer ctx.BackgroundMutex.RUnlock()

		if ctx.BackgroundTasks == nil {
			return "unknown"
		}

		if task, exists := ctx.BackgroundTasks[taskID]; exists {
			return string(task.Status)
		}
		return "unknown"
	}
}

// backgroundTaskCompleteFunc returns a function to check if a task is complete
func backgroundTaskCompleteFunc(ctx *ExecutionContext) func(string) bool {
	return func(taskID string) bool {
		ctx.BackgroundMutex.RLock()
		defer ctx.BackgroundMutex.RUnlock()

		if ctx.BackgroundTasks == nil {
			return false
		}

		if task, exists := ctx.BackgroundTasks[taskID]; exists {
			return task.Status == TaskComplete
		}
		return false
	}
}

// backgroundTaskFailedFunc returns a function to check if a task has failed
func backgroundTaskFailedFunc(ctx *ExecutionContext) func(string) bool {
	return func(taskID string) bool {
		ctx.BackgroundMutex.RLock()
		defer ctx.BackgroundMutex.RUnlock()

		if ctx.BackgroundTasks == nil {
			return false
		}

		if task, exists := ctx.BackgroundTasks[taskID]; exists {
			return task.Status == TaskFailed
		}
		return false
	}
}

// JoinArray joins array elements into a string with the specified separator
func JoinArray(arr interface{}, sep string) string {
	switch v := arr.(type) {
	case []string:
		return strings.Join(v, sep)
	case []interface{}:
		strs := make([]string, len(v))
		for i, val := range v {
			strs[i] = fmt.Sprintf("%v", val)
		}
		return strings.Join(strs, sep)
	default:
		return fmt.Sprintf("%v", arr)
	}
}

// filterLines returns lines containing the specified pattern
func filterLines(input, pattern string) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// cutFields extracts a specific field from each line using the delimiter
func cutFields(input, delimiter string, field int) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		fields := strings.Split(line, delimiter)
		if field < len(fields) {
			result = append(result, strings.TrimSpace(fields[field]))
		}
	}
	return strings.Join(result, "\n")
}

// execCommand executes a shell command and returns its output
func execCommand(cmd string) string {
	output, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}
	return string(output)
}

// renderTemplate parses and executes a Go template with the provided variables
func renderTemplate(tmplStr string, vars map[string]interface{}) (string, error) {
	funcs := templateFuncs
	if ctxVal, ok := vars["context"]; ok {
		if ctx, ok := ctxVal.(*ExecutionContext); ok && ctx != nil {
			if ctx.templateFuncs == nil {
				ctx.templateFuncs = extendTemplateFuncs(templateFuncs, ctx)
			}
			funcs = ctx.templateFuncs
		}
	}

	tmpl, err := template.New("template").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	result := buf.String()
	result = strings.ReplaceAll(result, "<no value>", "false")

	return result, nil
}

// transformOutput applies a template transformation to the given output
func transformOutput(output, transform string, ctx *ExecutionContext) (string, error) {
	vars := ctx.templateVars()
	vars["input"] = output
	vars["output"] = output

	return renderTemplate(transform, vars)
}
