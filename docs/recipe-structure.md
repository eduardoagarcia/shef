## Recipe Structure

Recipes are defined in YAML files:

```yaml
recipes:
  - name: "example"
    description: "An example recipe"
    category: "demo"
    help: |
      This is detailed help text for the example recipe.

      It can include multiple paragraphs and shows when users run:
        shef demo example -h
    operations:
      - name: "First Operation"
        id: "first_op"
        command: echo "Hello, World!"

      - name: "Second Operation"
        id: "second_op"
        command: ls -la
```

### Key Recipe Components

- **name**: Unique identifier for the recipe
- **description**: Human-readable description
- **category**: Used for organization and filtering
- **author**: Optional author attribution
- **help**: Detailed help documentation shown when using `-h` or `--help` flags
- **operations**: List of operations to execute in sequence

### Operations

Operations are the building blocks of recipes:

```yaml
- name: "Operation Name"            # Operation name
  id: "var_id"                      # [Optional] Identifier for referencing the variable for the operation
  command: echo "Hello"             # [Optional] Shell command to execute
  execution_mode: "standard"        # [Optional] How the command runs (standard, interactive, stream, or background)
  output_format: "raw"              # [Optional] How to format command output (raw [default], trim, or lines)
  silent: false                     # [Optional] Flag whether to suppress output to stdout. Default is false.
  exit: false                       # [Optional] When set to true, the recipe will exit after the operation completes. Default is false.
  condition: .var == "true"         # [Optional] Condition for execution
  on_success: "success_op"          # [Optional] Operation to run on success (if not defined, Shef continues to next operation)
  on_failure: "failure_op"          # [Optional] Operation to run on failure
  transform: "{{ trim .output }}"   # [Optional] Transform output
  prompts:                          # [Optional] Interactive prompts (can include one or more prompts)
     - name: "Prompt Name"
       id: "var_id"
       type: "input"
       message: "Enter value:"
  control_flow:                     # [Optional] Control flow structure
    type: "foreach"                 # Type of control flow (foreach, for, while)
  operations:                       # [Optional] Sub-operations for control flows
    - name: "Sub Operation"
      command: echo "Processing " {{ .item }}
      break: false                  # [Optional] When true, break out of a control flow and resume recipe
```

#### Execution Modes

- **standard**: Default mode (used when no `execution_mode` is specified). Output is captured and can be used by
  subsequent operations.
- **interactive**: The command has direct access to the terminal's stdin, stdout, and stderr. Useful for commands that
  require direct terminal interaction, but output cannot be captured for use in subsequent operations.
- **stream**: Similar to interactive mode but optimized for long-running processes that produce continuous output. The
  command's output streams to the terminal in real-time, but output cannot be captured for use in subsequent operations.
- **background**: Executes the command asynchronously in a separate process. The recipe execution continues immediately
  without waiting for the command to complete. Useful for long-running tasks that don't need to block recipe execution.

### Background Task Management

When using `execution_mode: "background"`, Shef provides template functions to monitor and interact with background
tasks:

- **bgTaskStatus**: Returns the current status of a background task (`pending`, `complete`, or `failed`)
- **bgTaskComplete**: Returns `true` if the task has completed successfully, `false` otherwise
- **bgTaskFailed**: Returns `true` if the task has failed, `false` otherwise

#### Requirements for Background Tasks

- Each background task must have a unique `id` specified
- The task's output will be available as a variable using the task's ID once completed

#### Task Status Checking

##### Check a task's status

```yaml
- name: "Check Status"
  command:
    echo 'Task status: {{ bgTaskStatus "task_id" }}'
```

> [!IMPORTANT]
> Notice we reference the task id by a _string_ when checking status, complete, and failed states.

##### Wait for task completion

```yaml
- name: "Wait For Task"
  control_flow:
    type: "while"
    condition: '{{ not (bgTaskComplete "task_id") }}'
  operations:
    - name: "Poll Status"
      command: echo "Waiting for task to complete..."

    - name: "Small Delay"
      command: sleep 1
```

#### Task Output Access

Once a background task completes, its output is available like any other operation:

```yaml
- name: "Echo Task Output"
  command: echo "Task result {{ .task_id }}"
  condition: '{{ bgTaskComplete "task_id" }}'
```

#### Background Task Completion Behavior

When you start a background task with `execution_mode: "background"`, it's important to understand how Shef handles task
completion:

- **Implicit Waiting**: Shef automatically waits for all background tasks to complete before terminating the recipe
  execution, even if you don't explicitly wait for them in your operations.
- **Output Capture**: All background tasks will have their outputs captured and made available as variables, regardless
  of whether you explicitly check their status.
- **Completion Order**: Background tasks complete in the order determined by their execution time, not the order they
  were started.
- **Recipe Exit**: The recipe won't exit until all background tasks have completed, which could cause the recipe to
  appear to "hang" if a background task takes a very long time.

Example of implicit waiting:

```yaml
- name: "Start Long Task"
  id: "long_task"
  command: "sleep 30 && echo 'Done!'"
  execution_mode: "background"

- name: "Immediate Feedback"
  command: echo "Started background task! Recipe will wait for it to complete before exiting."
```

### Output Format

Shef provides options for controlling how whitespace and empty lines are handled in command output:

- **raw**: The default mode. Preserves all whitespace and newlines exactly as produced by the command, behaving like a
  standard bash script.
- **trim**: Removes leading and trailing whitespace and newlines from the command output, useful for cleaner output when
  exact formatting isn't needed.
- **lines**: Splits the output by newlines, trims each line, removes empty lines, and joins them back together. Useful
  for processing lists where empty lines and whitespace should be ignored.

##### Example Usage

```yaml
operations:
  - name: "Get file content with preserved formatting"
    id: "read_file"
    command: cat config.yaml
    output_format: "raw"  # Preserves all whitespace and newlines

  - name: "Get version number"
    id: "get_version"
    command: echo "  v1.2.3\n\n"
    output_format: "trim"  # Result: "v1.2.3"

  - name: "Get clean list of items"
    id: "list_items"
    command: echo "  item1  \n\n\n\n\n      item2  \n\n  item3    "
    output_format: "lines"  # Result: "item1\nitem2\nitem3"
```

### Control Flow Configuration

Control flow structures are configured as follows:

#### Foreach

```yaml
control_flow:
  type: "foreach"               # Iterate over a collection
  collection: "Item 1\nItem 2"  # Collection of items (newline separated)
  as: "item"                    # Variable name for current item
```

#### For

```yaml
control_flow:
  type: "for"    # Execute a fixed number of times
  count: 5       # Number of iterations
  variable: "i"  # Variable name for iteration index (optional)
```

#### While

```yaml
control_flow:
  type: "while"                  # Execute while condition is true
  condition: .status != "ready"  # Condition to evaluate each iteration
```
