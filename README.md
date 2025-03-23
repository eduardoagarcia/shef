# Shef

Shef, a wordplay on *"shell"* and *"chef"*, is a powerful CLI tool for cooking up advanced shell recipes.

Essentially, imagine that [Make](https://www.gnu.org/software/make), [GitHub Actions](https://github.com/features/actions),
and [CyberChef](https://gchq.github.io/CyberChef) had a weird little `<moira-rose>bea-by</>`.

Shef allows you to pipe shell commands together, add interactive user prompts, loop using complex control structures,
easily run and manage background tasks, and build reusable workflows with advanced logic and conditions.

## Quick Start Example

The following example showcases a simple Shef recipe, giving you a quick glance at the syntax and functionality.

![Quick Start Conditional Example](images/conditional.gif)

```yaml
recipes:
  - name: "conditional"
    description: "A simple demo of conditional operations using direct prompt values"
    category: "demo"
    operations:
      - name: "Choose Fruit"
        id: "choose"
        command: 'echo "You selected: {{ .fruit }}"'
        prompts:
          - name: "Fruit Select"
            id: "fruit"
            type: "select"
            message: "Choose a fruit:"
            options:
              - "Apples"
              - "Oranges"

      - name: "Apple Operation"
        id: "apple"
        command: echo "This is the apple operation! 🍎"
        condition: .fruit == "Apples"

      - name: "Orange Operation"
        id: "orange"
        command: echo "This is the orange operation! 🍊"
        condition: .fruit == "Oranges"
```

> [!TIP]
> Want to see more before diving deeper? [Check out the demo recipes.](https://github.com/eduardoagarcia/shef/tree/main/recipes/demo)

## Documentation

- [Installation](docs/installation.md)
- [A Note on Bash and YAML](docs/bash-and-yaml.md)
- [Command Reference](docs/command-reference.md)
- [Recipe Sources](docs/recipe-sources.md)
- [Recipe Structure](docs/recipe-structure.md)
- [Operation Execution Order](docs/operation-execution-order.md)
- [Interactive Prompts](docs/interactive-user-prompts.md)
- [Control Flow Structures](docs/control-flow-structures.md)
- [Transformations](docs/transformations.md)
- [Conditional Execution](docs/conditional-execution.md)
- [Data Flow Between Operations](docs/data-flow-between-operations.md)
- [Branching Workflows](docs/branching-workflows.md)
- [Arguments and Flags](docs/arguments-and-flags.md)
- [Recipe Help Documentation](docs/recipe-help-documentation.md)
- [Creating Recipes](docs/creating-recipes.md)
- [Example Recipes](docs/example-recipes.md)
- [AI-Assisted Recipe Creation](docs/ai-assisted-recipe-creation.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Contributing to Shef](docs/contributing-to-shef.md)
- [Additional Reference](docs/additional-reference.md)

## Shef's Primary Features

- **Command Piping**: Chain multiple commands together, passing output from one command to the next
- **Transformations**: Transform command output with powerful templating
- **Interactive Prompts**: Add user input, selections, confirmations, and more
- **Conditional Logic**: Branching based on command results
- **Control Flow**: Create dynamic workflows with loops and control structures
- **Background Task Management**: Easily monitor and control background tasks
- **Progress Mode**: Inline updates for clean status updates and progress indicators
- **Multiple Sources and Contexts**: Use local, user, or public recipes
- **Public Recipes**: Common, useful recipes anyone can use. Browse public [recipes](https://github.com/eduardoagarcia/shef/tree/main/recipes).

## Quick Start

Once Shef is installed, you are ready to begin using it.

```bash
# Sync all public recipes locally
shef sync

# Run the Hello World recipe
shef demo hello-world

# View help information about a recipe
shef demo hello-world --help

# List available recipes (demo recipes are excluded by default)
shef ls

# List all recipes within a specific category
shef ls demo
```

## Dive In

Ready to learn more? Start with [learning how to build your own recipes](docs/recipe-structure.md), then check out
[recipe arguments and flags](docs/arguments-and-flags.md) and [background task management.](docs/recipe-structure.md#background-task-management)
