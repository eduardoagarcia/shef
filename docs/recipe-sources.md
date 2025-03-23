## Recipe Sources

Shef looks for recipes in multiple locations and contexts:

### Standard Paths (All Platforms)

1. **Local Recipes**: `./.shef/*.yaml` in the current directory
2. **User Recipes**: `~/.shef/user/*.yaml` in your home directory
3. **Public Recipes**: `~/.shef/public/*.yaml` in your home directory

### XDG Base Directory Paths (Linux Only)

On Linux systems, Shef also supports the XDG Base Directory Specification:

1. **User Recipes**: `$XDG_CONFIG_HOME/shef/user/*.yaml` (defaults to `~/.config/shef/user/*.yaml`)
2. **Public Recipes**: `$XDG_DATA_HOME/shef/public/*.yaml` (defaults to `~/.local/share/shef/public/*.yaml`)

Shef includes both standard and XDG paths on Linux systems.

### Source Priority

If you have recipes with the same name and category in different locations, you can prioritize a specific source:

```bash
shef -L git version  # Prioritize local recipes
shef -U git version  # Prioritize user recipes
shef -P git version  # Prioritize public recipes
```
