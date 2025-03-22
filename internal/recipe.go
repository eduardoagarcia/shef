package internal

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"os"
	"strings"
)

func evaluateRecipe(recipe Recipe, input string, vars map[string]interface{}, debug bool) error {
	ctx := ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
	}

	ctx.templateFuncs = extendTemplateFuncs(templateFuncs, &ctx)
	vars["context"] = &ctx

	for k, v := range vars {
		ctx.Vars[k] = v
	}

	if input != "" {
		ctx.Vars["input"] = input
		ctx.Data = input
	}

	opMap := make(map[string]Operation)
	registerOperations(recipe.Operations, opMap)

	handlerIDs := make(map[string]bool)
	identifyHandlers(recipe.Operations, handlerIDs)

	if debug {
		fmt.Println("Registered operations:")
		for id := range opMap {
			handlerStatus := ""
			if handlerIDs[id] {
				handlerStatus = " (handler)"
			}
			fmt.Printf("  - %s%s\n", id, handlerStatus)
		}
	}

	var executeOp func(op Operation, depth int) (bool, error)
	executeOp = func(op Operation, depth int) (bool, error) {
		if depth > 50 {
			return false, fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		// 1. Check the condition first
		if op.Condition != "" {
			if debug {
				fmt.Printf("Evaluating condition: %s\n", op.Condition)
			}
			result, err := evaluateCondition(op.Condition, &ctx)
			if err != nil {
				return false, fmt.Errorf("condition evaluation failed: %w", err)
			}

			if !result {
				if debug {
					fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
				}
				return false, nil
			}
		}

		// 2. Run the prompts
		for _, prompt := range op.Prompts {
			value, err := handlePrompt(prompt, &ctx)
			if err != nil {
				return false, err
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

		// 3. Run the control flow if it exists
		var controlFlowExit bool
		var controlFlowErr error
		if op.ControlFlow != nil {
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
				controlFlowExit, controlFlowErr = ExecuteForEach(op, forEach, &ctx, depth, executeOp, debug)

			case "while":
				whileFlow, err := op.GetWhileFlow()
				if err != nil {
					return false, err
				}
				controlFlowExit, controlFlowErr = ExecuteWhile(op, whileFlow, &ctx, depth, executeOp, debug)

			case "for":
				forFlow, err := op.GetForFlow()
				if err != nil {
					return false, err
				}
				controlFlowExit, controlFlowErr = ExecuteFor(op, forFlow, &ctx, depth, executeOp, debug)

			default:
				return false, fmt.Errorf("unknown control_flow type: %s", typeVal)
			}
		}

		if controlFlowErr != nil {
			return op.Exit, controlFlowErr
		}

		if controlFlowExit {
			if debug {
				fmt.Printf("Exiting recipe due to exit flag inside for control flow\n")
			}
			return true, nil
		}

		// 4. Run the command
		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render command template: %w", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		ctx.Vars["error"] = ""

		if op.ExecutionMode == "background" {
			if err := executeBackgroundCommand(op, &ctx, opMap, executeOp, depth, debug); err != nil {
				return false, err
			}
			return op.Exit, nil
		}

		output, err := executeCommand(cmd, ctx.Data, op.ExecutionMode, op.OutputFormat)
		operationSuccess := err == nil

		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
		}

		if err != nil {
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
		}

		// 5. Run the transforms
		if op.Transform != "" {
			transformedOutput, err := transformOutput(output, op.Transform, &ctx)
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

		// 7. Run the on_success handler
		if op.OnSuccess != "" && operationSuccess {
			nextOp, exists := opMap[op.OnSuccess]
			if !exists {
				return false, fmt.Errorf("on_success operation %s not found", op.OnSuccess)
			}
			shouldExit, err := executeOp(nextOp, depth+1)
			return shouldExit || op.Exit, err
		}

		if debug {
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

		return op.Exit, nil
	}

	for i, op := range recipe.Operations {
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

	ctx.BackgroundWg.Wait()
	ctx.BackgroundMutex.RLock()
	for id, task := range ctx.BackgroundTasks {
		if task.Status == TaskComplete {
			var op *Operation
			for _, recipeOp := range recipe.Operations {
				if recipeOp.ID == id {
					op = &recipeOp
					break
				}
			}
			if op == nil || !op.Silent {
				if task.Output != "" && debug {
					fmt.Printf("Background task %s output: %s\n", id, task.Output)
				}
			}
		} else if task.Status == TaskFailed && task.Error != "" && debug {
			fmt.Printf("Background task %s failed: %s\n", id, task.Error)
		}
	}
	ctx.BackgroundMutex.RUnlock()

	return nil
}

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
