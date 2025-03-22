package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
)

func handlePrompt(p Prompt, ctx *ExecutionContext) (interface{}, error) {
	vars := ctx.templateVars()

	msgTemplate, err := template.New("message").Funcs(templateFuncs).Parse(p.Message)
	if err != nil {
		return nil, err
	}

	var msgBuf bytes.Buffer
	if err := msgTemplate.Execute(&msgBuf, vars); err != nil {
		return nil, err
	}
	message := msgBuf.String()

	defaultValue := ""
	if p.Default != "" {
		defaultTemplate, err := template.New("default").Funcs(templateFuncs).Parse(p.Default)
		if err != nil {
			return nil, err
		}

		var defaultBuf bytes.Buffer
		if err := defaultTemplate.Execute(&defaultBuf, vars); err != nil {
			return nil, err
		}
		defaultValue = defaultBuf.String()
	}

	helpText := ""
	if p.HelpText != "" {
		helpTemplate, err := template.New("help").Funcs(templateFuncs).Parse(p.HelpText)
		if err != nil {
			return nil, err
		}

		var helpBuf bytes.Buffer
		if err := helpTemplate.Execute(&helpBuf, vars); err != nil {
			return nil, err
		}
		helpText = helpBuf.String()
	}

	switch p.Type {
	case "input":
		var answer string
		prompt := &survey.Input{
			Message: message,
			Default: defaultValue,
			Help:    helpText,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "select":
		options, err := getPromptOptions(p, ctx)
		if err != nil {
			return nil, err
		}

		defaultExists := false
		if defaultValue != "" {
			for _, opt := range options {
				if opt == defaultValue {
					defaultExists = true
					break
				}
			}
		}

		if !defaultExists && len(options) > 0 {
			defaultValue = options[0]
		}

		var answer string
		prompt := &survey.Select{
			Message: message,
			Options: options,
			Default: defaultValue,
			Help:    helpText,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "confirm":
		var answer bool
		prompt := &survey.Confirm{
			Message: message,
			Default: defaultValue == "true",
			Help:    helpText,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "password":
		var answer string
		prompt := &survey.Password{
			Message: message,
			Help:    helpText,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "multiselect":
		options, err := getPromptOptions(p, ctx)
		if err != nil {
			return nil, err
		}

		var defaultOptions []string
		if defaultValue != "" {
			defaultOptions = strings.Split(defaultValue, ",")
			for i, opt := range defaultOptions {
				defaultOptions[i] = strings.TrimSpace(opt)
			}
		}

		var validDefaults []string
		for _, def := range defaultOptions {
			for _, opt := range options {
				if def == opt {
					validDefaults = append(validDefaults, def)
					break
				}
			}
		}

		var answer []string
		prompt := &survey.MultiSelect{
			Message: message,
			Options: options,
			Default: validDefaults,
			Help:    helpText,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "number":
		var answer int
		prompt := &survey.Input{
			Message: message,
			Default: defaultValue,
			Help:    helpText,
		}

		validator := survey.ComposeValidators(
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

				if p.MinValue != 0 || p.MaxValue != 0 {
					if p.MinValue != 0 && num < p.MinValue {
						return fmt.Errorf("value must be at least %d", p.MinValue)
					}
					if p.MaxValue != 0 && num > p.MaxValue {
						return fmt.Errorf("value must be at most %d", p.MaxValue)
					}
				}
				return nil
			},
		)

		if err := survey.AskOne(prompt, &answer, survey.WithValidator(validator)); err != nil {
			return nil, err
		}
		return answer, nil

	case "editor":
		var answer string
		editorCmd := p.EditorCmd
		if editorCmd == "" {
			editorCmd = os.Getenv("EDITOR")
			if editorCmd == "" {
				editorCmd = "vim" // Default editor
			}
		}

		prompt := &survey.Editor{
			Message:       message,
			Default:       defaultValue,
			Help:          helpText,
			HideDefault:   true,
			AppendDefault: true,
			Editor:        editorCmd,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "path":
		var answer string
		prompt := &survey.Input{
			Message: message,
			Default: defaultValue,
			Help:    helpText,
		}

		validator := survey.ComposeValidators(
			func(val interface{}) error {
				if !p.Required {
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

				if len(p.FileExtensions) > 0 {
					ext := strings.ToLower(filepath.Ext(str))
					if ext == "" {
						return fmt.Errorf("file must have an extension")
					}

					validExt := false
					for _, allowedExt := range p.FileExtensions {
						if ext == strings.ToLower(allowedExt) || ext == strings.ToLower("."+allowedExt) {
							validExt = true
							break
						}
					}

					if !validExt {
						return fmt.Errorf("file must have one of these extensions: %s", strings.Join(p.FileExtensions, ", "))
					}
				}

				return nil
			},
		)

		if err := survey.AskOne(prompt, &answer, survey.WithValidator(validator)); err != nil {
			return nil, err
		}
		return answer, nil

	case "autocomplete":
		options, err := getPromptOptions(p, ctx)
		if err != nil {
			return nil, err
		}

		defaultExists := false
		if defaultValue != "" {
			for _, opt := range options {
				if opt == defaultValue {
					defaultExists = true
					break
				}
			}
		}

		if !defaultExists && len(options) > 0 {
			defaultValue = options[0]
		}

		var answer string
		prompt := &survey.Select{
			Message: message,
			Options: options,
			Default: defaultValue,
			Help:    helpText,
			Filter: func(filterValue string, optValue string, idx int) bool {
				return strings.Contains(strings.ToLower(optValue), strings.ToLower(filterValue))
			},
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	default:
		return nil, fmt.Errorf("unknown prompt type: %s", p.Type)
	}
}

func getPromptOptions(p Prompt, ctx *ExecutionContext) ([]string, error) {
	if p.SourceOp == "" {
		if len(p.Options) > 0 && p.Type != "multiselect" {
			return append(p.Options, ExitPrompt), nil
		}
		return p.Options, nil
	}

	output, exists := ctx.OperationOutputs[p.SourceOp]
	if !exists {
		return nil, fmt.Errorf("source operation %s not found or has no output", p.SourceOp)
	}

	if p.SourceTransform != "" {
		transformedOutput, err := transformOutput(output, p.SourceTransform, ctx)
		if err != nil {
			return nil, fmt.Errorf("transformation failed: %w", err)
		}
		options := parseOptionsFromOutput(transformedOutput)
		if len(options) > 0 && p.Type != "multiselect" {
			return append(options, ExitPrompt), nil
		}
		return options, nil
	}

	options := parseOptionsFromOutput(output)
	if len(options) == 0 {
		return nil, fmt.Errorf("no options found from source operation %s", p.SourceOp)
	}

	if p.Type != "multiselect" {
		options = append(options, ExitPrompt)
	}
	return options, nil
}

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
