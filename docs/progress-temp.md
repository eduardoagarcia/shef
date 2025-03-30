## Progress Bars

Shef provides customizable progress bars for long-running operations within loops, offering visual feedback about
iteration progress, timing, and completion status.

### Progress Bar Basics

Progress bars can be added to any `for` or `foreach` loop in your recipes:

```yaml
- name: "Process With Progress Bar"
  control_flow:
    type: "for"
    count: 100
    variable: "i"
    progress_bar: true  # Enable the progress bar
  operations:
    - name: "Process Item"
      command: sleep 0.1
```

The progress bar provides real-time visual feedback with:

- Current progress percentage
- Item count (e.g., "5/100")
- Elapsed time
- Clean, customizable appearance

### Customization Options

Progress bars can be extensively customized using the `progress_bar_options` field:

```yaml
progress_bar_options:
  description: "Processing Files"          # Text shown at the start of the bar
  width: 50                                # Width in characters (default: terminal width)
  show_count: true                         # Show "5/100" counts (default: true)
  show_percentage: true                    # Show percentage (default: true)
  show_elapsed_time: true                  # Show elapsed time (default: true)
  show_iteration_speed: true               # Show iterations/second (default: false)
  refresh_rate: 0.1                        # Update rate in seconds (default: every iteration)
  message_template: "Custom message here"  # Dynamic message template (optional)
  theme:                                   # Visual appearance customization
    saucer: "[green]=[reset]"              # Bar fill character
    saucer_head: "[green]>[reset]"         # Leading character
    saucer_padding: " "                    # Empty bar character
    bar_start: "["                         # Left bracket
    bar_end: "]"                           # Right bracket
```

### Message Templates

Progress bars can display dynamic messages using Go templates with access to all variables in the execution context:

```yaml
message_template: "Processing {{ .item }} ({{ .progress }}) - {{ .duration_fmt }} elapsed"
```

#### Available Template Variables

| Variable        | Description                            | Example Value |
|-----------------|----------------------------------------|---------------|
| `.iteration`    | Current iteration number (1-based)     | `5`           |
| `.total`        | Total number of iterations             | `100`         |
| `.progress`     | Formatted progress fraction            | `"5/100"`     |
| `.duration_ms`  | Duration in milliseconds               | `"12345"`     |
| `.duration_s`   | Duration in seconds                    | `"12"`        |
| `.duration_fmt` | Formatted duration (MM:SS or HH:MM:SS) | `"00:12"`     |
| `.item`         | Current item (foreach loops)           | `"file.txt"`  |

#### Template Function Examples

You can use all of Shef's template functions in your message templates:

```yaml
# Calculate and display processing speed
message_template: "Speed: {{ formatNumber \"%.2f\" (div .iteration .duration_s) }} items/sec"

# Format with color
message_template: "Processing {{ color \"green\" .item }}"

# Conditional formatting
message_template: "{{ if gt .iteration 50 }}Over halfway!{{ else }}Starting...{{ end }}"

# Time remaining estimate
message_template: "Est. remaining: {{ formatNumber \"%.1f\" (mul (sub .total .iteration) (div .duration_s .iteration)) }}s"
```

### Theme Customization

Progress bars can be styled with ANSI colors and custom characters:

```yaml
theme:
  # Green equals signs with arrow head
  saucer: "[green]=[reset]"
  saucer_head: "[green]>[reset]"
  saucer_padding: " "
  bar_start: "["
  bar_end: "]"
```

#### Color Options

All text elements support ANSI colors:

| Color Values | Description            |
|--------------|------------------------|
| `[black]`    | Black text             |
| `[red]`      | Red text               |
| `[green]`    | Green text             |
| `[yellow]`   | Yellow text            |
| `[blue]`     | Blue text              |
| `[magenta]`  | Magenta text           |
| `[cyan]`     | Cyan text              |
| `[white]`    | White text             |
| `[reset]`    | Reset to default color |

#### Theme Examples

```yaml
# Blue blocks style
theme:
  saucer: "[blue]▓[reset]"
  saucer_head: "[blue]▓[reset]"
  saucer_padding: "░"
  bar_start: "["
  bar_end: "]"

# Red slash style
theme:
  saucer: "[red]/[reset]"
  saucer_head: "[red]/[reset]"
  saucer_padding: " "
  bar_start: "//"
  bar_end: "//"

# Cyan dot style
theme:
  saucer: "[cyan]·[reset]"
  saucer_head: "[cyan]>[reset]"
  saucer_padding: " "
  bar_start: "{"
  bar_end: "}"
```

### Example Recipes

#### Basic Progress Bar

```yaml
- name: "Simple Processing"
  control_flow:
    type: "for"
    count: 100
    variable: "i"
    progress_bar: true
  operations:
    - name: "Process Item"
      command: sleep 0.1
```

#### Custom Progress Bar with Dynamic Message

```yaml
- name: "File Processing with Custom Bar"
  control_flow:
    type: "foreach"
    collection: "{{ .files }}"
    as: "file"
    progress_bar: true
    progress_bar_options:
      description: "Processing Files"
      width: 40
      message_template: "File: {{ .file }} - {{ formatPercent (percent .iteration .total) 0 }}% complete"
      theme:
        saucer: "[green]█[reset]"
        saucer_head: "[green]█[reset]"
        saucer_padding: "░"
        bar_start: "["
        bar_end: "]"
  operations:
    - name: "Process file"
      command: "process_file.sh {{ .file }}"
```

#### Progress Bar with Speed and Time Estimates

```yaml
- name: "Batch Job with Metrics"
  control_flow:
    type: "for"
    count: 1000
    variable: "i"
    progress_bar: true
    progress_bar_options:
      description: "Batch Processing"
      width: 50
      show_iteration_speed: true
      message_template: "Speed: {{ formatNumber \"%.1f\" (div .iteration .duration_s) }}/s | Est. remaining: {{ formatNumber \"%.1f\" (mul (sub .total .iteration) (div .duration_s .iteration)) }}s"
  operations:
    - name: "Process batch item"
      command: "process_item.sh {{ .i }}"
```
