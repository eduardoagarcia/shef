## Conditional Execution

Operations can be conditionally executed:

### Basic Conditions

```yaml
condition: .confirm == "true"  # Run only if confirm prompt is true
```

### Operation Result Conditions

```yaml
condition: build_op.success  # Run if build_op succeeded
condition: test_op.failure   # Run if test_op failed
```

### Variable Comparison

```yaml
condition: .environment == "production"  # Equality check
condition: .count != 0                   # Inequality check
```

### Numeric Comparison

```yaml
condition: .count > 5
condition: .memory <= 512
condition: .errors >= 10
condition: .progress < 100
```

### Complex Logic

```yaml
condition: build_op.success && .confirm_deploy == "true"
condition: test_op.failure || lint_op.failure
condition: !.skip_validation
```
