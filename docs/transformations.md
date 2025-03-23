## Transformations

Transformations let you modify a command's output before it's passed to the next operation.

> [!IMPORTANT]
> In Go templates (which Shef uses), parameters are passed in the order they appear in the function call. When using
pipe
> syntax (`|`), the piped value becomes the **last parameter** of the function, not the first as in many template
engines.

### Basic Syntax

#### Direct function call (recommended for clarity)

```yaml
transform: "{{ function1 .output param1 }}"
```

#### Pipe syntax (parameters flipped - piped value becomes last parameter)

```yaml
transform: "{{ param1 | function1 .output }}"
```

### Available Transformation Functions

| Function         | Description                        | Parameters                 | Direct Example                        | Pipe Example                           | Input                  | Output                   |
|------------------|------------------------------------|----------------------------|---------------------------------------|----------------------------------------|------------------------|--------------------------|
| `trim`           | Remove whitespace                  | (string)                   | `{{ trim .output }}`                  | `{{ .output \| trim }}`                | `"  hello  "`          | `"hello"`                |
| `split`          | Split string by delimiter          | (string, delimiter)        | `{{ split .output "," }}`             | `{{ "," \| split .output }}`           | `"a,b,c"`              | `["a", "b", "c"]`        |
| `join`           | Join array with delimiter          | (array, delimiter)         | `{{ join .array "," }}`               | `{{ "," \| join .array }}`             | `["a", "b", "c"]`      | `"a,b,c"`                |
| `joinArray`      | Join any array type with delimiter | (array, delimiter)         | `{{ joinArray .array ":" }}`          | `{{ ":" \| joinArray .array }}`        | `[1, 2, 3]`            | `"1:2:3"`                |
| `trimPrefix`     | Remove prefix from string          | (string, prefix)           | `{{ trimPrefix .output "pre" }}`      | `{{ "pre" \| trimPrefix .output }}`    | `"prefix"`             | `"fix"`                  |
| `trimSuffix`     | Remove suffix from string          | (string, suffix)           | `{{ trimSuffix .output "fix" }}`      | `{{ "fix" \| trimSuffix .output }}`    | `"prefix"`             | `"pre"`                  |
| `contains`       | Check if string contains pattern   | (string, substring)        | `{{ contains .output "pat" }}`        | `{{ "pat" \| contains .output }}`      | `"pattern"`            | `true`                   |
| `replace`        | Replace text                       | (string, old, new)         | `{{ replace .output "old" "new" }}`   | `{{ "old" \| replace .output "new" }}` | `"oldtext"`            | `"newtext"`              |
| `filter`, `grep` | Keep lines containing a pattern    | (string, pattern)          | `{{ filter .output "err" }}`          | `{{ "err" \| filter .output }}`        | `"error\nok\nerror2"`  | `"error\nerror2"`        |
| `cut`            | Extract field from each line       | (string, delimiter, field) | `{{ cut .output ":" 1 }}`             | `{{ ":" \| cut .output 1 }}`           | `"name:value"`         | `"value"`                |
| `atoi`           | Convert string to int              | (string)                   | `{{ atoi .output }}`                  | `{{ .output \| atoi }}`                | `"42"`                 | `42`                     |
| `add`            | Add numbers                        | (num1, num2)               | `{{ add 5 3 }}` or `{{ add .num 5 }}` | `{{ 5 \| add 3 }}`                     | `5, 3`                 | `8`                      |
| `sub`            | Subtract numbers                   | (num1, num2)               | `{{ sub 10 4 }}`                      | `{{ 4 \| sub 10 }}`                    | `10, 4`                | `6`                      |
| `div`            | Divide numbers                     | (num1, num2)               | `{{ div 10 2 }}`                      | `{{ 2 \| div 10 }}`                    | `10, 2`                | `5`                      |
| `mul`            | Multiply numbers                   | (num1, num2)               | `{{ mul 6 7 }}`                       | `{{ 7 \| mul 6 }}`                     | `6, 7`                 | `42`                     |
| `exec`           | Execute command                    | (command)                  | `{{ exec "date" }}`                   | N/A                                    | `"date"`               | Output of `date` command |
| `color`          | Add color to text                  | (color, text)              | `{{ color "green" "Success!" }}`      | `{{ "Success!" \| color "green" }}`    | `"green", "Success!"`  | Green-colored "Success!" |
| `style`          | Add styling to text                | (style, text)              | `{{ style "bold" "Important!" }}`     | `{{ "Important!" \| style "bold" }}`   | `"bold", "Important!"` | Bold "Important!"        |
| `resetFormat`    | Reset colors and styles            | ()                         | `{{ resetFormat }}`                   | N/A                                    | N/A                    | ANSI reset code          |

#### Available Math Functions

| Function        | Description                       | Parameters        | Direct Example                      | Pipe Example                 | Input             | Output             |
|-----------------|-----------------------------------|-------------------|-------------------------------------|------------------------------|-------------------|--------------------|
| `mod`           | Modulo operation                  | (a, b)            | `{{ mod 10 3 }}`                    | `{{ 3 \| mod 10 }}`          | `10, 3`           | `1`                |
| `round`         | Round to nearest integer          | (value)           | `{{ round 3.7 }}`                   | `{{ 3.7 \| round }}`         | `3.7`             | `4`                |
| `rand`          | Generate random integer in range  | (min, max)        | `{{ rand 1 10 }}`                   | N/A                          | `1, 10`           | Random number 1-10 |
| `percent`       | Calculate percentage              | (part, total)     | `{{ percent 25 100 }}`              | `{{ 100 \| percent 25 }}`    | `25, 100`         | `25.0`             |
| `formatPercent` | Format percentage with decimals   | (value, decimals) | `{{ formatPercent 33.333 1 }}`      | N/A                          | `33.333, 1`       | `"33.3%"`          |
| `ceil`          | Round up to next integer          | (value)           | `{{ ceil 3.1 }}`                    | `{{ 3.1 \| ceil }}`          | `3.1`             | `4`                |
| `floor`         | Round down to integer             | (value)           | `{{ floor 3.9 }}`                   | `{{ 3.9 \| floor }}`         | `3.9`             | `3`                |
| `abs`           | Absolute value for floats         | (value)           | `{{ abs -3.5 }}`                    | `{{ -3.5 \| abs }}`          | `-3.5`            | `3.5`              |
| `max`           | Maximum of two integers           | (a, b)            | `{{ max 5 10 }}`                    | `{{ 10 \| max 5 }}`          | `5, 10`           | `10`               |
| `min`           | Minimum of two integers           | (a, b)            | `{{ min 5 10 }}`                    | `{{ 10 \| min 5 }}`          | `5, 10`           | `5`                |
| `pow`           | Power function                    | (base, exponent)  | `{{ pow 2 3 }}`                     | `{{ 3 \| pow 2 }}`           | `2, 3`            | `8.0`              |
| `sqrt`          | Square root                       | (value)           | `{{ sqrt 9 }}`                      | `{{ 9 \| sqrt }}`            | `9`               | `3.0`              |
| `log`           | Natural logarithm                 | (value)           | `{{ log 2.718 }}`                   | `{{ 2.718 \| log }}`         | `2.718`           | `1.0`              |
| `log10`         | Base-10 logarithm                 | (value)           | `{{ log10 100 }}`                   | `{{ 100 \| log10 }}`         | `100`             | `2.0`              |
| `formatNumber`  | Format numbers with pattern       | (format, args...) | `{{ formatNumber "%.2f" 3.14159 }}` | N/A                          | `"%.2f", 3.14159` | `"3.14"`           |
| `roundTo`       | Round to specified decimal places | (value, decimals) | `{{ roundTo 3.14159 2 }}`           | `{{ 2 \| roundTo 3.14159 }}` | `3.14159, 2`      | `3.14`             |

### Recommended Practices

1. **Use direct function calls for clarity** rather than pipe syntax, especially for functions that take multiple
   parameters
2. **For standard Go functions like `trimPrefix`**, remember they follow Go's parameter ordering:
   ```go
   // In standard Go
   strings.TrimPrefix(str, prefix)

   // In templates (direct call)
   {{ trimPrefix .output "[" }}  // Correct

   // In templates (pipe syntax) parameters are reversed
   {{ "[" | trimPrefix .output }}  // Correct, but confusing
   {{ .output | trimPrefix "[" }}  // Incorrect as it will try to remove .output from "["
   ```

### Function Groups

#### String Manipulation (Standard Go functions)

- `trim`, `trimPrefix`, `trimSuffix`, `split`, `join`, `contains`, `replace`

#### Array Operations

- `joinArray` (works with arrays of any type, unlike `join` which is for string arrays)

#### Text Processing

- `filter`, `grep`, `cut`

#### Numeric Operations

- `atoi`, `add`, `sub`, `div`, `mul`

#### Math Operations

- `mod`, `round`, `ceil`, `floor`, `abs`, `max`, `min`, `pow`, `sqrt`, `log`, `log10`, `percent`, `formatPercent`,
  `rand`, `roundTo`, `formatNumber`

#### Shell Integration

- `exec`

#### Formatting

- `color`, `style`, `resetFormat`

### Terminal Colors and Styles

You can make your recipe outputs more readable by adding colors and styles. These are automatically disabled when using
the `NO_COLOR` environment variable.

#### Available Colors

| Color Type        | Available Colors                                                                              |
|-------------------|-----------------------------------------------------------------------------------------------|
| Text Colors       | `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`                         |
| Background Colors | `bg-black`, `bg-red`, `bg-green`, `bg-yellow`, `bg-blue`, `bg-magenta`, `bg-cyan`, `bg-white` |

#### Available Styles

| Style       | Description     |
|-------------|-----------------|
| `bold`      | Bold text       |
| `dim`       | Dimmed text     |
| `italic`    | Italic text     |
| `underline` | Underlined text |

#### Using Colors and Styles

Colors and styles can be used in commands, transformations, and anywhere templates are rendered:

##### Basic Color Usage

```yaml
command: echo {{ color "green" "Success!" }}
```

##### Basic Style Usage

```yaml
command: echo {{ style "bold" "Important!" }}
```

##### Combine Color and Style

```yaml
command: echo {{ style "bold" (color "red" "Error!") }}
 ```

##### Transformations

```yaml
transform: |
  {{ if contains .output "error" }}
  {{ color "red" (style "bold" "✗ Operation failed") }}
  {{ else }}
  {{ color "green" (style "bold" "✓ Operation succeeded") }}
  {{ end }}
```

### Accessing Variables

You can access all context variables in transformations:

```yaml
transform: "{{ if eq .format `json` }}{{ .output }}{{ else }}{{ .output | cut ` ` 0 }}{{ end }}"
```

Variables available in templates:

- `.output`: The output to the current transformation (output from the command)
- `.input`: The input to the current command (input from previous operation or the string input from the user running
  the recipe)
- `.error`: If an operation fails, the error will be captured here. Resets every operation.
- `.{variable_name}`: Any variable, argument, or flag
- `.{prompt_name}`: Any variable from defined prompts
- `.{operation_id}`: The output of a specific operation by ID
- `.operationOutputs`: Map of all operation outputs by ID
- `.operationResults`: Map of operation success/failure results by ID

> [!NOTE]
> Undefined variables will always evaluate to the string value of `"false"`
