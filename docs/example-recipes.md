## Example Recipes

For even more examples, [check out the demo recipes.](https://github.com/eduardoagarcia/shef/tree/main/recipes/demo)

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
          - name: "Name Input"
            id: "name"
            type: "input"
            message: "What is your name?"
            default: "World"
```

### Success and Error Handling

```yaml
recipes:
  - name: "handlers"
    description: "A simple demo of error and success handlers"
    category: "demo"
    operations:
      - name: "Help"
        command: echo {{ color "magenta" "Please provide a valid or invalid directory argument to test" }}
        condition: .input == "false"

      - name: "Test if directory exists"
        command: cd {{ .input }}
        on_failure: "handle_error"
        on_success: "handle_success"
        condition: .input != "false"

      - name: "Handle error"
        id: "handle_error"
        command: echo {{ color "red" (printf "Directory %s does NOT exist!" .input) }}

      - name: "Handle success"
        id: "handle_success"
        command: echo {{ color "green" (printf "Directory %s exists!" .input) }}

```

### Transform Pipeline

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

      - name: "Filter 'a' Items"
        id: "filter_a"
        command: cat
        transform: '{{ filter .generate "a" }}'
        silent: true

      - name: "Display 'a' Results"
        id: "display_a"
        command: echo "Items containing 'a':\n{{ color "green" .filter_a }}"
```
