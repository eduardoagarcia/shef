package internal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
)

// executeCommand runs a shell command in the specified execution mode
func executeCommand(cmdStr string, input string, executionMode string, outputFormat string) (string, error) {
	if executionMode == "" {
		executionMode = "standard"
	}

	switch executionMode {
	case "standard":
		return executeStandardCommand(cmdStr, input, outputFormat)
	case "interactive", "stream":
		return executeInteractiveCommand(cmdStr)
	case "background":
		return string(TaskPending), nil
	default:
		return "", fmt.Errorf("unknown execution mode: %s", executionMode)
	}
}

// executeStandardCommand runs a command and captures its output
func executeStandardCommand(cmdStr string, input string, outputFormat string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\nStderr: %s", err, stderr.String())
	}

	return formatOutput(stdout.String(), outputFormat)
}

// formatOutput processes command output according to the specified format
func formatOutput(output string, outputFormat string) (string, error) {
	switch outputFormat {
	case "trim":
		return strings.TrimSpace(output), nil
	case "lines":
		var lines []string
		for _, line := range strings.Split(output, "\n") {
			if trimmedLine := strings.TrimSpace(line); trimmedLine != "" {
				lines = append(lines, trimmedLine)
			}
		}
		return strings.Join(lines, "\n"), nil
	case "raw", "":
		return output, nil
	default:
		return output, nil
	}
}

// executeInteractiveCommand runs a command with direct connection to terminal I/O
func executeInteractiveCommand(cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	return waitForInteractiveCommand(cmd)
}

// waitForInteractiveCommand waits for an interactive command to complete
func waitForInteractiveCommand(cmd *exec.Cmd) (string, error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		if err := interruptAndWaitForCommand(cmd, done); err != nil {
			return "", err
		}
		return "", nil
	case err := <-done:
		return "", err
	}
}

// interruptAndWaitForCommand sends interrupt signal and waits for command to exit
func interruptAndWaitForCommand(cmd *exec.Cmd, done chan error) error {
	err := cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return err
	}

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return cmd.Process.Kill()
	}
}

// executeBackgroundCommand runs a command asynchronously in the background
func executeBackgroundCommand(op Operation, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) error {
	if op.ID == "" {
		return fmt.Errorf("background execution requires an operation ID")
	}

	renderedID, err := renderTemplate(op.ID, ctx.templateVars())
	if err != nil {
		return fmt.Errorf("failed to render operation ID template: %w", err)
	}
	if renderedID != op.ID {
		opCopy := op
		opCopy.ID = renderedID
		op = opCopy

		if debug {
			fmt.Printf("Rendered background task ID: '%s' -> '%s'\n", op.ID, renderedID)
		}
	}

	cmd, err := renderTemplate(op.Command, ctx.templateVars())
	if err != nil {
		return fmt.Errorf("failed to render command template: %w", err)
	}

	ctx.BackgroundMutex.Lock()
	task, exists := ctx.BackgroundTasks[op.ID]
	if exists && task.Status == TaskPending {
		ctx.BackgroundMutex.Unlock()
		if debug {
			fmt.Printf("Task %s is already running, skipping duplicate execution!\n", op.ID)
		}
		return nil
	}

	if debug {
		fmt.Printf("Starting background task: %s\n", cmd)
	}

	initializeBackgroundTask(op.ID, cmd, ctx)
	ctx.BackgroundMutex.Unlock()

	ctx.BackgroundWg.Add(1)
	go executeBackgroundTask(op, cmd, ctx, opMap, executeOp, depth, debug)

	return nil
}

// initializeBackgroundTask sets up a new background task in the context
func initializeBackgroundTask(taskID, cmd string, ctx *ExecutionContext) {
	if ctx.BackgroundTasks == nil {
		ctx.BackgroundTasks = make(map[string]*BackgroundTask)
	}

	task := &BackgroundTask{
		ID:      taskID,
		Command: cmd,
		Status:  TaskPending,
	}

	ctx.BackgroundTasks[taskID] = task
	ctx.OperationOutputs[taskID] = string(TaskPending)
	ctx.OperationResults[taskID] = false
}

// executeBackgroundTask runs the task in a goroutine and handles success/failure
func executeBackgroundTask(op Operation, cmd string, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) {
	defer ctx.BackgroundWg.Done()

	output, err := executeStandardCommand(cmd, ctx.Data, op.OutputFormat)

	ctx.BackgroundMutex.Lock()
	defer ctx.BackgroundMutex.Unlock()

	taskID := op.ID
	task := ctx.BackgroundTasks[taskID]

	if err != nil {
		handleBackgroundTaskFailure(op, task, ctx, opMap, executeOp, err, depth, debug)
	} else {
		handleBackgroundTaskSuccess(op, task, ctx, opMap, executeOp, output, depth, debug)
	}
}

// handleBackgroundTaskFailure processes a failed background task
func handleBackgroundTaskFailure(op Operation, task *BackgroundTask, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), err error, depth int, debug bool) {
	task.Status = TaskFailed
	task.Error = err.Error()
	ctx.OperationResults[op.ID] = false
	ctx.Vars["error"] = err.Error()
	ctx.OperationOutputs[op.ID] = fmt.Sprintf("Error: %s", err.Error())

	if debug {
		fmt.Printf("Background task %s failed: %v\n", op.ID, err)
	}

	if op.OnFailure != "" {
		executeFailureHandler(op, opMap, executeOp, depth, debug)
	}
}

// handleBackgroundTaskSuccess processes a successful background task
func handleBackgroundTaskSuccess(op Operation, task *BackgroundTask, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), output string, depth int, debug bool) {
	if op.Transform != "" {
		transformedOutput, transformErr := transformOutput(output, op.Transform, ctx)
		if transformErr == nil {
			output = transformedOutput
		} else if debug {
			fmt.Printf("Transform error for background task %s: %v\n", op.ID, transformErr)
		}
	}

	task.Status = TaskComplete
	task.Output = output
	ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
	ctx.OperationResults[op.ID] = true

	if output != "" && !op.Silent {
		fmt.Println(output)
	}

	if op.OnSuccess != "" {
		executeSuccessHandler(op, opMap, executeOp, depth, debug)
	}
}

// executeSuccessHandler runs the specified on_success operation
func executeSuccessHandler(op Operation, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) {
	nextOp, exists := opMap[op.OnSuccess]
	if exists {
		if debug {
			fmt.Printf("Executing on_success handler %s for background task %s\n", op.OnSuccess, op.ID)
		}
		_, _ = executeOp(nextOp, depth+1)
	}
}

// executeFailureHandler runs the specified on_failure operation
func executeFailureHandler(op Operation, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) {
	nextOp, exists := opMap[op.OnFailure]
	if exists {
		if debug {
			fmt.Printf("Executing on_failure handler %s for background task %s\n", op.OnFailure, op.ID)
		}
		_, _ = executeOp(nextOp, depth+1)
	}
}
