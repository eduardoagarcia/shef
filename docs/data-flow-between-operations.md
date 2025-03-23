## Data Flow Between Operations

Each operation's output is automatically piped to the next operation's input. You can also access any operation's output
by its ID:

```yaml
- name: "Get Hostname"
  id: "hostname_op"
  command: hostname

- name: "Show Info"
  command: echo "Running on {{ .hostname_op }}"
```
