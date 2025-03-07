# Shef

Shef is a powerful CLI tool that lets you combine shell commands into reusable recipes. Think of it as "[CyberChef](https://gchq.github.io/CyberChef) for
your terminal" - chain commands together, add interactive prompts, and create a toolbelt of common workflows.

## Features

- **Command Piping**: Chain commands together, passing output from one to the next
- **Transformations**: Transform command output with powerful templating
- **Interactive Prompts**: Add user input, selections, and confirmations
- **Conditional Logic**: Use if/else branching based on command results
- **Organized Recipes**: Categorize and share your recipes
- **Multiple Sources**: Use local, user, or public recipes

## Installation

```bash
# Install with Go
go install github.com/eduardoagarcia/shef@latest

# Or clone and build
git clone git@github.com:eduardoagarcia/shef.git
cd shef
go build -o shef
```

## Quick Start

```bash
# Download (or update) all public recipes
shef update

# Run a recipe
shef git version

# List available recipes
shef -l

# List recipes in a category
shef -l git
```

## Recipe Sources

Shef looks for recipes in multiple locations:

1. **Local Recipes**: `./.shef/*.yaml` in the current directory
2. **User Recipes**: `~/.shef/user/*.yaml` in your home directory
3. **Public Recipes**: `~/.shef/public/*.yaml` in your home directory

To prioritize a specific source:

```bash
shef -L git version  # Prioritize local recipes
shef -U git version  # Prioritize user recipes
shef -P git version  # Prioritize public recipes
```

## Recipe Structure

Recipes are defined in YAML files:

```yaml
recipes:
  - name: "example"
    description: "An example recipe"
    category: "demo"
    operations:
      - name: "First Operation"
        id: "first_op"
        command: "echo 'Hello, World!'"
      
      - name: "Second Operation"
        command: "grep 'Hello'"
```

### Key Recipe Components

- **name**: Unique identifier for the recipe
- **description**: Human-readable description
- **category**: Used for organization and filtering
- **operations**: List of operations to execute in sequence

### Operations

Operations are the building blocks of recipes:

```yaml
- name: "Operation Name"
  id: "unique_id"           # Optional identifier for referencing output
  command: "echo 'Hello'"   # Shell command to execute
  condition: "var == true"  # Optional condition for execution
  on_success: "success_op"  # Operation to run on success
  on_failure: "failure_op"  # Operation to run on failure
  transform: "{{ .input | trim }}"  # Transform output
  prompts:                  # Interactive prompts
    - name: "var_name"
      type: "input"
      message: "Enter value:"
```

## Interactive Prompts

Shef supports three types of prompts:

### Text Input

```yaml
- name: "username"
  type: "input"
  message: "Enter your username:"
  default: "admin"
```

### Selection

```yaml
- name: "environment"
  type: "select"
  message: "Select environment:"
  options:
    - "dev"
    - "staging"
    - "production"
  default: "dev"
```

### Confirmation

```yaml
- name: "confirm_deploy"
  type: "confirm"
  message: "Deploy to production?"
  default: "false"
```

### Dynamic Options

You can generate selection options from a previous operation's output:

```yaml
- name: "List Files"
  id: "files_list"
  command: "find . -name '*.go'"

- name: "Select File"
  command: "cat {{ .file }}"
  prompts:
    - name: "file"
      type: "select"
      message: "Select a file:"
      source_operation: "files_list"
      source_transform: "{{ .input | trim }}"
```

## Transformations

Transformations let you modify command output before it's passed to the next operation or used in prompts.

### Basic Syntax

```yaml
transform: "{{ .input | function1 | function2 }}"
```

### Available Transformation Functions

| Function     | Description                      | Example                                          |
|--------------|----------------------------------|--------------------------------------------------|
| `trim`       | Remove whitespace                | `{{ .input \| trim }}`                           |
| `split`      | Split string by delimiter        | `{{ .input \| split "," }}`                      |
| `join`       | Join array with delimiter        | `{{ .input \| join "\n" }}`                      |
| `filter`     | Keep lines containing a pattern  | `{{ .input \| filter "pattern" }}`               |
| `grep`       | Alias for filter                 | `{{ .input \| grep "pattern" }}`                 |
| `cut`        | Extract field from each line     | `{{ .input \| cut ":" 1 }}`                      |
| `trimPrefix` | Remove prefix                    | `{{ .input \| trimPrefix "foo" }}`               |
| `trimSuffix` | Remove suffix                    | `{{ .input \| trimSuffix "bar" }}`               |
| `contains`   | Check if string contains pattern | `{{ if contains .input "pattern" }}yes{{ end }}` |
| `replace`    | Replace text                     | `{{ .input \| replace "old" "new" }}`            |
| `atoi`       | Convert string to int            | `{{ .input \| atoi }}`                           |
| `add`        | Add numbers                      | `{{ .input \| atoi \| add 5 }}`                  |
| `sub`        | Subtract numbers                 | `{{ $num \| sub 3 }}`                            |
| `div`        | Divide numbers                   | `{{ $num \| div 2 }}`                            |
| `mul`        | Multiply numbers                 | `{{ $num \| mul 4 }}`                            |
| `exec`       | Execute command                  | `{{ exec "date" }}`                              |

### Accessing Variables

You can access all context variables in transformations:

```yaml
transform: "{{ if eq .format \"json\" }}{{ .input }}{{ else }}{{ .input | cut \" \" 0 }}{{ end }}"
```

## Conditional Execution

Operations can be conditionally executed:

### Basic Conditions

```yaml
condition: "confirm == true"  # Run only if confirm prompt is true
```

### Operation Result Conditions

```yaml
condition: "build_op.success"  # Run if build_op succeeded
condition: "test_op.failure"   # Run if test_op failed
```

### Complex Logic

```yaml
condition: "build_op.success && confirm_deploy == true"
condition: "test_op.failure || lint_op.failure"
condition: "!skip_validation"
```

## Branching Workflows

You can create branching workflows based on success or failure:

```yaml
- name: "Build App"
  id: "build_op"
  command: "make build"
  on_success: "deploy_op"  # Go to deploy_op on success
  on_failure: "fix_op"     # Go to fix_op on failure

- name: "Deploy"
  id: "deploy_op"
  command: "make deploy"
  
- name: "Fix Issues"
  id: "fix_op"
  command: "make lint"
```

## Data Flow Between Operations

Each operation's output is automatically piped to the next operation's input. You can also access any operation's output
by its ID:

```yaml
- name: "Get Hostname"
  id: "hostname_op"
  command: "hostname"

- name: "Show Info"
  command: "echo 'Running on {{ .operationOutputs.hostname_op }}'"
```

## Command Reference

### Basic Command Structure

```
shef [category] recipe_name [flags]
```

### Global Flags

| Flag             | Description                |
|------------------|----------------------------|
| `-h, --help`     | Show help information      |
| `-v, --version`  | Show version information   |
| `-l, --list`     | List available recipes     |
| `-d, --debug`    | Enable debug output        |
| `-c, --category` | Specify a category         |
| `-L, --local`    | Force local recipes first  |
| `-U, --user`     | Force user recipes first   |
| `-P, --public`   | Force public recipes first |

### Commands

| Command       | Description           |
|---------------|-----------------------|
| `shef update` | Update public recipes |

## Configuration

Shef looks for configuration in:

1. Project-specific config: `./.shef/config.yaml`
2. User config: `~/.shef/config.yaml`

Example config file:

```yaml
# ~/.shef/config.yaml
default_category: git
debug: false
```

## Example Recipes

### Git Workflow

```yaml
- name: "feature"
  description: "Create and push a git feature branch"
  category: "git"
  operations:
    - name: "Create Branch"
      id: "branch_op"
      command: "git checkout -b feature/{{ .feature_name }}"
      prompts:
        - name: "feature_name"
          type: "input"
          message: "Feature name:"
    
    - name: "Push Branch"
      command: "git push -u origin feature/{{ .feature_name }}"
      condition: "confirm_push == true"
      prompts:
        - name: "confirm_push"
          type: "confirm"
          message: "Push the branch to remote?"
          default: "true"
```

### Docker Container Management

```yaml
- name: "exec"
  description: "Execute commands in a Docker container"
  category: "docker"
  operations:
    - name: "List Containers"
      id: "list_containers"
      command: "docker ps --format '{{.Names}} ({{.Image}})'"
    
    - name: "Select Container"
      id: "select_container"
      command: "echo '{{ .container }}'"
      prompts:
        - name: "container"
          type: "select"
          message: "Select container:"
          source_operation: "list_containers"
          source_transform: "{{ .input | cut \" \" 0 }}"
    
    - name: "Execute Command"
      command: "docker exec -it {{ .container }} {{ .cmd }}"
      prompts:
        - name: "cmd"
          type: "input"
          message: "Command to run:"
          default: "bash"
```

## Creating Recipes

To create your own recipes:

1. Create your user directory: `mkdir -p ~/.shef/user` (if it does not already exist)
2. Create a new YAML file: `touch ~/.shef/user/my-recipes.yaml`
3. Add your recipes following the format above
4. Run `shef -l` to see your new recipes

## AI-Assisted Recipe Creation

Generate powerful Shef recipes quickly using AI tools like ChatGPT, Claude, or other large language models.

### Using AI to Create Recipes

1. Copy the prompt below
2. Paste it into your AI assistant of choice
3. Replace `[DESCRIBE YOUR WORKFLOW IN DETAIL]` with a detailed description of what you want your recipe to do
4. The AI will generate a complete Shef recipe based on your description
5. Test and iterate

### Example Usage

Here's an example of how to fill in your workflow description:

```
I need a Docker management workflow that helps me clean up unused containers and images to free up disk space.

The workflow should:
1. Show the current Docker disk usage
2. List all stopped containers and allow me to select which ones to remove
3. Confirm before removing the selected containers
4. List dangling images (unused) and allow me to select which ones to remove
5. Offer an option to perform a more aggressive cleanup (removing all unused images)
6. Show before/after disk usage comparison
7. Include error handling in case any operation fails

The recipe should be interactive and safe, requiring confirmation before any destructive operations.
```

### Tips for Better Results

- Be specific about what commands should be executed at each step
- Mention if you need interactive prompts, conditions, or transformations
- For complex workflows, break down your requirements into clear, logical steps
- Include any specific error handling or conditional branches you need
- Request comments in the generated recipe to explain complex sections

The AI-generated recipes provide an excellent starting point that you can further customize to fit your exact needs.

### The Prompt

```text
I need help creating a Shef recipe. Shef is a CLI tool that combines shell commands into reusable recipes defined in YAML.

[DESCRIBE YOUR WORKFLOW IN DETAIL]

A Shef recipe is defined in YAML with this structure:
recipes:
  - name: "short-name"
    description: "Human-readable description"
    category: "optional-category" 
    operations:
      - name: "Operation Name"
        id: "unique_id"
        command: "shell command"
        condition: "optional condition"
        on_success: "next_op_id"
        on_failure: "fallback_op_id"
        transform: "{{ .input | transformation }}"
        prompts:
          - name: "variable_name"
            type: "input|select|confirm"
            message: "Prompt message"
            default: "Default value"
            options: ["option1", "option2"]  # For select type
            source_operation: "operation_id"  # For dynamic options
            source_transform: "{{ .input | transform }}"  # For processing source options

RECIPE MECHANICS:
1. Operations execute in sequence unless redirected by on_success/on_failure
2. Each operation's output becomes input to the next operation
3. Variables from prompts are accessible as {{ .variable_name }}
4. Operation outputs are accessible as {{ .operation_id }}
5. You can combine variables and operation outputs in command templates

INTERACTIVE PROMPTS:
- input: Free text input (default: string)
- select: Choose from options (static or dynamic from previous operation)
- confirm: Yes/no boolean question

TRANSFORMATION EXAMPLES:
- Trim whitespace: {{ .input | trim }}
- Split by delimiter: {{ .input | split "," }}
- Join array: {{ .input | join "\n" }}
- Filter lines: {{ .input | filter "pattern" }} or {{ .input | grep "pattern" }}
- Extract field: {{ .input | cut ":" 1 }}
- Remove prefix/suffix: {{ .input | trimPrefix "foo" }} {{ .input | trimSuffix "bar" }}
- Check content: {{ if contains .input "pattern" }}yes{{ end }}
- Replace text: {{ .input | replace "old" "new" }}
- Math operations: {{ .input | atoi | add 5 | mul 2 | div 3 | sub 1 }}
- Execute command: {{ exec "date" }}

CONDITIONAL LOGIC:
- Variable equality: variable == "value" or variable != "value"
- Operation success/failure: operation_id.success or operation_id.failure
- Boolean operators: condition1 && condition2, condition1 || condition2, !condition
- Complex example: (check_files.success && has_tests == true) || skip_tests == true

ADVANCED FEATURES:
- Dynamic selection options from previous commands
- Conditional branching based on operation results
- Multi-step workflows with error handling
- Custom error messages and recovery steps
- Transforming outputs between operations

EXAMPLE RECIPE PATTERNS:
1. Get input → Process → Show result
2. List options → Select one → Take action
3. Check condition → Branch based on result → Handle each case
4. Execute command → Transform output → Use in next command
5. Try operation → Handle success/failure differently

Please create a complete Shef recipe that accomplishes my goal, with proper indentation and comments explaining complex parts.
```

## Troubleshooting

### Common Issues

**Recipe not found**

- Check if you're using the correct name and category
- Use `shef -l` to see available recipes
- Check your recipe file syntax

**Command fails with unexpected output**

- Remember all commands run through a shell, so shell syntax applies
- Escape special characters in command strings
- Use the `transform` field to format output
