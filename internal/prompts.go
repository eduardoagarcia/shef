package internal

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// handlePrompt processes a prompt definition and returns the user's response
func handlePrompt(p Prompt, ctx *ExecutionContext) (interface{}, error) {
	vars := ctx.templateVars()

	message, err := renderTemplate(p.Message, vars)
	if err != nil {
		return nil, err
	}

	defaultValue, err := renderTemplate(p.Default, vars)
	if err != nil {
		return nil, err
	}

	helpText, err := renderTemplate(p.HelpText, vars)
	if err != nil {
		return nil, err
	}

	switch p.Type {
	case "input":
		return handleInputPrompt(message, defaultValue, helpText)
	case "select":
		return handleSelectPrompt(p, ctx, message, defaultValue, helpText)
	case "confirm":
		return handleConfirmPrompt(message, defaultValue, helpText)
	case "password":
		return handlePasswordPrompt(message, helpText)
	case "multiselect":
		return handleMultiselectPrompt(p, ctx, message, defaultValue, helpText)
	case "number":
		return handleNumberPrompt(p, message, defaultValue, helpText)
	case "editor":
		return handleEditorPrompt(p, message, defaultValue, helpText)
	case "path":
		return handlePathPrompt(p, message, defaultValue, helpText)
	case "autocomplete":
		return handleAutocompletePrompt(p, ctx, message, defaultValue, helpText)
	default:
		return nil, fmt.Errorf("unknown prompt type: %s", p.Type)
	}
}

// handleInputPrompt displays a simple text input prompt
func handleInputPrompt(message, defaultValue, helpText string) (string, error) {
	var answer string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
		Help:    helpText,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// handleSelectPrompt displays a selection menu prompt
func handleSelectPrompt(p Prompt, ctx *ExecutionContext, message, defaultValue, helpText string) (string, error) {
	options, err := getPromptOptions(p, ctx)
	if err != nil {
		return "", err
	}

	defaultVal := getDefaultOption(defaultValue, options)

	var answer string
	prompt := &survey.Select{
		Message: message,
		Options: options,
		Default: defaultVal,
		Help:    helpText,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// handleConfirmPrompt displays a yes/no confirmation prompt
func handleConfirmPrompt(message, defaultValue, helpText string) (bool, error) {
	var answer bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue == "true",
		Help:    helpText,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return false, err
	}
	return answer, nil
}

// handlePasswordPrompt displays a masked password input prompt
func handlePasswordPrompt(message, helpText string) (string, error) {
	var answer string
	prompt := &survey.Password{
		Message: message,
		Help:    helpText,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// handleMultiselectPrompt displays a multi-option selection prompt
func handleMultiselectPrompt(p Prompt, ctx *ExecutionContext, message, defaultValue, helpText string) ([]string, error) {
	options, err := getPromptOptions(p, ctx)
	if err != nil {
		return nil, err
	}

	defaultOptions := parseDefaultOptions(defaultValue, options)

	var answer []string
	prompt := &survey.MultiSelect{
		Message: message,
		Options: options,
		Default: defaultOptions,
		Help:    helpText,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return nil, err
	}
	return answer, nil
}

// handleNumberPrompt displays a numeric input prompt with validation
func handleNumberPrompt(p Prompt, message, defaultValue, helpText string) (int, error) {
	var answer int
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
		Help:    helpText,
	}

	validator := numberValidator(p.MinValue, p.MaxValue)

	if err := survey.AskOne(prompt, &answer, survey.WithValidator(validator)); err != nil {
		return 0, err
	}
	return answer, nil
}

// handleEditorPrompt displays a text editor for multi-line input
func handleEditorPrompt(p Prompt, message, defaultValue, helpText string) (string, error) {
	var answer string
	editorCmd := getEditorCommand(p.EditorCmd)

	prompt := &survey.Editor{
		Message:       message,
		Default:       defaultValue,
		Help:          helpText,
		HideDefault:   true,
		AppendDefault: true,
		Editor:        editorCmd,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// handlePathPrompt displays a file path input with validation
func handlePathPrompt(p Prompt, message, defaultValue, helpText string) (string, error) {
	var answer string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
		Help:    helpText,
	}

	validator := pathValidator(p.Required, p.FileExtensions)

	if err := survey.AskOne(prompt, &answer, survey.WithValidator(validator)); err != nil {
		return "", err
	}
	return answer, nil
}

// handleAutocompletePrompt displays a filterable selection menu
func handleAutocompletePrompt(p Prompt, ctx *ExecutionContext, message, defaultValue, helpText string) (string, error) {
	options, err := getPromptOptions(p, ctx)
	if err != nil {
		return "", err
	}

	defaultVal := getDefaultOption(defaultValue, options)

	var answer string
	prompt := &survey.Select{
		Message: message,
		Options: options,
		Default: defaultVal,
		Help:    helpText,
		Filter:  filterOptionsBySubstring,
	}
	if err := survey.AskOne(prompt, &answer); err != nil {
		return "", err
	}
	return answer, nil
}

// filterOptionsBySubstring filters selection options by case-insensitive substring matching
func filterOptionsBySubstring(filterValue string, optValue string, idx int) bool {
	return strings.Contains(strings.ToLower(optValue), strings.ToLower(filterValue))
}

// getDefaultOption finds a valid default option from the options list
func getDefaultOption(defaultValue string, options []string) string {
	if defaultValue == "" {
		return options[0]
	}

	for _, opt := range options {
		if opt == defaultValue {
			return defaultValue
		}
	}

	return options[0]
}

// parseDefaultOptions extracts valid default options from a comma-separated string
func parseDefaultOptions(defaultValue string, options []string) []string {
	if defaultValue == "" {
		return nil
	}

	var validDefaults []string
	defaultOptions := strings.Split(defaultValue, ",")

	for _, def := range defaultOptions {
		trimmed := strings.TrimSpace(def)
		for _, opt := range options {
			if trimmed == opt {
				validDefaults = append(validDefaults, trimmed)
				break
			}
		}
	}

	return validDefaults
}

// getPromptOptions retrieves the options for selection-type prompts
func getPromptOptions(p Prompt, ctx *ExecutionContext) ([]string, error) {
	if p.SourceOp == "" {
		if len(p.Options) > 0 && p.Type != "multiselect" {
			return append(p.Options, ExitPrompt), nil
		}
		return p.Options, nil
	}

	return getOptionsFromSourceOp(p, ctx)
}

// getOptionsFromSourceOp extracts options from a source operation's output
func getOptionsFromSourceOp(p Prompt, ctx *ExecutionContext) ([]string, error) {
	output, exists := ctx.OperationOutputs[p.SourceOp]
	if !exists {
		return nil, fmt.Errorf("source operation %s not found or has no output", p.SourceOp)
	}

	if p.SourceTransform != "" {
		transformedOutput, err := transformOutput(output, p.SourceTransform, ctx)
		if err != nil {
			return nil, fmt.Errorf("transformation failed: %w", err)
		}
		return finalizeOptions(transformedOutput, p.Type), nil
	}

	options := parseOptionsFromOutput(output)
	if len(options) == 0 {
		return nil, fmt.Errorf("no options found from source operation %s", p.SourceOp)
	}

	return finalizeOptions(strings.Join(options, "\n"), p.Type), nil
}

// finalizeOptions adds exit option if needed and returns the final options list
func finalizeOptions(output string, promptType string) []string {
	options := parseOptionsFromOutput(output)

	if len(options) > 0 && promptType != "multiselect" {
		return append(options, ExitPrompt)
	}

	return options
}

// parseOptionsFromOutput converts multi-line output to a string slice of options
func parseOptionsFromOutput(output string) []string {
	result := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// getEditorCommand determines which editor to use for the prompt
func getEditorCommand(configuredEditor string) string {
	if configuredEditor != "" {
		return configuredEditor
	}

	editorCmd := os.Getenv("EDITOR")
	if editorCmd != "" {
		return editorCmd
	}

	return "vim" // Default editor
}

// numberValidator returns a validator for numeric input with range checking
func numberValidator(minValue, maxValue int) survey.Validator {
	return survey.ComposeValidators(
		survey.Required,
		func(val interface{}) error {
			str, ok := val.(string)
			if !ok {
				return fmt.Errorf("expected string value")
			}
			num, err := strconv.Atoi(str)
			if err != nil {
				return fmt.Errorf("please enter a valid number")
			}

			if minValue != 0 || maxValue != 0 {
				if minValue != 0 && num < minValue {
					return fmt.Errorf("value must be at least %d", minValue)
				}
				if maxValue != 0 && num > maxValue {
					return fmt.Errorf("value must be at most %d", maxValue)
				}
			}
			return nil
		},
	)
}

// pathValidator returns a validator for file path input
func pathValidator(required bool, fileExtensions []string) survey.Validator {
	return survey.ComposeValidators(
		func(val interface{}) error {
			if !required {
				return nil
			}
			str, ok := val.(string)
			if !ok || str == "" {
				return fmt.Errorf("path is required")
			}
			return nil
		},
		func(val interface{}) error {
			str, ok := val.(string)
			if !ok || str == "" {
				return nil
			}

			_, err := os.Stat(str)
			if err != nil {
				return fmt.Errorf("invalid path: %v", err)
			}

			if len(fileExtensions) > 0 {
				ext := strings.ToLower(filepath.Ext(str))
				if ext == "" {
					return fmt.Errorf("file must have an extension")
				}

				validExt := false
				for _, allowedExt := range fileExtensions {
					allowedExt = strings.ToLower(allowedExt)
					if ext == allowedExt || ext == "."+allowedExt {
						validExt = true
						break
					}
				}

				if !validExt {
					return fmt.Errorf("file must have one of these extensions: %s", strings.Join(fileExtensions, ", "))
				}
			}

			return nil
		},
	)
}
