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
# Run a recipe
shef git version

# List available recipes
shef -l

# List recipes in a category
shef -l git

# Update public recipes
shef update
```

## Recipe Sources

Shef looks for recipes in multiple locations:

1. **Local Recipes**: `./.shef/*.yaml` in the current directory
2. **User Recipes**: `~/.shef/*.yaml` in your home directory
3. **Public Recipes**: Downloaded from the public repository

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
public_repo: https://github.com/yourusername/shef-recipes
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

1. Create a directory: `mkdir -p ~/.shef`
2. Create a YAML file: `touch ~/.shef/mycategory.yaml`
3. Add your recipes following the format above
4. Run `shef -l` to see your new recipes

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

## License

MIT License

Copyright (c) 2025 Eduardo A. Garcia

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
