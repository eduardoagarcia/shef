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

func executeInteractiveCommand(cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		err := cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return "", err
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			err := cmd.Process.Kill()
			if err != nil {
				return "", err
			}
		}
		return "", nil

	case err := <-done:
		return "", err
	}
}

func executeBackgroundCommand(op Operation, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) error {
	if op.ID == "" {
		return fmt.Errorf("background execution requires an operation ID")
	}

	cmd, err := renderTemplate(op.Command, ctx.templateVars())
	if err != nil {
		return fmt.Errorf("failed to render command template: %w", err)
	}

	if debug {
		fmt.Printf("Starting background command: %s\n", cmd)
	}

	ctx.BackgroundMutex.Lock()
	if ctx.BackgroundTasks == nil {
		ctx.BackgroundTasks = make(map[string]*BackgroundTask)
	}

	task := &BackgroundTask{
		ID:      op.ID,
		Command: cmd,
		Status:  TaskPending,
	}

	ctx.BackgroundTasks[op.ID] = task
	ctx.OperationOutputs[op.ID] = string(TaskPending)
	ctx.OperationResults[op.ID] = false
	ctx.BackgroundMutex.Unlock()
	ctx.BackgroundWg.Add(1)

	go func() {
		defer ctx.BackgroundWg.Done()

		output, err := executeStandardCommand(cmd, ctx.Data, op.OutputFormat)

		ctx.BackgroundMutex.Lock()
		if err != nil {
			task.Status = TaskFailed
			task.Error = err.Error()
			ctx.OperationResults[op.ID] = false
			ctx.Vars["error"] = err.Error()

			ctx.OperationOutputs[op.ID] = fmt.Sprintf("Error: %s", err.Error())

			if debug {
				fmt.Printf("Background task %s failed: %v\n", op.ID, err)
			}

			if op.OnFailure != "" {
				nextOp, exists := opMap[op.OnFailure]
				if exists {
					if debug {
						fmt.Printf("Executing on_failure handler %s for background task %s\n", op.OnFailure, op.ID)
					}
					_, executeOpErr := executeOp(nextOp, depth+1)
					if executeOpErr != nil {
						return
					}
				}
			}
		} else {
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
				nextOp, exists := opMap[op.OnSuccess]
				if exists {
					if debug {
						fmt.Printf("Executing on_success handler %s for background task %s\n", op.OnSuccess, op.ID)
					}
					_, executeOpErr := executeOp(nextOp, depth+1)
					if executeOpErr != nil {
						return
					}
				}
			}
		}
		ctx.BackgroundMutex.Unlock()
	}()

	return nil
}
