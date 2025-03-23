## Arguments and Flags

Shef allows you to pass arguments and flags directly to your recipes from the command line.

### Basic Argument Syntax

```
shef [category] [recipe-name] [input-text] [flags...]
```

or without category:

```
shef [recipe-name] [input-text] [flags...]
```

### Available Flag Types

| Flag Type                  | Example               | Variable         | Value           |
|----------------------------|-----------------------|------------------|-----------------|
| Input Text                 | `"Hello World"`       | `.input`         | `"Hello World"` |
| Short Flag (boolean only)  | `-f`                  | `.f`             | `true`          |
| Long Flag                  | `--name=John`         | `.name`          | `"John"`        |
| Long Flag with dash        | `--user-agent=Chrome` | `.user_agent`    | `"Chrome"`      |
| Multi-short (boolean only) | `-abc`                | `.a`, `.b`, `.c` | `true`          |

### Usage Examples

```bash
# Pass text input to a recipe
shef demo arguments "Hello World"

# Pass a boolean flag
shef demo arguments -f

# Pass a value flag
shef demo arguments --name=John

# Combine multiple types
shef demo arguments "My input text" -vf -a --count=5 --verbose
```

### Accessing Arguments in Recipes

You can access these values in your recipe operations:

```yaml
- name: "Display Arguments"
  command: |
    echo "Input: {{ .input }}"
    echo "Flag f: {{ .f }}"
    echo "Name: {{ .name }}"

- name: "Conditional based on flag"
  command: echo "Flag was set!"
  condition: .f == true
```
