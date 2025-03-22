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

var templateFuncs = template.FuncMap{
	"split":      strings.Split,
	"join":       strings.Join,
	"joinArray":  JoinArray,
	"trim":       strings.TrimSpace,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"contains":   strings.Contains,
	"replace":    strings.ReplaceAll,
	"filter":     filterLines,
	"grep":       filterLines,
	"cut":        cutFields,
	"exec":       execCommand,
	"atoi": func(s string) interface{} {
		return normalizeNumber(toFloat64(s))
	},
	"add": func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) + toFloat64(b))
	},
	"sub": func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) - toFloat64(b))
	},
	"div": func(a, b interface{}) interface{} {
		bVal := toFloat64(b)
		if bVal == 0 {
			return 0
		}
		return normalizeNumber(toFloat64(a) / bVal)
	},
	"mul": func(a, b interface{}) interface{} {
		return normalizeNumber(toFloat64(a) * toFloat64(b))
	},
	"mod": func(a, b interface{}) interface{} {
		bVal := toFloat64(b)
		if bVal == 0 {
			return 0
		}
		return normalizeNumber(math.Mod(toFloat64(a), bVal))
	},
	"round": func(value interface{}) int {
		return int(math.Round(toFloat64(value)))
	},
	"rand": func(min, max interface{}) int {
		minVal := int(toFloat64(min))
		maxVal := int(toFloat64(max))
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		return minVal + rand.Intn(maxVal-minVal+1)
	},
	"percent": func(part, total interface{}) interface{} {
		totalVal := toFloat64(total)
		if totalVal == 0 {
			return 0
		}
		return normalizeNumber((toFloat64(part) / totalVal) * 100)
	},
	"formatPercent": func(value interface{}, decimals interface{}) string {
		return fmt.Sprintf("%.*f%%", int(toFloat64(decimals)), toFloat64(value))
	},
	"ceil": func(value interface{}) int {
		return int(math.Ceil(toFloat64(value)))
	},
	"floor": func(value interface{}) int {
		return int(math.Floor(toFloat64(value)))
	},
	"abs": func(value interface{}) interface{} {
		return normalizeNumber(math.Abs(toFloat64(value)))
	},
	"max": func(a, b interface{}) interface{} {
		return normalizeNumber(math.Max(toFloat64(a), toFloat64(b)))
	},
	"min": func(a, b interface{}) interface{} {
		return normalizeNumber(math.Min(toFloat64(a), toFloat64(b)))
	},
	"pow": func(base, exponent interface{}) interface{} {
		return normalizeNumber(math.Pow(toFloat64(base), toFloat64(exponent)))
	},
	"sqrt": func(value interface{}) interface{} {
		return normalizeNumber(math.Sqrt(toFloat64(value)))
	},
	"log": func(value interface{}) interface{} {
		return normalizeNumber(math.Log(toFloat64(value)))
	},
	"log10": func(value interface{}) interface{} {
		return normalizeNumber(math.Log10(toFloat64(value)))
	},
	"formatNumber": func(format string, args ...interface{}) string {
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
	},
	"roundTo": func(value interface{}, decimals interface{}) interface{} {
		precision := math.Pow(10, toFloat64(decimals))
		return normalizeNumber(math.Round(toFloat64(value)*precision) / precision)
	},
	"color": func(color string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := colorCodes[strings.ToLower(color)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + colorCodes["reset"]
	},
	"style": func(styleType string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := styleCodes[strings.ToLower(styleType)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + styleCodes["reset"]
	},
	"resetFormat": func() string {
		if os.Getenv("NO_COLOR") != "" {
			return ""
		}
		return colorCodes["reset"]
	},
}

func extendTemplateFuncs(baseFuncs template.FuncMap, ctx *ExecutionContext) template.FuncMap {
	newFuncs := make(template.FuncMap)
	for k, v := range baseFuncs {
		newFuncs[k] = v
	}

	newFuncs["bgTaskStatus"] = func(taskID string) string {
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
	newFuncs["bgTaskComplete"] = func(taskID string) bool {
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
	newFuncs["bgTaskFailed"] = func(taskID string) bool {
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
	return newFuncs
}

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

func filterLines(input, pattern string) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

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

func execCommand(cmd string) string {
	output, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}
	return string(output)
}

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

func transformOutput(output, transform string, ctx *ExecutionContext) (string, error) {
	vars := ctx.templateVars()
	vars["input"] = output
	vars["output"] = output

	return renderTemplate(transform, vars)
}
