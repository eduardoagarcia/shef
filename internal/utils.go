package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// normalizeNumber converts float values to integers when they have no fractional part
func normalizeNumber(val float64) interface{} {
	if val == float64(int(val)) {
		return int(val)
	}
	return val
}

// toFloat64 converts various types to float64 with best-effort conversion
func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		s := fmt.Sprintf("%v", val)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	}
}

// formatDuration formats a duration as HH:MM:SS or MM:SS depending on length
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// formatDurationWithMs formats a duration with millisecond precision
func formatDurationWithMs(d time.Duration) string {
	baseFormat := formatDuration(d)
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%s.%03d", baseFormat, milliseconds)
}

// updateDurationVars updates duration-related variables in the execution context
func updateDurationVars(ctx *ExecutionContext, startTime time.Time) {
	elapsed := time.Since(startTime)

	ctx.Vars["duration_ms"] = fmt.Sprintf("%d", elapsed.Milliseconds())
	ctx.Vars["duration_s"] = fmt.Sprintf("%d", int(elapsed.Seconds()))

	ctx.Vars["duration_fmt"] = formatDuration(elapsed)
	ctx.Vars["duration_ms_fmt"] = formatDurationWithMs(elapsed)
}

// parseOptionsFromOutput converts multi-line output to a string slice of options
func parseOptionsFromOutput(output string) []string {
	return toList(output)
}

// handleDefaultEmpty ensures proper string template replacement
func handleDefaultEmpty(s string) string {
	s = strings.ReplaceAll(s, "<nil>", "")
	s = strings.ReplaceAll(s, "<no value>", "false")

	return s
}

// ensureWorkingDirectory makes sure any workdir values exist on the system
func ensureWorkingDirectory(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create working directory %s: %w", path, err)
		}
		Log(CategoryFileSystem, fmt.Sprintf("Created working directory: %s", path))
	}
	return nil
}

// toList converts any input value to a normalized list representation
func toList(input interface{}) []string {
	if input == nil {
		return []string{}
	}

	var result []string

	switch v := input.(type) {
	case []string:
		for _, s := range v {
			if clean := strings.TrimSpace(s); clean != "" {
				result = append(result, clean)
			}
		}

	case []interface{}:
		for _, item := range v {
			if s := fmt.Sprintf("%v", item); s != "" && s != "<nil>" {
				result = append(result, strings.TrimSpace(s))
			}
		}

	case string:
		if v == "" {
			return []string{}
		}

		trimmed := strings.TrimSpace(v)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if inner == "" {
				return []string{}
			}

			foundComma := strings.Contains(inner, ",")

			if foundComma {
				for _, item := range strings.Split(inner, ",") {
					cleaned := strings.Trim(strings.TrimSpace(item), "\"'")
					if cleaned != "" {
						result = append(result, cleaned)
					}
				}
			} else {
				for _, item := range strings.Fields(inner) {
					cleaned := strings.Trim(item, "\"'")
					if cleaned != "" {
						result = append(result, cleaned)
					}
				}
			}
			return result
		}

		if strings.Contains(trimmed, "\n") {
			for _, line := range strings.Split(trimmed, "\n") {
				if clean := strings.TrimSpace(line); clean != "" {
					result = append(result, clean)
				}
			}
			return result
		}

		if strings.Contains(trimmed, ",") {
			for _, item := range strings.Split(trimmed, ",") {
				if clean := strings.TrimSpace(item); clean != "" {
					result = append(result, clean)
				}
			}
			return result
		}

		result = append(result, trimmed)

	default:
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			result = append(result, strings.TrimSpace(s))
		}
	}

	return result
}

// toNestedList converts input to a nested list structure
func toNestedList(input interface{}) [][]string {
	var result [][]string

	if input == nil {
		return result
	}

	switch v := input.(type) {
	case [][]string:
		return v

	case [][]interface{}:
		result = make([][]string, len(v))
		for i, row := range v {
			result[i] = make([]string, len(row))
			for j, cell := range row {
				result[i][j] = fmt.Sprintf("%v", cell)
			}
		}
		return result

	case []interface{}:
		for _, row := range v {
			if rowArr, ok := row.([]interface{}); ok {
				cells := make([]string, len(rowArr))
				for j, cell := range rowArr {
					cells[j] = fmt.Sprintf("%v", cell)
				}
				result = append(result, cells)
			} else if rowStr, ok := row.([]string); ok {
				result = append(result, rowStr)
			} else {
				rowCells := toList(row)
				if len(rowCells) > 0 {
					result = append(result, rowCells)
				}
			}
		}
		return result

	case string:
		if v == "" {
			return result
		}

		trimmed := strings.TrimSpace(v)

		if strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]") {
			var jsonArray [][]interface{}
			err := json.Unmarshal([]byte(trimmed), &jsonArray)

			if err == nil {
				for _, row := range jsonArray {
					cells := make([]string, len(row))
					for j, cell := range row {
						cells[j] = fmt.Sprintf("%v", cell)
					}
					result = append(result, cells)
				}
				return result
			}

			inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])

			depth := 0
			var currentRow strings.Builder
			inQuotes := false
			quoteChar := ' '

			for _, char := range inner {
				if (char == '"' || char == '\'') && (depth <= 1) {
					if inQuotes && char == quoteChar {
						inQuotes = false
					} else if !inQuotes {
						inQuotes = true
						quoteChar = char
					}
					currentRow.WriteRune(char)
					continue
				}

				if inQuotes {
					currentRow.WriteRune(char)
					continue
				}

				if char == '[' {
					depth++
					if depth == 1 {
						currentRow.Reset()
					}
					currentRow.WriteRune(char)
				} else if char == ']' {
					depth--
					currentRow.WriteRune(char)
					if depth == 0 {
						rowContent := currentRow.String()
						if strings.HasPrefix(rowContent, "[") && strings.HasSuffix(rowContent, "]") {
							rowContent = strings.TrimSpace(rowContent[1 : len(rowContent)-1])
						}
						rowCells := toList(rowContent)
						if len(rowCells) > 0 {
							result = append(result, rowCells)
						}
					}
				} else {
					currentRow.WriteRune(char)
				}
			}

			return result
		}

		rows := toList(trimmed)
		for _, row := range rows {
			result = append(result, toList(row))
		}
		return result

	default:
		str := fmt.Sprintf("%v", v)
		if str != "" && str != "<nil>" {
			return toNestedList(str)
		}
	}

	return result
}

// formatResult formats a string slice result to match the original input format
// This preserves the format of the original input (array, newline-separated string,
// comma-separated string, or space-separated array syntax) for a consistent user experience
func formatResult(result []string, originalInput interface{}) interface{} {
	if len(result) == 0 {
		switch originalInput.(type) {
		case []string, []interface{}:
			return []string{}
		default:
			return ""
		}
	}

	switch v := originalInput.(type) {
	case string:
		trimmed := strings.TrimSpace(v)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			return "[" + strings.Join(result, " ") + "]"
		}

		if strings.Contains(trimmed, ",") && !strings.Contains(trimmed, "\n") {
			return strings.Join(result, ", ")
		}

		return strings.Join(result, "\n")

	case []string, []interface{}:
		return result

	default:
		return strings.Join(result, "\n")
	}
}
