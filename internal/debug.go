package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LogHeader = "=== DEBUG LOGS ==="
	logFooter = "=== END DEBUG LOGS ==="
)

// DebugLogger handles the collection and display of debug logs
type DebugLogger struct {
	enabled     bool
	logs        []DebugEntry
	indentation int
	mu          sync.Mutex
	startTime   time.Time
}

// DebugEntry represents a single debug log entry
type DebugEntry struct {
	Timestamp   time.Time
	Category    string
	Message     string
	Indentation int
	Metadata    map[string]interface{}
}

// Global instance of the debug logger
var debugLogger = &DebugLogger{
	logs:      make([]DebugEntry, 0),
	startTime: time.Now(),
}

// Categories for debug logs
const (
	CategoryRecipe      = "RECIPE"
	CategoryOperation   = "OPERATION"
	CategoryCommand     = "COMMAND"
	CategoryOutput      = "OUTPUT"
	CategoryCondition   = "CONDITION"
	CategoryPrompt      = "PROMPT"
	CategoryComponent   = "COMPONENT"
	CategoryControlFlow = "CONTROL_FLOW"
	CategoryLoop        = "LOOP"
	CategoryBackground  = "BACKGROUND"
	CategoryTemplate    = "TEMPLATE"
	CategoryTransform   = "TRANSFORM"
	CategoryFileSystem  = "FILE_SYSTEM"
	CategoryInit        = "INIT"
	CategoryError       = "ERROR"
)

// InitDebugLogger initializes the debug logger
func InitDebugLogger(isEnabled bool) {
	debugLogger.mu.Lock()

	debugLogger.enabled = isEnabled
	debugLogger.logs = make([]DebugEntry, 0)
	debugLogger.indentation = 0
	debugLogger.startTime = time.Now()

	if isEnabled {
		entry := DebugEntry{
			Timestamp:   time.Now(),
			Category:    CategoryInit,
			Message:     "Debug logging enabled",
			Indentation: 0,
			Metadata:    make(map[string]interface{}),
		}

		debugLogger.logs = append(debugLogger.logs, entry)
	}

	debugLogger.mu.Unlock()
}

// IsDebugEnabled returns whether debug logging is enabled
func IsDebugEnabled() bool {
	debugLogger.mu.Lock()
	defer debugLogger.mu.Unlock()
	return debugLogger.enabled
}

// Log adds a debug entry with the specified category and message
func Log(category, message string, metadata ...map[string]interface{}) {
	if !debugLogger.enabled {
		return
	}

	debugLogger.mu.Lock()
	defer debugLogger.mu.Unlock()

	var meta map[string]interface{}
	if len(metadata) > 0 && metadata[0] != nil {
		meta = metadata[0]
	} else {
		meta = make(map[string]interface{})
	}

	entry := DebugEntry{
		Timestamp:   time.Now(),
		Category:    category,
		Message:     message,
		Indentation: debugLogger.indentation,
		Metadata:    meta,
	}

	debugLogger.logs = append(debugLogger.logs, entry)
}

// LogOperation logs information about an operation
func LogOperation(name, id string, metadata map[string]interface{}) {
	msg := name
	if id != "" {
		msg = fmt.Sprintf("%s (ID: %s)", name, id)
	}

	Log(CategoryOperation, msg, metadata)
}

// LogCommand logs a command execution
func LogCommand(command string, metadata map[string]interface{}) {
	Log(CategoryCommand, command, metadata)
}

// LogOutput logs command output
func LogOutput(output string, metadata map[string]interface{}) {
	if len(output) > 1000 {
		output = output[:1000] + "... [truncated]"
	}
	Log(CategoryOutput, output, metadata)
}

// LogCondition logs condition evaluation
func LogCondition(condition string, result bool, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["result"] = result
	Log(CategoryCondition, condition, metadata)
}

// LogError logs an error
func LogError(message string, err error, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	Log(CategoryError, message, metadata)
}

// IncreaseIndent increases the indentation level for logs
func IncreaseIndent() {
	if !debugLogger.enabled {
		return
	}

	debugLogger.mu.Lock()
	defer debugLogger.mu.Unlock()
	debugLogger.indentation++
}

// DecreaseIndent decreases the indentation level for logs
func DecreaseIndent() {
	if !debugLogger.enabled {
		return
	}

	debugLogger.mu.Lock()
	defer debugLogger.mu.Unlock()

	if debugLogger.indentation > 0 {
		debugLogger.indentation--
	}
}

// LogLoopStart logs the start of a loop
func LogLoopStart(loopType string, metadata map[string]interface{}) {
	Log(CategoryLoop, fmt.Sprintf("Starting %s loop", loopType), metadata)
	IncreaseIndent()
}

// LogLoopIteration logs a loop iteration
func LogLoopIteration(loopType string, iteration int, total int, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["iteration"] = iteration
	metadata["total"] = total

	Log(CategoryLoop, fmt.Sprintf("%s loop iteration %d/%d", loopType, iteration, total), metadata)
}

// LogLoopEnd logs the end of a loop
func LogLoopEnd(loopType string, metadata map[string]interface{}) {
	DecreaseIndent()
	Log(CategoryLoop, fmt.Sprintf("Finished %s loop", loopType), metadata)
}

// LogBackgroundTask logs information about a background task
func LogBackgroundTask(taskID, status string, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["taskID"] = taskID
	metadata["status"] = status

	Log(CategoryBackground, fmt.Sprintf("Background task %s: %s", taskID, status), metadata)
}

// formatDuration formats a duration since the start time
func formatDurationSinceStart(timestamp time.Time) string {
	duration := timestamp.Sub(debugLogger.startTime)
	return fmt.Sprintf("+%.3fs", duration.Seconds())
}

// formatIndentation returns a string with proper indentation
func formatIndentation(indent int) string {
	if indent <= 0 {
		return ""
	}
	return strings.Repeat("  ", indent)
}

// FormatLogs returns all debug logs as a formatted string
func FormatLogs(withMetadata bool) string {
	if !debugLogger.enabled {
		return ""
	}

	debugLogger.mu.Lock()
	defer debugLogger.mu.Unlock()

	if len(debugLogger.logs) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString(LogHeader + "\n")
	b.WriteString(fmt.Sprintf("Started: %s\n", debugLogger.startTime.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Entries: %d\n\n", len(debugLogger.logs)))

	for _, entry := range debugLogger.logs {
		timestamp := formatDurationSinceStart(entry.Timestamp)
		indent := formatIndentation(entry.Indentation)
		b.WriteString(fmt.Sprintf("[%s] %s%s: %s\n", timestamp, indent, entry.Category, entry.Message))

		if withMetadata && len(entry.Metadata) > 0 {
			for k, v := range entry.Metadata {
				b.WriteString(fmt.Sprintf("%s  %s: %v\n", indent, k, v))
			}
		}
	}

	b.WriteString(fmt.Sprintf("\nTotal logs: %d\n", len(debugLogger.logs)))
	b.WriteString(logFooter + "\n")

	return b.String()
}

// PrintLogs prints all collected logs to the console
func PrintLogs() {
	if !debugLogger.enabled {
		return
	}

	if len(debugLogger.logs) == 0 {
		return
	}

	fmt.Println()
	fmt.Print(FormatLogs(false))
}

// SaveLogsToFile saves all collected logs to a file
func SaveLogsToFile(filePath string) error {
	if !debugLogger.enabled || len(debugLogger.logs) == 0 {
		return nil
	}

	if strings.HasPrefix(filePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand ~ in path: %w", err)
		}
		filePath = filepath.Join(homeDir, filePath[2:])
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for debug logs: %w", err)
	}

	logContent := FormatLogs(true)
	return os.WriteFile(filePath, []byte(logContent), 0644)
}
