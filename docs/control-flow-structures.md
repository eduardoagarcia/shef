## Control Flow Structures

Shef supports advanced control flow structures that let you create dynamic, iterative workflows.

### Foreach Loops

You can iterate over a collection of items and perform a flow of operations on each item.

#### Key Foreach Components

- **control_flow**
    - **type**: foreach
    - **collection**: The list of items to iterate over (string with items separated by newlines)
    - **as**: The variable name to use for the current item in each iteration
- **operations**: The sub-operations to perform for each item (all sub-operations have access to the `as` loop variable)

#### Mechanics of the Foreach Loop

1. Parse the collection into separate items (splitting by newlines)
2. For each item, set the variable specified in "as"
3. Execute all operations in the foreach block for each item
4. Clean up the loop variable when done

#### Common Uses

- Processing multiple files
- Handling lists of servers, containers, or resources
- Applying the same transformation to multiple inputs
- Building dynamic workflows based on discovered items

> [!TIP]
> Within a foreach loop, you can use conditional operations, transformations, and all other Shef features.

#### Example Foreach Recipes

```yaml
- name: "Process Each Item"
  control_flow:
     type: "foreach"
     collection: "🍎 Apples\n🍌 Bananas\n🍒 Cherries\n🍊 Oranges"
     as: "fruit"  # Each item will be available as .fruit
  operations:
     - name: "Process Fruit"
       command: echo "Processing {{ .fruit }}"
```

You can also generate the collection dynamically:

```yaml
- name: "List Files"
  id: "files"
  command: find . -type f -name "*.txt"

- name: "Process Each File"
  control_flow:
    type: "foreach"
    collection: "{{ .files }}"  # Using output from previous operation
    as: "file"                  # Each item will be available as .file
  operations:
    - name: "Process File"
      command: cat {{ .file }} | wc -l
```

### For Loops

You can execute a set of operations a fixed number of times.

#### Key For Loop Components

- **control_flow**
    - **type**: for
    - **count**: The number of iterations to execute
    - **variable**: (Optional) The variable name to use for the current iteration index (defaults to "i")
- **operations**: The sub-operations to perform for each iteration

#### Mechanics of the For Loop

1. Parse and evaluate the count value to determine the number of iterations
2. For each iteration, set the loop variable to the current index (starting from 0)
3. Also set the `.iteration` variable to the 1-based iteration number
4. Execute all operations in the operations block for each iteration
5. Clean up the loop variables when done

#### Common Uses

- Repeating an operation a fixed number of times
- Creating numbered resources or items
- Running tests multiple times
- Implementing retry logic with a maximum attempt limit

> [!TIP]
> Within a for loop, both the specified variable (zero-based index) and `.iteration` (one-based counter) are available.

#### Example For Loop Recipes

```yaml
- name: "Run a For Loop"
  control_flow:
    type: "for"
    count: 5
    variable: "i"
  operations:
    - name: "Print Iteration"
      command: 'echo "Running iteration {{ .iteration }} (zero-based index: {{ .i }})"'
```

You can also use a dynamic count:

```yaml
- name: "Get Count"
  id: "count"
  command: echo "3"
  transform: "{{ trim .input }}"

- name: "Dynamic For Loop"
  control_flow:
    type: "for"
    count: "{{ .count }}"
    variable: "step"
  operations:
    - name: "Execute Step"
      command: echo "Executing step {{ .step }} of {{ .count }}"
```

### While Loops

You can repeatedly execute operations as long as a condition remains true.

#### Key While Loop Components

- **control_flow**
    - **type**: while
    - **condition**: The condition to evaluate before each iteration
- **operations**: The sub-operations to perform for each iteration

#### Mechanics of the While Loop

1. Evaluate the condition before each iteration
2. If the condition is true, execute the operations and repeat
3. If the condition is false, exit the loop
4. An `.iteration` variable is automatically set to track the current iteration (starting from 1)
5. A safety limit prevents infinite loops (maximum 1000 iterations)

#### Common Uses

- Polling for a condition (e.g., waiting for a service to be ready)
- Processing data until a certain state is reached
- Implementing retry logic with conditional termination
- Continuously monitoring resources until a specific event occurs

> [!TIP]
> Within a while loop, the `.iteration` variable lets you track how many iterations have occurred.

#### Example While Loop Recipes

```yaml
- name: "Initialize Status"
  id: "status"
  command: echo "running"
  transform: "{{ trim .input }}"
  silent: true

- name: "Wait For Completion"
  control_flow:
    type: "while"
    condition: .status == "running"
  operations:
    - name: "Check Status"
      command: echo "Checking status (iteration {{ .iteration }})..."
      id: "status"
      transform: "{{ if eq .iteration 5 }}completed{{ else }}running{{ end }}"
```

Real-world polling example:

```yaml
- name: "Poll Service Until Ready"
  control_flow:
    type: "while"
    condition: .status != "ready"
  operations:
    - name: "Check Service Status"
      id: "status"
      command: curl -s http://service/status
      transform: "{{ trim .output }}"
```

### Duration Tracking in Loops

All loop types in Shef (`for`, `foreach`, and `while`) automatically track duration. This allows you to measure
execution time, implement timeouts, or provide progress feedback in your recipes.

#### Available Duration Variables

Inside any loop, the following variables are available:

| Variable          | Type   | Description                             | Example                      |
|-------------------|--------|-----------------------------------------|------------------------------|
| `duration_ms`     | String | Total milliseconds elapsed              | "12345"                      |
| `duration_s`      | String | Total seconds elapsed (as whole number) | "12"                         |
| `duration`        | String | Formatted time (MM:SS or HH:MM:SS)      | "00:12" or "1:23:45"         |
| `duration_ms_fmt` | String | Formatted time with milliseconds        | "00:12.345" or "1:23:45.678" |

#### Usage Examples

##### Displaying progress with timing:

```yaml
operations:
  - name: "Process Files with Duration Tracking"
    control_flow:
      type: foreach
      collection: "{{ exec `find . -type f -name '*.txt' | sort` }}"
      as: file
    operations:
      - name: "Process file with duration info"
        command: "echo 'Processing {{ .file }} (elapsed: {{ .duration }})'"
```

##### Implementing a timeout condition:

```yaml
operations:
  - name: "Loop with timeout"
    control_flow:
      type: while
      condition: .duration_s < 30  # Timeout after 30 seconds
    operations:
      - name: "Do something until timeout"
        command: "echo 'Working... ({{ .duration }} elapsed)'"
        
      - name: "Wait a bit"
        command: "sleep 1"
```

##### Using duration for performance testing:

```yaml
operations:
  - name: "Performance test"
    id: perf_test
    control_flow:
      type: for
      count: "100"
      variable: i
    operations:
      - name: "Run test iteration"
        command: "your_command --iteration {{ i }}"
        
      - name: "Display progress"
        command: "echo 'Completed {{ .iteration }}/100 in {{ .duration }}'"
        
  - name: "Show results"
    command: "echo 'Test completed in {{ perf_test.duration_ms_fmt }}'"
```

> [!NOTE]
> Duration variables persist after the loop completes, allowing you to access the total execution time of the loop in
> subsequent operations.

### Progress Mode

Progress mode allows operations within loops to update in-place on a single line, creating a cleaner interface for
status updates and progress indicators.

#### Key Progress Mode Features

- **control_flow**
    - **progress_mode**: When set to `true`, enables single-line updates for all operations in the loop
- **Behavior**: Each operation's output replaces the previous output on the same line rather than printing new lines
- **Limitations**: Only the first line of output is displayed; additional lines are ignored

#### Mechanics of Progress Mode

1. When a loop has `progress_mode: true`, all operations within the loop display their output on a single line
2. Each new output overwrites the previous output (returning to the start of the line)
3. At the end of the loop, a newline is automatically added to maintain clean formatting

#### Common Uses

- Displaying progress counters (e.g., "Processing 5/100...")
- Showing status updates for long-running operations
- Creating animated loading indicators
- Providing real-time feedback without cluttering the terminal

> [!TIP]
> Progress mode works best for simple, concise status messages. For complex multi-line output, standard mode is more
appropriate.

#### Example Progress Mode Recipes

With a for loop:

```yaml
- name: "Download Progress Example"
  control_flow:
    type: "for"
    count: 100
    variable: "i"
    progress_mode: true
  operations:
    - name: "Update Progress"
      command: echo "Downloading... {{ .i }}% complete"
```

With a while loop:

```yaml
- name: "Service Monitor Example"
  control_flow:
    type: "while"
    condition: .duration_s < 60  # Monitor for 60 seconds
    progress_mode: true
  operations:
    - name: "Check Service Status"
      command: echo "Monitoring service... ({{ .duration_fmt }} elapsed)"

    - name: "Wait"
      command: sleep 1
```

With a foreach loop:

```yaml
- name: "Batch Processing Example"
  control_flow:
    type: "foreach"
    collection: "{{ .files }}"
    as: "file"
    progress_mode: true
  operations:
    - name: "Process File"
      command: echo "Processing {{ .file }}"
```
