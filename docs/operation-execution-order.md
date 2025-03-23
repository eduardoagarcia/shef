## Operation Execution Order

Each operation in a Shef recipe is executed in a specific order to ensure consistent behavior and proper flow control.

### Execution Flow

Operations are executed in the following order:

1. **Condition Check**: The condition (if specified) is evaluated first. If the condition is not met, the operation is
   skipped entirely.
2. **Prompts**: All prompts are processed next, collecting user input before any other execution occurs.
3. **Control Flow**: If the operation has a control flow structure (foreach, while, for), it is executed after prompts
   are collected.
4. **Command**: The command is executed after the control flow completes.
5. **Transforms**: Any transformations are applied to the command output.
6. **Success/Failure Handlers**: Based on the operation result, either the on_success or on_failure handlers are
   executed.

This order ensures that user input is collected before any execution, control flow structures are processed completely
before running commands, and transformations are applied to command outputs.

For a detailed flow chart, check out the [Operation Flow Diagram.](additional-reference.md#flow-diagram)
