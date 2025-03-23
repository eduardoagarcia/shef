## Recipe Help Documentation

Shef provides a built-in help system for recipes, allowing users to get detailed information about a recipe's purpose,
requirements, and usage without having to read the recipe code.

### Accessing Recipe Help

Help for any recipe can be accessed in two ways:

```bash
# Using the -h flag
shef demo hello-world -h

# Using the --help flag
shef demo hello-world --help
```

### Help Output Format

When a user requests help for a recipe, they receive formatted information:

```
NAME:
    recipe-name - short description from the description field

CATEGORY:
    recipe-category

USAGE:
    shef recipe-name [input] [options]
    shef category recipe-name [input] [options]

OVERVIEW:
    Detailed help text from the help field.
    If no help field is provided, Shef shows a default message.
```

### Writing Effective Help Documentation

When creating recipes, consider including comprehensive help text that covers:

1. **Purpose**: What the recipe does and when to use it
2. **Requirements**: Any prerequisites or dependencies
3. **Examples**: Sample usages with different options
4. **Parameters**: Available flags and arguments with explanations
5. **Expected Output**: What users should expect to see
6. **Troubleshooting**: Common issues and how to resolve them

#### Example Recipe with Help Documentation

```yaml
recipes:
  - name: "database-backup"
    description: "Backup a MySQL database"
    category: "database"
    help: |
      This recipe creates a backup of a MySQL database and stores it in a
      timestamped file.

      Requirements:
        - MySQL client tools must be installed
        - Database credentials with read permissions

      Examples:
        shef database-backup                    # Uses interactive prompts
        shef database-backup --db=mydb          # Specify database name
        shef database-backup --no-compress      # Skip compression
        shef database-backup --output=/backups  # Custom output directory

      Flags:
        --db=NAME      # Database name
        --user=USER    # Database username
        --pass=PASS    # Database password
        --host=HOST    # Database host (default: localhost)
        --no-compress  # Skip compression
        --output=DIR   # Output directory (default: ./backups)
    operations:
    # Operations follow here
```
