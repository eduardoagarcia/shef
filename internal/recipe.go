package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/google/uuid"
)

// evaluateRecipe executes a recipe with given input and variables
func evaluateRecipe(recipe Recipe, input string, vars map[string]interface{}, debug bool) error {
	ctx := &ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
		LoopStack:        make([]*LoopContext, 0),
	}

	ctx.templateFuncs = extendTemplateFuncs(templateFuncs, ctx)
	vars["context"] = ctx

	if recipe.Vars != nil {
		for k, v := range recipe.Vars {
			ctx.Vars[k] = v
		}
	}

	if recipe.Workdir != "" {
		if err := ensureWorkingDirectory(recipe.Workdir, debug); err != nil {
			return err
		}
		ctx.Vars["workdir"] = recipe.Workdir
	}

	for k, v := range vars {
		ctx.Vars[k] = v
	}

	if input != "" {
		ctx.Vars["input"] = input
		ctx.Data = input
	}

	opMap := make(map[string]Operation)

	expandedOperations, err := ExpandComponentReferences(recipe.Operations, opMap, debug)
	if err != nil {
		return fmt.Errorf("failed to expand component references: %w", err)
	}

	if debug && len(expandedOperations) != len(recipe.Operations) {
		fmt.Printf("Expanded %d operations into %d operations after component resolution\n",
			len(recipe.Operations), len(expandedOperations))
	}

	registerOperations(expandedOperations, opMap)

	handlerIDs := make(map[string]bool)
	identifyHandlers(expandedOperations, handlerIDs)

	if debug {
		printRegisteredOperations(opMap, handlerIDs)
	}

	var executeOp func(op Operation, depth int) (bool, error)
	executeOp = func(op Operation, depth int) (bool, error) {
		if depth > 50 {
			return false, fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		renderedID := op.ID
		if op.ID != "" {
			var err error
			renderedID, err = renderTemplate(op.ID, ctx.templateVars())
			if err != nil {
				return false, fmt.Errorf("failed to render operation ID template: %w", err)
			}
			if renderedID != op.ID {
				opCopy := op
				opCopy.ID = renderedID
				op = opCopy

				if debug {
					fmt.Printf("Rendered operation ID: '%s' -> '%s'\n", op.ID, renderedID)
				}
			}
		}

		// 1. Check condition
		if !shouldRunOperation(op, ctx, debug) {
			return false, nil
		}

		// 2. Handle prompts
		if err := processPrompts(op, ctx); err != nil {
			return false, err
		}

		// 3. Process control flow
		if op.ControlFlow != nil {
			exit, err := processControlFlow(op, ctx, depth, executeOp, debug)
			if err != nil {
				return op.Exit, err
			}
			if exit {
				if debug {
					fmt.Printf("Exiting recipe due to exit flag inside control flow\n")
				}
				return true, nil
			}
		}

		// 4. Prepare command
		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render command template: %w", err)
		}
		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}
		ctx.Vars["error"] = ""
		workdir := ""
		if workdirVal, exists := ctx.Vars["workdir"]; exists {
			workdir = fmt.Sprintf("%v", workdirVal)
		}

		// 5. Execute command in the background
		if op.ExecutionMode == "background" {
			if err := executeBackgroundCommand(op, ctx, opMap, executeOp, depth, debug, workdir); err != nil {
				return false, err
			}
			return op.Exit, nil
		}

		// 6. Execute command normally
		output, err := executeCommand(cmd, ctx.Data, op.ExecutionMode, op.OutputFormat, workdir)
		operationSuccess := err == nil
		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
		}

		// 7. Handle command errors
		if err != nil {
			return handleCommandError(op, ctx, opMap, executeOp, err, depth, debug)
		}

		// 8. Process command output
		return processCommandOutput(op, output, ctx, opMap, executeOp, depth, debug)
	}

	for i, op := range expandedOperations {
		if op.ID != "" && handlerIDs[op.ID] {
			if debug {
				fmt.Printf("Skipping handler operation %d: %s (ID: %s)\n", i+1, op.Name, op.ID)
			}
			continue
		}

		if debug {
			fmt.Printf("Executing operation %d: %s\n", i+1, op.Name)
		}

		shouldExit, err := executeOp(op, 0)
		if err != nil {
			return err
		}

		if shouldExit {
			if debug {
				fmt.Printf("Exiting recipe execution after operation: %s\n", op.Name)
			}
			return nil
		}
	}

	// Wait for background tasks to complete
	ctx.BackgroundWg.Wait()

	if debug {
		ctx.BackgroundMutex.RLock()
		for id, task := range ctx.BackgroundTasks {
			if task.Status == TaskComplete && task.Output != "" {
				fmt.Printf("Background task %s output: %s\n", id, task.Output)
			} else if task.Status == TaskFailed && task.Error != "" {
				fmt.Printf("Background task %s failed: %s\n", id, task.Error)
			}
		}
		ctx.BackgroundMutex.RUnlock()
	}

	return nil
}

// shouldRunOperation checks if an operation's condition is met
func shouldRunOperation(op Operation, ctx *ExecutionContext, debug bool) bool {
	if op.Condition == "" {
		return true
	}

	if debug {
		fmt.Printf("Evaluating condition: %s\n", op.Condition)
	}

	result, err := evaluateCondition(op.Condition, ctx)
	if err != nil {
		if debug {
			fmt.Printf("Condition evaluation failed: %v\n", err)
		}
		return false
	}

	if !result && debug {
		fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
	}

	return result
}

// processPrompts handles all prompts for an operation
func processPrompts(op Operation, ctx *ExecutionContext) error {
	for _, prompt := range op.Prompts {
		value, err := handlePrompt(prompt, ctx)
		if err != nil {
			return err
		}

		if value == ExitPrompt && (prompt.Type == "select" || prompt.Type == "autocomplete") {
			os.Exit(0)
		}

		varName := prompt.Name
		if prompt.ID != "" {
			varName = prompt.ID
		}
		ctx.Vars[varName] = value
	}

	return nil
}

// processControlFlow handles foreach, while, and for loops
func processControlFlow(op Operation, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error), debug bool) (bool, error) {
	flowMap, ok := op.ControlFlow.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid control_flow structure")
	}

	typeVal, ok := flowMap["type"].(string)
	if !ok {
		return false, fmt.Errorf("control_flow requires a 'type' field")
	}

	switch typeVal {
	case "foreach":
		forEach, err := op.GetForEachFlow()
		if err != nil {
			return false, err
		}
		return ExecuteForEach(op, forEach, ctx, depth, executeOp, debug)

	case "while":
		whileFlow, err := op.GetWhileFlow()
		if err != nil {
			return false, err
		}
		return ExecuteWhile(op, whileFlow, ctx, depth, executeOp, debug)

	case "for":
		forFlow, err := op.GetForFlow()
		if err != nil {
			return false, err
		}
		return ExecuteFor(op, forFlow, ctx, depth, executeOp, debug)

	default:
		return false, fmt.Errorf("unknown control_flow type: %s", typeVal)
	}
}

// handleCommandError processes errors from command execution
func handleCommandError(op Operation, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), err error, depth int, debug bool) (bool, error) {
	ctx.Vars["error"] = err.Error()

	if debug {
		fmt.Printf("Warning: command execution had errors: %v\n", err)
	}

	if op.OnFailure != "" {
		if debug {
			fmt.Printf("Executing on_failure handler: %s\n", op.OnFailure)
		}

		nextOp, exists := opMap[op.OnFailure]
		if !exists {
			return false, fmt.Errorf("on_failure operation %s not found", op.OnFailure)
		}
		shouldExit, err := executeOp(nextOp, depth+1)
		return shouldExit || op.Exit, err
	}

	fmt.Printf("Error in operation '%s': \n%v\n", op.Name, err)

	var continueExecution bool
	prompt := &survey.Confirm{
		Message: "Continue with recipe execution?",
		Default: false,
	}
	if err := survey.AskOne(prompt, &continueExecution); err != nil {
		return false, err
	}

	if !continueExecution {
		return true, fmt.Errorf("recipe execution aborted by user after command error")
	}

	return false, nil
}

// processCommandOutput handles successful command output
func processCommandOutput(op Operation, output string, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int, debug bool) (bool, error) {
	if op.Transform != "" {
		transformedOutput, err := transformOutput(output, op.Transform, ctx)
		if err != nil {
			if debug {
				fmt.Printf("Warning: output transformation failed: %v\n", err)
			}
		} else {
			output = transformedOutput
		}
	}

	ctx.Data = output

	if op.ID != "" {
		ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
	}

	if output != "" && !op.Silent {
		if ctx.ProgressMode {
			firstLine := output
			if idx := strings.Index(output, "\n"); idx >= 0 {
				firstLine = output[:idx]
			}
			fmt.Print("\r" + firstLine + " " + "\033[K")
		} else {
			fmt.Println(output)
		}
	}

	if op.OnSuccess != "" {
		nextOp, exists := opMap[op.OnSuccess]
		if !exists {
			return false, fmt.Errorf("on_success operation %s not found", op.OnSuccess)
		}
		shouldExit, err := executeOp(nextOp, depth+1)
		return shouldExit || op.Exit, err
	}

	if debug {
		printOperationDebug(op, ctx)
	}

	return op.Exit, nil
}

// printRegisteredOperations displays information about registered operations
func printRegisteredOperations(opMap map[string]Operation, handlerIDs map[string]bool) {
	fmt.Println("Registered operations:")
	for id := range opMap {
		handlerStatus := ""
		if handlerIDs[id] {
			handlerStatus = " (handler)"
		}
		fmt.Printf("  - %s%s\n", id, handlerStatus)
	}
}

// printOperationDebug prints debug information about an operation
func printOperationDebug(op Operation, ctx *ExecutionContext) {
	fmt.Printf("Operation %s result: %v\n", op.ID, ctx.OperationResults[op.ID])
	fmt.Printf("Handler for on_success: '%s'\n", op.OnSuccess)
	fmt.Printf("Handler for on_failure: '%s'\n", op.OnFailure)
	if op.Exit {
		fmt.Printf("Exit flag is set. Will exit after this operation.\n")
	}
	if op.Break {
		fmt.Printf("Break flag is set. Will break out of control flow.\n")
	}
}

// registerOperations adds operations to the operation map
func registerOperations(operations []Operation, opMap map[string]Operation) {
	for _, op := range operations {
		if op.ID != "" {
			opMap[op.ID] = op
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			registerOperations(op.Operations, opMap)
		}
	}
}

// identifyHandlers finds operations used as success or failure handlers
func identifyHandlers(operations []Operation, handlerIDs map[string]bool) {
	for _, op := range operations {
		if op.OnSuccess != "" {
			handlerIDs[op.OnSuccess] = true
		}
		if op.OnFailure != "" {
			handlerIDs[op.OnFailure] = true
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			identifyHandlers(op.Operations, handlerIDs)
		}
	}
}

// allTasksComplete returns "true" if all background tasks are complete, "false" if tasks are still running
func (ctx *ExecutionContext) allTasksComplete() string {
	ctx.BackgroundMutex.RLock()
	defer ctx.BackgroundMutex.RUnlock()

	if ctx.BackgroundTasks == nil || len(ctx.BackgroundTasks) == 0 {
		return "true"
	}

	for _, task := range ctx.BackgroundTasks {
		if task.Status != TaskComplete {
			return "false"
		}
	}
	return "true"
}

// anyTasksFailed returns "true" if any background tasks fail in execution, "false" otherwise
func (ctx *ExecutionContext) anyTasksFailed() string {
	ctx.BackgroundMutex.RLock()
	defer ctx.BackgroundMutex.RUnlock()

	if ctx.BackgroundTasks == nil {
		return "false"
	}

	for _, task := range ctx.BackgroundTasks {
		if task.Status == TaskFailed {
			return "true"
		}
	}
	return "false"
}

// pushLoopContext starts tracking a new loop
func (ctx *ExecutionContext) pushLoopContext(loopType string, depth int) *LoopContext {
	loopCtx := &LoopContext{
		ID:        uuid.New().String(),
		StartTime: time.Now(),
		Type:      loopType,
		Depth:     depth,
	}

	ctx.LoopStack = append(ctx.LoopStack, loopCtx)
	ctx.CurrentLoopIdx = len(ctx.LoopStack) - 1
	return loopCtx
}

// popLoopContext removes the current loop context
func (ctx *ExecutionContext) popLoopContext() {
	if len(ctx.LoopStack) > 0 {
		ctx.LoopStack = ctx.LoopStack[:len(ctx.LoopStack)-1]
		ctx.CurrentLoopIdx = len(ctx.LoopStack) - 1
	}
}

// updateLoopDuration updates the duration of the current loop
func (ctx *ExecutionContext) updateLoopDuration() {
	if ctx.CurrentLoopIdx >= 0 && ctx.CurrentLoopIdx < len(ctx.LoopStack) {
		loop := ctx.LoopStack[ctx.CurrentLoopIdx]
		loop.Duration = time.Since(loop.StartTime)
	}
}

// getCurrentLoopDuration gets the duration of the current loop
func (ctx *ExecutionContext) getCurrentLoopDuration() time.Duration {
	if ctx.CurrentLoopIdx >= 0 && ctx.CurrentLoopIdx < len(ctx.LoopStack) {
		return ctx.LoopStack[ctx.CurrentLoopIdx].Duration
	}
	return 0
}
