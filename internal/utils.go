package internal

import (
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
	result := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// handleDefaultEmpty ensures proper string template replacement
func handleDefaultEmpty(s string) string {
	s = strings.ReplaceAll(s, "<nil>", "")
	s = strings.ReplaceAll(s, "<no value>", "false")

	return s
}

// ensureWorkingDirectory makes sure any workdir values exist on the system
func ensureWorkingDirectory(path string, debug bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create working directory %s: %w", path, err)
		}
		if debug {
			fmt.Printf("Created working directory: %s\n", path)
		}
	}
	return nil
}
