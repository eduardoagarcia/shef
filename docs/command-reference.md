## Command Reference

### Basic Shef Command Structure

```
shef [category] [recipe-name]
```

### Global Flags

| Flag                  | Description                |
|-----------------------|----------------------------|
| `-h, --help`          | Show help information      |
| `-v, --version`       | Show version information   |
| `-d, --debug`         | Enable debug output        |
| `-c, --category`      | Specify a category         |
| `-L, --local`         | Force local recipes first  |
| `-U, --user`          | Force user recipes first   |
| `-P, --public`        | Force public recipes first |
| `-r, --recipe-file`   | Path to the recipe file    |

### Recipe Input and Flags

When running a recipe, you can provide both positional input text and custom flags:

| Input Type                 | Example                        | Access in Recipe  | Description                                             |
|----------------------------|--------------------------------|-------------------|---------------------------------------------------------|
| `-h, --help`               | `shef recipe -h`               | N/A               | Show help information for the recipe                    |
| Text Input                 | `shef recipe "My text"`        | `{{ .input }}`    | First argument after recipe name becomes input variable |
| Any short flag `-x`        | `shef recipe -x`               | `{{ .x }}`        | Any short flag becomes available as a boolean variable  |
| Any long flag `--var-name` | `shef recipe --var-name=value` | `{{ .var_name }}` | Any long flag becomes available as variable             |

For more details on argument types and usage, see the [Arguments and Flags](arguments-and-flags.md) section.

### Utility Commands

| Command                                  | Description                                                           |
|------------------------------------------|-----------------------------------------------------------------------|
| `sync` `s`                               | Sync public recipes locally                                           |
| `list` `ls` `l`                          | List available recipes (note: `demo` recipes are excluded by default) |
| `which` `w` \[category\] \[recipe-name\] | Show the location of a recipe file                                    |
