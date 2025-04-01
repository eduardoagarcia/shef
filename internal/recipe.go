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
func evaluateRecipe(recipe Recipe, input string, vars map[string]interface{}) error {
	Log(CategoryRecipe, "Starting recipe evaluation", map[string]interface{}{
		"name":      recipe.Name,
		"input":     input,
		"varsCount": len(vars),
	})

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
		Log(CategoryRecipe, fmt.Sprintf("Adding %d recipe variables", len(recipe.Vars)))
		for k, v := range recipe.Vars {
			ctx.Vars[k] = v
		}
	}

	if recipe.Workdir != "" {
		Log(CategoryFileSystem, fmt.Sprintf("Setting working directory: %s", recipe.Workdir))
		if err := ensureWorkingDirectory(recipe.Workdir); err != nil {
			LogError("Failed to create working directory", err, map[string]interface{}{"workdir": recipe.Workdir})
			return err
		}
		ctx.Vars["workdir"] = recipe.Workdir
	}

	Log(CategoryRecipe, fmt.Sprintf("Adding %d external variables", len(vars)))
	for k, v := range vars {
		ctx.Vars[k] = v
	}

	if input != "" {
		Log(CategoryRecipe, "Setting input data")
		ctx.Vars["input"] = input
		ctx.Data = input
	}

	opMap := make(map[string]Operation)

	Log(CategoryComponent, "Expanding component references")
	expandedOperations, err := ExpandComponentReferences(recipe.Operations, opMap)
	if err != nil {
		LogError("Failed to expand component references", err, nil)
		return fmt.Errorf("failed to expand component references: %w", err)
	}

	if len(expandedOperations) != len(recipe.Operations) {
		Log(CategoryComponent, fmt.Sprintf("Expanded %d operations into %d operations",
			len(recipe.Operations), len(expandedOperations)))
	}

	registerOperations(expandedOperations, opMap)

	handlerIDs := make(map[string]bool)
	identifyHandlers(expandedOperations, handlerIDs)

	printRegisteredOperations(opMap, handlerIDs)

	var executeOp func(op Operation, depth int) (bool, error)
	executeOp = func(op Operation, depth int) (bool, error) {
		if depth > 50 {
			LogError("Possible infinite loop detected", nil, map[string]interface{}{"depth": depth})
			return false, fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		LogOperation(op.Name, op.ID, map[string]interface{}{"depth": depth})
		IncreaseIndent()
		defer DecreaseIndent()

		renderedID := op.ID
		if op.ID != "" {
			var err error
			originalID := op.ID
			renderedID, err = renderTemplate(op.ID, ctx.templateVars())
			if err != nil {
				LogError("Failed to render operation ID template", err, nil)
				return false, fmt.Errorf("failed to render operation ID template: %w", err)
			}
			if renderedID != op.ID {
				opCopy := op
				opCopy.ID = renderedID
				op = opCopy

				Log(CategoryTemplate, fmt.Sprintf("Rendered operation ID: '%s' -> '%s'", originalID, renderedID))
			}
		}

		// 1. Check condition
		if !shouldRunOperation(op, ctx) {
			return false, nil
		}

		// 2. Handle prompts
		if err := processPrompts(op, ctx); err != nil {
			return false, err
		}

		// 3. Process control flow
		if op.ControlFlow != nil {
			exit, err := processControlFlow(op, ctx, depth, executeOp)
			if err != nil {
				return op.Exit, err
			}
			if exit {
				Log(CategoryControlFlow, "Exiting recipe due to exit flag inside control flow")
				return true, nil
			}
		}

		// 4. Prepare command
		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render command template: %w", err)
		}
		LogCommand(cmd, nil)
		ctx.Vars["error"] = ""
		workdir := ""
		if workdirVal, exists := ctx.Vars["workdir"]; exists {
			workdir = fmt.Sprintf("%v", workdirVal)
		}

		// 5. Execute command in the background
		if op.ExecutionMode == "background" {
			if err := executeBackgroundCommand(op, ctx, opMap, executeOp, depth, workdir); err != nil {
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
			return handleCommandError(op, ctx, opMap, executeOp, err, depth)
		}

		// 8. Process command output
		return processCommandOutput(op, output, ctx, opMap, executeOp, depth)
	}

	for i, op := range expandedOperations {
		if op.ID != "" && handlerIDs[op.ID] {
			Log(CategoryOperation, fmt.Sprintf("Skipping handler operation %d: %s (ID: %s)", i+1, op.Name, op.ID))
			continue
		}

		Log(CategoryOperation, fmt.Sprintf("Executing operation %d: %s", i+1, op.Name))

		shouldExit, err := executeOp(op, 0)
		if err != nil {
			return err
		}

		if shouldExit {
			Log(CategoryRecipe, fmt.Sprintf("Exiting recipe execution after operation: %s", op.Name))
			return nil
		}
	}

	// Wait for background tasks to complete
	ctx.BackgroundWg.Wait()

	ctx.BackgroundMutex.RLock()
	for id, task := range ctx.BackgroundTasks {
		if task.Status == TaskComplete && task.Output != "" {
			LogBackgroundTask(id, "completed", map[string]interface{}{"output": task.Output})
		} else if task.Status == TaskFailed && task.Error != "" {
			LogBackgroundTask(id, "failed", map[string]interface{}{"error": task.Error})
		}
	}
	ctx.BackgroundMutex.RUnlock()

	return nil
}

// shouldRunOperation checks if an operation's condition is met
func shouldRunOperation(op Operation, ctx *ExecutionContext) bool {
	if op.Condition == "" {
		return true
	}

	Log(CategoryCondition, fmt.Sprintf("Evaluating condition: %s", op.Condition))

	result, err := evaluateCondition(op.Condition, ctx)
	if err != nil {
		LogError("Condition evaluation failed", err, map[string]interface{}{"condition": op.Condition})
		return false
	}

	LogCondition(op.Condition, result, map[string]interface{}{"operation": op.Name})
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
		ctx.OperationOutputs[varName] = fmt.Sprintf("%v", value)
	}

	return nil
}

// processControlFlow handles foreach, while, and for loops
func processControlFlow(op Operation, ctx *ExecutionContext, depth int, executeOp func(Operation, int) (bool, error)) (bool, error) {
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
		return ExecuteForEach(op, forEach, ctx, depth, executeOp)

	case "while":
		whileFlow, err := op.GetWhileFlow()
		if err != nil {
			return false, err
		}
		return ExecuteWhile(op, whileFlow, ctx, depth, executeOp)

	case "for":
		forFlow, err := op.GetForFlow()
		if err != nil {
			return false, err
		}
		return ExecuteFor(op, forFlow, ctx, depth, executeOp)

	default:
		return false, fmt.Errorf("unknown control_flow type: %s", typeVal)
	}
}

// handleCommandError processes errors from command execution
func handleCommandError(op Operation, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), err error, depth int) (bool, error) {
	ctx.Vars["error"] = err.Error()

	LogError("Command execution error", err, map[string]interface{}{
		"operation": op.Name,
		"id":        op.ID,
	})

	if op.OnFailure != "" {
		Log(CategoryOperation, fmt.Sprintf("Executing on_failure handler: %s", op.OnFailure))

		nextOp, exists := opMap[op.OnFailure]
		if !exists {
			LogError("On_failure handler not found", nil, map[string]interface{}{"handler": op.OnFailure})
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
		Log(CategoryRecipe, "Recipe execution aborted by user after command error")
		return true, fmt.Errorf("recipe execution aborted by user after command error")
	}

	return false, nil
}

// processCommandOutput handles successful command output
func processCommandOutput(op Operation, output string, ctx *ExecutionContext, opMap map[string]Operation, executeOp func(Operation, int) (bool, error), depth int) (bool, error) {
	LogOutput(output, map[string]interface{}{
		"operation": op.Name,
		"id":        op.ID,
	})

	if op.Transform != "" {
		Log(CategoryTransform, fmt.Sprintf("Applying transformation: %s", op.Transform))
		transformedOutput, err := transformOutput(output, op.Transform, ctx)
		if err != nil {
			LogError("Output transformation failed", err, map[string]interface{}{"transform": op.Transform})
		} else {
			output = transformedOutput
			Log(CategoryTransform, "Transformation applied successfully")
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
		Log(CategoryOperation, fmt.Sprintf("Executing on_success handler: %s", op.OnSuccess))
		nextOp, exists := opMap[op.OnSuccess]
		if !exists {
			LogError("On_success handler not found", nil, map[string]interface{}{"handler": op.OnSuccess})
			return false, fmt.Errorf("on_success operation %s not found", op.OnSuccess)
		}
		shouldExit, err := executeOp(nextOp, depth+1)
		return shouldExit || op.Exit, err
	}

	if op.Exit {
		Log(CategoryOperation, "Exit flag set, will exit after this operation")
	}
	if op.Break {
		Log(CategoryOperation, "Break flag set, will break out of control flow")
	}

	return op.Exit, nil
}

// printRegisteredOperations displays information about registered operations
func printRegisteredOperations(opMap map[string]Operation, handlerIDs map[string]bool) {
	Log(CategoryInit, fmt.Sprintf("Registered %d operations", len(opMap)))
	for id := range opMap {
		isHandler := handlerIDs[id]
		Log(CategoryInit, fmt.Sprintf("Registered operation: %s", id),
			map[string]interface{}{"is_handler": isHandler})
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
