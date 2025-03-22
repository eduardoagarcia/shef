package internal

import (
	"fmt"
	"strconv"
	"time"
)

func normalizeNumber(val float64) interface{} {
	if val == float64(int(val)) {
		return int(val)
	}
	return val
}

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

func formatDurationWithMs(d time.Duration) string {
	baseFormat := formatDuration(d)
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%s.%03d", baseFormat, milliseconds)
}

func updateDurationVars(ctx *ExecutionContext, startTime time.Time) {
	elapsed := time.Since(startTime)

	ctx.Vars["duration_ms"] = fmt.Sprintf("%d", elapsed.Milliseconds())
	ctx.Vars["duration_s"] = fmt.Sprintf("%d", int(elapsed.Seconds()))

	ctx.Vars["duration_fmt"] = formatDuration(elapsed)
	ctx.Vars["duration_ms_fmt"] = formatDurationWithMs(elapsed)
}
