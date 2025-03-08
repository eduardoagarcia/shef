# Shef

Shef, a wordplay on *"shell"* and *"chef"*, is a powerful CLI tool for cooking up shell recipes without the mess.

Think of it as [CyberChef](https://gchq.github.io/CyberChef) for your terminal: pipe commands together, add interactive prompts, and build reusable workflows without complex scripting.

## Table of Contents

- [Features](#features)
- [Why Shef vs. Bash Scripts?](#why-shef-vs-bash-scripts)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Recipe Sources](#recipe-sources)
- [Recipe Structure](#recipe-structure)
- [Interactive Prompts](#interactive-prompts)
- [Transformations](#transformations)
- [Conditional Execution](#conditional-execution)
- [Branching Workflows](#branching-workflows)
- [Data Flow Between Operations](#data-flow-between-operations)
- [Shef Command Reference](#shef-command-reference)
- [Example Recipes](#example-recipes)
- [Creating Recipes](#creating-recipes)
- [AI-Assisted Recipe Creation](#ai-assisted-recipe-creation)
- [Troubleshooting](#troubleshooting)
- [Contributing to Shef](#contributing-to-shef)

## Features

- **Command Piping**: Chain multiple commands together, passing output from one command to the next
- **Transformations**: Transform command output and input with powerful templating
- **Interactive Prompts**: Add user input, selections, confirmations, and more
- **Conditional Logic**: Use if/else branching based on command results
- **Multiple Sources**: Use local, user, or public recipes
- **Organized Recipes**: Categorize and share your recipes with others

## Why Shef vs. Bash Scripts?

While many of Shef's capabilities could be implemented with bash scripts, Shef provides a structured approach that eliminates the complexity of shell scripting. It offers interactive prompts, conditional logic, and command piping through a simple YAML interface—no need to wrestle with bash syntax, error handling, or input validation.

Shef gives you the best of both worlds: the power of shell commands without the scripting headaches. Think of it as a Makefile that works everywhere—in projects, system-wide, or via shared recipes—but with better interactivity and cleaner syntax. Complex workflows become accessible, regardless of your scripting expertise.

## Installation

### Prerequisites

Before installing Shef, ensure you have Go installed and configured on your system:

1. **Install Go**: If you don't have Go installed, download and install it from [golang.org](https://golang.org/dl/) or
   use your system's package manager:

   ```bash
   # macOS (using Homebrew)
   brew install go

   # Ubuntu/Debian
   sudo apt update
   sudo apt install golang-go

   # Fedora
   sudo dnf install golang
   ```

2. **Configure Go Environment**: Ensure your Go environment is properly set up:

   ```bash
   # Add these to your shell configuration (.bashrc, .zshrc, etc.)
   export GOPATH=$HOME/go
   export PATH=$PATH:$GOPATH/bin
   ```

3. **Verify Installation**: Confirm Go is correctly installed:

   ```bash
   go version
   ```

### Quick Installation

The simplest way to install Shef is with Make:

```bash
# Clone the repository
git clone git@github.com:eduardoagarcia/shef.git
cd shef

# Install (requires sudo for system-wide installation)
make install

# Or install to your home directory (no sudo required)
make install-local
```

### Manual Installation Options

#### Install with Go

Once you have Go installed, you can install Shef directly:

```bash
go install github.com/eduardoagarcia/shef@latest
```

This will install the `shef` binary to your `$GOPATH/bin` directory.

#### Build from Source

```bash
# Clone the repository
git clone git@github.com:eduardoagarcia/shef.git

# Build the application
cd shef
go build -o shef

# Move to a directory in your PATH
sudo mv shef /usr/local/bin/
```

### Adding to PATH

If the installation directory is not in your PATH, you'll need to add it:

```bash
# Add this to your .bashrc, .bash_profile, or .zshrc
export PATH="$PATH:$GOPATH/bin"  # For go install
# OR
export PATH="$PATH:$HOME/bin"    # For make install-local
```

Then reload your shell configuration: `source ~/.bashrc` (or `~/.zshrc`, `~/.bash_profile` depending on your shell)

## Quick Start

Once Shef is installed, you are ready to begin using it.

```bash
# Download (or update) all public recipes locally
shef update

# Run the Hello World recipe
shef demo hello-world

# List available recipes
shef -l

# List all recipes within a category
shef -l demo
```

## Recipe Sources

Shef looks for recipes in multiple locations:

1. **Local Recipes**: `./.shef/*.yaml` in the current directory
2. **User Recipes**: `~/.shef/user/*.yaml` in your home directory
3. **Public Recipes**: `~/.shef/public/*.yaml` in your home directory

If you have recipies with the same name and category in different locations, you can prioritize a specific source:

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
        command: "cat"
        transform: "{{ filter .input 'Hello' }}"
```

### Key Recipe Components

- **name**: Unique identifier for the recipe
- **description**: Human-readable description
- **category**: Used for organization and filtering
- **author**: Optional author attribution
- **operations**: List of operations to execute in sequence

### Operations

Operations are the building blocks of recipes:

```yaml
- name: "Operation Name"            # Operation name
  id: "unique_id"                   # Identifier for referencing output
  command: "echo 'Hello'"           # Shell command to execute
  execution_mode: "standard"        # [Optional] How the command runs (standard, interactive, or stream)
  silent: false                     # [Optional] Whether to suppress output to stdout
  condition: "var == true"          # [Optional] Condition for execution
  on_success: "success_op"          # [Optional] Operation to run on success
  on_failure: "failure_op"          # [Optional] Operation to run on failure
  transform: "{{ .input | trim }}"  # [Optional] Transform output
  prompts:                          # [Optional] Interactive prompts
    - name: "var_name"
      type: "input"
      message: "Enter value:"
```

#### Execution Modes

- **standard**: Default mode (used when no execution_mode is specified). Output is captured and can be used by subsequent operations.
- **interactive**: The command has direct access to the terminal's stdin, stdout, and stderr. Useful for commands that require direct terminal interaction, but output cannot be captured for use in subsequent operations.
- **stream**: Similar to interactive mode but optimized for long-running processes that produce continuous output. The command's output streams to the terminal in real-time, but output cannot be captured for use in subsequent operations.

## Interactive Prompts

Shef supports the following types of prompts:

### Basic Input Types

```yaml
# Text Input
- name: "username"
  type: "input"
  message: "Enter your username:"
  default: "admin"
  help_text: "This will be used for authentication"

# Selection
- name: "environment"
  type: "select"
  message: "Select environment:"
  options:
    - "dev"
    - "staging"
    - "production"
  default: "dev"
  help_text: "Choose the deployment environment"

# Confirmation (yes/no)
- name: "confirm_deploy"
  type: "confirm"
  message: "Deploy to production?"
  default: "false"
  help_text: "This will start the deployment process"

# Password (input is masked)
- name: "password"
  type: "password"
  message: "Enter your password:"
  help_text: "Your input will be hidden"
```

### Advanced Input Types

```yaml
# Multi-select
- name: "features"
  type: "multiselect"
  message: "Select features to enable:"
  options:
    - "logging"
    - "metrics"
    - "debugging"
  default: "logging,metrics"
  help_text: "Use space to toggle, enter to confirm"

# Numeric Input
- name: "count"
  type: "number"
  message: "Enter number of instances:"
  default: "3"
  min_value: 1
  max_value: 10
  help_text: "Value must be between 1 and 10"

# File Path Input
- name: "config_file"
  type: "path"
  message: "Select configuration file:"
  default: "./config.json"
  file_extensions:
    - "json"
    - "yaml"
    - "yml"
  required: true
  help_text: "File must exist and have the right extension"

# Text Editor
- name: "description"
  type: "editor"
  message: "Enter a detailed description:"
  default: "# Project Description\n\nEnter details here..."
  editor_cmd: "vim"  # Uses $EDITOR env var if not specified
  help_text: "This will open your text editor"

# Autocomplete Selection
- name: "service"
  type: "autocomplete"
  message: "Select a service:"
  options:
    - "authentication"
    - "database"
    - "storage"
    - "analytics"
  help_text: "Type to filter options"
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

| Function     | Description                        | Example                                          |
|--------------|------------------------------------|--------------------------------------------------|
| `trim`       | Remove whitespace                  | `{{ .input \| trim }}`                           |
| `split`      | Split string by delimiter          | `{{ .input \| split "," }}`                      |
| `join`       | Join array with delimiter          | `{{ .input \| join "\n" }}`                      |
| `joinArray`  | Join any array type with delimiter | `{{ .items \| joinArray "," }}`                  |
| `filter`     | Keep lines containing a pattern    | `{{ .input \| filter "pattern" }}`               |
| `grep`       | Alias for filter                   | `{{ .input \| grep "pattern" }}`                 |
| `cut`        | Extract field from each line       | `{{ .input \| cut ":" 1 }}`                      |
| `trimPrefix` | Remove prefix                      | `{{ .input \| trimPrefix "foo" }}`               |
| `trimSuffix` | Remove suffix                      | `{{ .input \| trimSuffix "bar" }}`               |
| `contains`   | Check if string contains pattern   | `{{ if contains .input "pattern" }}yes{{ end }}` |
| `replace`    | Replace text                       | `{{ .input \| replace "old" "new" }}`            |
| `atoi`       | Convert string to int              | `{{ .input \| atoi }}`                           |
| `add`        | Add numbers                        | `{{ .input \| atoi \| add 5 }}`                  |
| `sub`        | Subtract numbers                   | `{{ .input \| atoi \| sub 3 }}`                  |
| `div`        | Divide numbers                     | `{{ .input \| atoi \| div 2 }}`                  |
| `mul`        | Multiply numbers                   | `{{ .input \| atoi \| mul 4 }}`                  |
| `exec`       | Execute command                    | `{{ exec "date" }}`                              |

### Accessing Variables

You can access all context variables in transformations:

```yaml
transform: "{{ if eq .format \"json\" }}{{ .input }}{{ else }}{{ .input | cut \" \" 0 }}{{ end }}"
```

Variables available in templates:

- `.input`: The input to the current transformation (output from previous operation)
- `.{prompt_name}`: Any variable from defined prompts
- `.{operation_id}`: The output of a specific operation by ID
- `.operationOutputs`: Map of all operation outputs by ID
- `.operationResults`: Map of operation success/failure results by ID

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

### Variable Comparison

```yaml
condition: "environment == 'production'"  # Equality check
condition: "count != 0"                   # Inequality check
```

### Numeric Comparison

```yaml
condition: "count > 5"
condition: "memory <= 512"
condition: "errors >= 10"
condition: "progress < 100"
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
  command: "echo 'Running on {{ .hostname_op }}'"
```

## Shef Command Reference

### Basic Shef Command Structure

```
shef [category] [recipe-name]
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

### Utility Commands

| Command       | Description           |
|---------------|-----------------------|
| `shef update` | Update public recipes |

**Note**: make sure your Shef git repo is up to date (`git pull`) before running `shef update`

## Example Recipes

### Hello World

```yaml
recipes:
  - name: "hello-world"
    description: "A simple hello world recipe"
    category: "demo"
    operations:
      - name: "Greet User"
        command: |
          echo "Hello, {{ .name }}!"
          echo "Current time: $(date)"
          echo "Welcome to Shef, the shell recipe tool."
        prompts:
          - name: "name"
            type: "input"
            message: "What is your name?"
            default: "World"
```

### Conditional Operations

```yaml
recipes:
  - name: "conditional"
    description: "A simple demo of conditional operations"
    category: "demo"
    operations:
      - name: "Choose Fruit"
        id: "choose"
        command: "echo 'You selected: {{ .fruit }}'"
        prompts:
          - name: "fruit"
            type: "select"
            message: "Choose a fruit:"
            options:
              - "Apples"
              - "Oranges"

      - name: "Apple Operation"
        id: "apple"
        command: "echo 'This is the apple operation! 🍎'"
        condition: ".fruit == 'Apples'"

      - name: "Orange Operation"
        id: "orange"
        command: "echo 'This is the orange operation! 🍊'"
        condition: ".fruit == 'Oranges'"
```

### Transformation Pipeline

```yaml
recipes:
  - name: "transform"
    description: "A simple demo of data transformation and pipeline flow"
    category: "demo"
    operations:
      - name: "Generate a Simple List"
        id: "generate"
        command: |
          echo "apple
          banana
          cherry
          dragonfruit
          eggplant"

      - name: "Filter Items"
        id: "filter"
        command: "cat"
        transform: "{{ filter .input \"a\" }}"
        silent: true

      - name: "Display Results"
        id: "display"
        command: "echo 'Items containing \"a\":\n{{ .filter }}'"
```

## Creating Recipes

To create your own recipes:

1. Create your user directory: `mkdir -p ~/.shef/user` (if it does not already exist)
2. Create a new YAML file: `touch ~/.shef/user/my-recipes.yaml`
3. Build and develop your recipes following the instructions above
4. Run `shef -l` to see your new recipes

## AI-Assisted Recipe Creation

You can generate powerful Shef recipes quickly using AI tools like ChatGPT, Claude, or other large language models.

### Using AI to Create Recipes

1. Copy the prompt below
2. Paste it into your AI assistant of choice
3. Replace `[DESCRIBE YOUR WORKFLOW IN DETAIL]` with a detailed description of your recipe's workflow
4. The AI will generate a complete Shef recipe based on your description
5. Test and iterate until the recipe works as expected

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

- Use the latest AI models with advanced reasoning capabilities
- Be specific about what commands should be executed at each step
- Mention if you need interactive prompts, conditions, or transformations
- For complex workflows, break down your requirements into clear, logical steps
- Include any specific error handling or conditional branches you need
- Request comments in the generated recipe to explain complex sections
- Ask the AI to analyze and iterate on its recipe solution, considering edge cases and improvements
- If the first recipe doesn't fully meet your needs, refine your requirements and ask for adjustments

Remember, the AI-generated recipes can provide an excellent starting point that you can further customize to fit your exact needs.

### The Prompt

```text
I need help creating a Shef recipe. Shef is a CLI tool that combines shell commands into reusable recipes defined in YAML.

[DESCRIBE YOUR WORKFLOW IN DETAIL]

A Shef recipe is defined in YAML with this structure:

recipes:
  - name: "short-name"
    description: "Human-readable description"
    category: "category"
    author: "optional author"
    operations:
      - name: "Operation Name"
        id: "unique_id"
        command: "shell command"
        execution_mode: "standard|interactive|stream"  # How the command runs
        silent: true|false  # Whether to suppress the command's output
        condition: "optional condition"
        on_success: "next_op_id"
        on_failure: "fallback_op_id"
        transform: "{{ .input | transformation }}"
        prompts:
          - name: "variable_name"
            type: "input|select|confirm|password|multiselect|number|editor|path|autocomplete"
            message: "Prompt message"
            default: "Default value"
            help_text: "Additional help information"
            required: true|false  # Whether input is required
            options: ["option1", "option2"]  # For select/multiselect/autocomplete types
            source_operation: "operation_id"  # For dynamic options
            source_transform: "{{ .input | transform }}"  # For processing source options
            min_value: 0  # For number type
            max_value: 100  # For number type
            file_extensions: ["txt", "json"]  # For path type
            multiple_limit: 3  # For multiselect type
            editor_cmd: "vim"  # For editor type

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
- password: Masked text input
- multiselect: Choose multiple options
- number: Numeric input with range validation
- editor: Multi-line text input in an editor
- path: File path with validation
- autocomplete: Selection with filtering

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
- Numeric comparison: value > 5, count <= 10
- Complex example: (check_files.success && has_tests == true) || skip_tests == true

ADVANCED FEATURES:
- Dynamic selection options from previous commands
- Conditional branching based on operation results
- Multi-step workflows with error handling
- Custom error messages and recovery steps
- Transforming outputs between operations
- Execution modes (standard or interactive)
- Silent operations that suppress output

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

**Prompt validation errors**

- Ensure minimum/maximum values are within range
- Check that file paths exist and have correct extensions
- Verify select options contain the default value

## Contributing to Shef

Shef thrives on community contributions, whether you're improving the core Go codebase or sharing useful recipes. Here's
how you can contribute:

### Contributing Code

#### Development Setup

1. **Fork the Repository**
   ```bash
   # Fork via GitHub UI, then clone your fork
   git clone git@github.com:yourusername/shef.git
   cd shef
   ```

2. **Set Up Development Environment**
   ```bash
   # Install development dependencies
   go mod download

   # Build the development version
   go build -o shef
   ```

3. **Create a New Branch**
   ```bash
   git checkout -b my-awesome-feature
   ```

#### Development Guidelines

- **Code Style**: Follow standard Go conventions and the existing style in the codebase
- **Documentation**: Update documentation for any new features or changes
- **Commit Messages**: Write clear, descriptive commit messages explaining your changes

#### Submitting Your Changes

1. **Push to Your Fork**
   ```bash
   git push origin my-awesome-feature
   ```

2. **Create a Pull Request**: Visit your fork on GitHub and create a pull request against the main repository

3. **PR Description**: Include a clear description of what your changes do and why they should be included

4. **Code Review**: Respond to any feedback during the review process

### Contributing Recipes

Sharing your recipes helps grow the Shef ecosystem and benefits the entire community.

#### Creating Public Recipes

1. **Develop and Test Your Recipe Locally**
   ```bash
   # Create your recipe in the user directory first
   mkdir -p ~/.shef/user
   vim ~/.shef/user/my-recipe.yaml
   
   # Test thoroughly
   shef -U my-category my-recipe-name
   ```

2. **Recipe Quality Guidelines**
   - Include clear descriptions for the recipe and each operation
   - Add helpful prompts with descriptive messages and defaults
   - Handle errors gracefully
   - Follow YAML best practices
   - Comment complex transformations or conditionals

3. **Submitting Your Recipe**

   **Option 1: Via Pull Request**
   - Fork the Shef repository
   - Add your recipe to the `recipes/public/` directory
   - Create a pull request with your recipe

   **Option 2: Via Issue**
   - Create a new issue on the Shef repository
   - Attach your recipe file or paste its contents
   - Describe what your recipe does and why it's useful

#### Recipe Documentation

When submitting a recipe, include a section in your PR or issue that explains:

1. **Purpose**: What problem does your recipe solve?
2. **Usage**: How to use the recipe, including example commands
3. **Requirements**: Any special requirements or dependencies
4. **Examples**: Sample outputs or use cases

### Community Guidelines

- Be respectful of others' contributions
- Help review pull requests and test others' recipes
- Report bugs and suggest improvements
- Share your Shef success stories and use cases

### Getting Help

If you need help with your contribution, you can:

- Open an issue on GitHub
- Ask questions in the discussions section
- Contact the maintainers directly

Thank you for contributing to Shef and helping to make shell workflows easier for everyone!
