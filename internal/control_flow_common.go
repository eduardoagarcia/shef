package internal

import (
	"fmt"
)

// executeLoopOperations runs all operations for a single iteration.
func executeLoopOperations(operations []Operation, ctx *ExecutionContext, depth int,
	executeOp func(Operation, int) (bool, error)) (exit bool, breakLoop bool) {

	for _, subOp := range operations {
		if !shouldRunOperation(subOp, ctx) {
			continue
		}

		shouldExit, err := executeOp(subOp, depth+1)
		if err != nil {
			return shouldExit, false
		}

		if shouldExit || subOp.Exit {
			Log(CategoryControlFlow, fmt.Sprintf("Exiting entire recipe due to exit flag in '%s'", subOp.Name))
			return true, false
		}

		if subOp.Break {
			Log(CategoryControlFlow, fmt.Sprintf("Breaking out of loop due to break flag in '%s'", subOp.Name))
			return false, true
		}
	}

	return false, false
}

// setupProgressMode configures progress mode for control flow execution.
func setupProgressMode(ctx *ExecutionContext, useProgressMode bool) (originalMode bool) {
	originalMode = ctx.ProgressMode
	if useProgressMode {
		ctx.ProgressMode = true
	}
	return originalMode
}

// cleanupLoopState removes loop variables and sets operation result.
func cleanupLoopState(ctx *ExecutionContext, opID string, varName string) {
	delete(ctx.Vars, varName)
	delete(ctx.Vars, "iteration")

	if opID != "" {
		ctx.OperationResults[opID] = true
	}
}
