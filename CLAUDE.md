# Shef - Shell Recipe Tool

## Overview
Shef is a CLI tool for creating and running shell-based workflows defined in YAML files ("recipes"). The name is a wordplay on "shell" and "chef" - cooking up shell recipes.

## Key Concepts

### Recipes
- YAML files defining shell operations and workflows
- Include commands, prompts, control flow, and transformations
- Can be organized in categories (demo, docker, git, etc.)
- Located in local, user, or public directories

### Operations
- Basic building blocks of recipes
- Each operation has a name, command, and optional properties
- Commands are executed in sequence with piping and data flow
- Can have conditions, transformations, and error handling

### Features
- Interactive prompts for user input (text, select, confirm)
- Control flow (loops, conditions, branching)
- Command output transformation via templates
- Background tasks and monitoring
- Progress indicators
- Reusable components
- Template function support

## Code Structure

### Main Components
- `recipe.go`: Core recipe execution logic
- `types.go`: Data structures for recipes, operations, etc.
- `templates.go`: Template system for variable substitution
- `sync.go`: Public recipe repository synchronization
- `app.go`: CLI application setup and command handling

### Key Data Structures
- `Recipe`: Represents a full recipe with metadata and operations
- `Operation`: Single executable step with conditions, prompts, etc.
- `ExecutionContext`: Runtime state during recipe execution
- `Component`: Reusable set of operations

## Commands
- `shef [category] [recipe-name]`: Run a recipe
- `shef ls`: List available recipes
- `shef sync`: Download and update public recipes
- `shef which`: Show recipe file location

## Recipe Syntax
```yaml
recipes:
  - name: "example"
    description: "Example recipe"
    category: "demo"
    operations:
      - name: "Operation 1"
        command: "echo Hello"
        prompts:
          - name: "User Input"
            id: "name"
            type: "input"
            message: "Enter your name:"
        condition: ".name != ''"
```

## Operation Properties
- `name`: Display name
- `id`: Identifier for referencing outputs
- `command`: The shell command to execute
- `condition`: When to run the operation
- `prompts`: User inputs needed
- `transform`: Process command output
- `control_flow`: Loops and iterations
- `on_success/on_failure`: Error handling

## Template Functions
- String: `split`, `join`, `trim`, `replace`, `filter`, etc.
- Math: `add`, `sub`, `mul`, `div`, `mod`, `round`, etc.
- Formatting: `color`, `style`, `formatNumber`, etc.
- Background tasks: `bgTaskStatus`, `bgTaskComplete`, etc.

## File Locations
- Local: Current directory (`./recipes/`)
- User: User-specific recipes (`~/.shef/user/`)
- Public: Shared recipes (`~/.shef/public/`)

## Current Branch
Currently on branch `refactor-update-recipes`
Recent commits:
- Add filtering to docker logs
- Update docker recipes