package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/agnivade/levenshtein"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const (
	Version               = "v0.1.10"
	GithubRepo            = "https://github.com/eduardoagarcia/shef"
	PublicRecipesFilename = "recipes.tar.gz"
	PublicRecipesFolder   = "recipes"
)

type Config struct {
	Recipes []Recipe `yaml:"recipes"`
}

type Recipe struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Category    string      `yaml:"category,omitempty"`
	Author      string      `yaml:"author,omitempty"`
	Operations  []Operation `yaml:"operations"`
}

type Operation struct {
	Name          string      `yaml:"name"`
	ID            string      `yaml:"id,omitempty"`
	Command       string      `yaml:"command,omitempty"`
	ControlFlow   interface{} `yaml:"control_flow,omitempty"`
	Operations    []Operation `yaml:"operations,omitempty"`
	ExecutionMode string      `yaml:"execution_mode,omitempty"`
	OutputFormat  string      `yaml:"output_format,omitempty"`
	Silent        bool        `yaml:"silent,omitempty"`
	Condition     string      `yaml:"condition,omitempty"`
	OnSuccess     string      `yaml:"on_success,omitempty"`
	OnFailure     string      `yaml:"on_failure,omitempty"`
	Transform     string      `yaml:"transform,omitempty"`
	Prompts       []Prompt    `yaml:"prompts,omitempty"`
	Exit          bool        `yaml:"exit,omitempty"`
}

type Prompt struct {
	Name            string            `yaml:"name"`
	Type            string            `yaml:"type"`
	Message         string            `yaml:"message"`
	Default         string            `yaml:"default,omitempty"`
	Options         []string          `yaml:"options,omitempty"`
	SourceOp        string            `yaml:"source_operation,omitempty"`
	SourceTransform string            `yaml:"source_transform,omitempty"`
	MinValue        int               `yaml:"min_value,omitempty"`
	MaxValue        int               `yaml:"max_value,omitempty"`
	Required        bool              `yaml:"required,omitempty"`
	FileExtensions  []string          `yaml:"file_extensions,omitempty"`
	MultipleLimit   int               `yaml:"multiple_limit,omitempty"`
	EditorCmd       string            `yaml:"editor_cmd,omitempty"`
	HelpText        string            `yaml:"help_text,omitempty"`
	Validators      []PromptValidator `yaml:"validators,omitempty"`
}

type PromptValidator struct {
	Type    string `yaml:"type"`
	Pattern string `yaml:"pattern,omitempty"`
	Message string `yaml:"message,omitempty"`
	Min     int    `yaml:"min,omitempty"`
	Max     int    `yaml:"max,omitempty"`
}

type ExecutionContext struct {
	Data             string
	Vars             map[string]interface{}
	OperationOutputs map[string]string
	OperationResults map[string]bool
}

func (ctx *ExecutionContext) templateVars() map[string]interface{} {
	vars := make(map[string]interface{})

	for k, v := range ctx.Vars {
		vars[k] = v
	}

	for opID, output := range ctx.OperationOutputs {
		vars[opID] = output
	}

	vars["operationOutputs"] = ctx.OperationOutputs
	vars["operationResults"] = ctx.OperationResults

	return vars
}

var colorCodes = map[string]string{
	"black":      "\033[30m",
	"red":        "\033[31m",
	"green":      "\033[32m",
	"yellow":     "\033[33m",
	"blue":       "\033[34m",
	"magenta":    "\033[35m",
	"cyan":       "\033[36m",
	"white":      "\033[37m",
	"bg-black":   "\033[40m",
	"bg-red":     "\033[41m",
	"bg-green":   "\033[42m",
	"bg-yellow":  "\033[43m",
	"bg-blue":    "\033[44m",
	"bg-magenta": "\033[45m",
	"bg-cyan":    "\033[46m",
	"bg-white":   "\033[47m",
	"reset":      "\033[0m",
}

var styleCodes = map[string]string{
	"bold":      "\033[1m",
	"dim":       "\033[2m",
	"italic":    "\033[3m",
	"underline": "\033[4m",
	"reset":     "\033[0m",
}

var templateFuncs = template.FuncMap{
	"split":      strings.Split,
	"join":       strings.Join,
	"joinArray":  JoinArray,
	"trim":       strings.TrimSpace,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"contains":   strings.Contains,
	"replace":    strings.ReplaceAll,
	"filter":     filterLines,
	"grep":       filterLines,
	"cut":        cutFields,
	"exec":       execCommand,
	"atoi": func(s string) int {
		var i int
		_, err := fmt.Sscanf(s, "%d", &i)
		if err != nil {
			return 0
		}
		return i
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"mul": func(a, b int) int { return a * b },
	"color": func(color string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := colorCodes[strings.ToLower(color)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + colorCodes["reset"]
	},
	"style": func(styleType string, text interface{}) string {
		if os.Getenv("NO_COLOR") != "" {
			return fmt.Sprintf("%v", text)
		}

		code, ok := styleCodes[strings.ToLower(styleType)]
		if !ok {
			return fmt.Sprintf("%v", text)
		}

		return code + fmt.Sprintf("%v", text) + styleCodes["reset"]
	},
	"resetFormat": func() string {
		if os.Getenv("NO_COLOR") != "" {
			return ""
		}
		return colorCodes["reset"]
	},
}

func JoinArray(arr interface{}, sep string) string {
	switch v := arr.(type) {
	case []string:
		return strings.Join(v, sep)
	case []interface{}:
		strs := make([]string, len(v))
		for i, val := range v {
			strs[i] = fmt.Sprintf("%v", val)
		}
		return strings.Join(strs, sep)
	default:
		return fmt.Sprintf("%v", arr)
	}
}

func filterLines(input, pattern string) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func cutFields(input, delimiter string, field int) string {
	var result []string
	for _, line := range strings.Split(input, "\n") {
		fields := strings.Split(line, delimiter)
		if field < len(fields) {
			result = append(result, strings.TrimSpace(fields[field]))
		}
	}
	return strings.Join(result, "\n")
}

func execCommand(cmd string) string {
	output, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func renderTemplate(tmplStr string, vars map[string]interface{}) (string, error) {
	tmpl, err := template.New("template").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	result := buf.String()
	result = strings.ReplaceAll(result, "<no value>", "false")

	return result, nil
}

func transformOutput(output, transform string, ctx *ExecutionContext) (string, error) {
	vars := ctx.templateVars()
	vars["input"] = output
	vars["output"] = output

	return renderTemplate(transform, vars)
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadRecipes(sources []string, category string) ([]Recipe, error) {
	var allRecipes []Recipe

	for _, source := range sources {
		config, err := loadConfig(source)
		if err != nil {
			fmt.Printf("Warning: Failed to load recipes from %s: %v\n", source, err)
			continue
		}

		if category == "" {
			allRecipes = append(allRecipes, config.Recipes...)
			continue
		}

		for _, recipe := range config.Recipes {
			if recipe.Category == category {
				allRecipes = append(allRecipes, recipe)
			}
		}
	}

	return allRecipes, nil
}

func findRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	findYamlFiles := func(root string) ([]string, error) {
		var files []string
		visited := make(map[string]bool)

		var walkDir func(path string) error
		walkDir = func(path string) error {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil
			}

			if visited[absPath] {
				return nil
			}
			visited[absPath] = true

			// Get file info, will follow symlinks
			fileInfo, err := os.Stat(path)
			if err != nil {
				return nil
			}

			if fileInfo.IsDir() {
				entries, err := os.ReadDir(path)
				if err != nil {
					return nil
				}

				for _, entry := range entries {
					entryPath := filepath.Join(path, entry.Name())
					if err := walkDir(entryPath); err != nil {
						return err
					}
				}
			} else if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}

			return nil
		}

		err := walkDir(root)
		return files, err
	}

	if localDir {
		if _, err := os.Stat(".shef"); err == nil {
			if localFiles, err := findYamlFiles(".shef"); err == nil {
				sources = append(sources, localFiles...)
			}
		}
	}

	if userDir || publicRepo {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			userRoot := filepath.Join(homeDir, ".shef")
			if _, err := os.Stat(userRoot); err == nil {
				if userFiles, err := findYamlFiles(userRoot); err == nil {
					sources = append(sources, userFiles...)
				}
			}

			if isLinux() {
				sources = addXDGRecipeSources(sources, userDir, publicRepo, findYamlFiles)
			}
		}
	}

	return sources, nil
}

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
		return parseOptionsFromOutput(transformedOutput), nil
	}

	options := parseOptionsFromOutput(output)
	if len(options) == 0 {
		return nil, fmt.Errorf("no options found from source operation %s", p.SourceOp)
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

func executeCommand(cmdStr string, input string, executionMode string, outputFormat string) (string, error) {
	if executionMode == "" {
		executionMode = "standard"
	}

	if executionMode == "standard" {
		cmd := exec.Command("sh", "-c", cmdStr)

		if input != "" {
			cmd.Stdin = strings.NewReader(input)
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("command failed: %w\nStderr: %s", err, stderr.String())
		}

		switch outputFormat {
		case "trim":
			return strings.TrimSpace(stdout.String()), nil
		case "lines":
			var lines []string
			for _, line := range strings.Split(stdout.String(), "\n") {
				if trimmedLine := strings.TrimSpace(line); trimmedLine != "" {
					lines = append(lines, trimmedLine)
				}
			}
			return strings.Join(lines, "\n"), nil
		case "raw", "":
			return stdout.String(), nil
		default:
			return stdout.String(), nil
		}
	}

	// Execute command with interactive or streaming mode
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-sigChan:
		err := cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return "", err
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			err := cmd.Process.Kill()
			if err != nil {
				return "", err
			}
		}
		return "", nil

	case err := <-done:
		return "", err
	}
}

func evaluateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	if condition == "" {
		return true, nil
	}

	if strings.Contains(condition, "{{") && strings.Contains(condition, "}}") {
		rendered, err := renderTemplate(condition, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render condition template: %w", err)
		}
		rendered = strings.ReplaceAll(rendered, "<no value>", "false")
		return evaluateCondition(rendered, ctx)
	}

	condition = strings.TrimSpace(condition)

	if strings.Contains(condition, "&&") {
		return evaluateAndCondition(condition, ctx)
	}

	if strings.Contains(condition, "||") {
		return evaluateOrCondition(condition, ctx)
	}

	if strings.HasPrefix(condition, "!") {
		return evaluateNotCondition(condition, ctx)
	}

	result, err := evaluateNumericComparison(condition, ctx)
	if err == nil {
		return result, nil
	}

	if strings.Contains(condition, ".success") || strings.Contains(condition, ".failure") {
		return evaluateOperationResult(condition, ctx)
	}

	if strings.Contains(condition, "==") || strings.Contains(condition, "!=") {
		return evaluateVariableComparison(condition, ctx)
	}

	if condition == "true" {
		return true, nil
	}
	if condition == "false" {
		return false, nil
	}

	return false, fmt.Errorf("unsupported condition format: %s", condition)
}

func evaluateAndCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, "&&")
	for _, part := range parts {
		result, err := evaluateCondition(part, ctx)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func evaluateOrCondition(condition string, ctx *ExecutionContext) (bool, error) {
	parts := strings.Split(condition, "||")
	for _, part := range parts {
		result, err := evaluateCondition(part, ctx)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

func evaluateNotCondition(condition string, ctx *ExecutionContext) (bool, error) {
	subCondition := strings.TrimSpace(condition[1:])
	result, err := evaluateCondition(subCondition, ctx)
	if err != nil {
		return false, err
	}
	return !result, nil
}

func evaluateNumericComparison(condition string, ctx *ExecutionContext) (bool, error) {
	var op string
	var parts []string

	if strings.Contains(condition, ">=") {
		parts = strings.Split(condition, ">=")
		op = ">="
	} else if strings.Contains(condition, "<=") {
		parts = strings.Split(condition, "<=")
		op = "<="
	} else if strings.Contains(condition, ">") {
		parts = strings.Split(condition, ">")
		op = ">"
	} else if strings.Contains(condition, "<") {
		parts = strings.Split(condition, "<")
		op = "<"
	} else {
		return false, fmt.Errorf("not a numeric comparison")
	}

	if len(parts) != 2 {
		return false, fmt.Errorf("invalid numeric comparison format: %s", condition)
	}

	leftStr, err := resolveValue(strings.TrimSpace(parts[0]), ctx)
	if err != nil {
		return false, err
	}

	rightStr, err := resolveValue(strings.TrimSpace(parts[1]), ctx)
	if err != nil {
		return false, err
	}

	if leftStr == "false" {
		leftStr = "0"
	}
	if rightStr == "false" {
		rightStr = "0"
	}

	leftInt, leftErr := strconv.Atoi(leftStr)
	rightInt, rightErr := strconv.Atoi(rightStr)

	if leftErr == nil && rightErr == nil {
		switch op {
		case ">":
			return leftInt > rightInt, nil
		case "<":
			return leftInt < rightInt, nil
		case ">=":
			return leftInt >= rightInt, nil
		default:
			return leftInt <= rightInt, nil
		}
	}

	leftFloat, leftErr := strconv.ParseFloat(leftStr, 64)
	rightFloat, rightErr := strconv.ParseFloat(rightStr, 64)

	if leftErr != nil || rightErr != nil {
		return false, fmt.Errorf("numeric comparison requires numeric values, got '%s' and '%s'", leftStr, rightStr)
	}

	switch op {
	case ">":
		return leftFloat > rightFloat, nil
	case "<":
		return leftFloat < rightFloat, nil
	case ">=":
		return leftFloat >= rightFloat, nil
	default:
		return leftFloat <= rightFloat, nil
	}
}

func evaluateOperationResult(condition string, ctx *ExecutionContext) (bool, error) {
	if strings.Contains(condition, ".success") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "success" {
			opID := strings.TrimSpace(parts[0])
			result, exists := ctx.OperationResults[opID]
			if exists {
				return result, nil
			}
			return false, nil
		}
	}

	if strings.Contains(condition, ".failure") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "failure" {
			opID := strings.TrimSpace(parts[0])
			result, exists := ctx.OperationResults[opID]
			if exists {
				return !result, nil
			}
			return true, nil
		}
	}

	return false, fmt.Errorf("invalid operation result condition: %s", condition)
}

func evaluateVariableComparison(condition string, ctx *ExecutionContext) (bool, error) {
	var op string
	var parts []string

	if strings.Contains(condition, "==") {
		parts = strings.Split(condition, "==")
		op = "=="
	} else if strings.Contains(condition, "!=") {
		parts = strings.Split(condition, "!=")
		op = "!="
	} else {
		return false, fmt.Errorf("not a variable comparison")
	}

	if len(parts) != 2 {
		return false, fmt.Errorf("invalid variable comparison format: %s", condition)
	}

	leftPart := strings.TrimSpace(parts[0])
	rightPart := strings.TrimSpace(parts[1])
	rightPart = strings.Trim(rightPart, "\"'")

	varName := leftPart
	if strings.HasPrefix(varName, "$") {
		varName = varName[1:]
	}
	if strings.HasPrefix(varName, ".") {
		varName = varName[1:]
	}

	var actualValue string
	var exists bool

	if value, ok := ctx.Vars[varName]; ok {
		actualValue = fmt.Sprintf("%v", value)
		exists = true
	}

	if !exists {
		if value, ok := ctx.OperationOutputs[varName]; ok {
			actualValue = value
			exists = true
		}
	}

	if !exists {
		actualValue = "false"
	}

	var result bool
	if op == "==" {
		result = actualValue == rightPart
	} else {
		result = actualValue != rightPart
	}

	return result, nil
}

func resolveValue(value string, ctx *ExecutionContext) (string, error) {
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		rendered, err := renderTemplate(value, ctx.templateVars())
		if err != nil {
			return "", err
		}
		return rendered, nil
	}

	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, ".") {
		varName := value
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:]
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:]
		}

		if val, exists := ctx.Vars[varName]; exists {
			return fmt.Sprintf("%v", val), nil
		}
		if val, exists := ctx.OperationOutputs[varName]; exists {
			return val, nil
		}
		return "false", nil
	}

	return value, nil
}

func executeRecipe(recipe Recipe, input string, vars map[string]interface{}, debug bool) error {
	ctx := ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
	}

	for k, v := range vars {
		ctx.Vars[k] = v
	}

	if input != "" {
		ctx.Vars["input"] = input
		ctx.Data = input
	}

	opMap := make(map[string]Operation)
	registerOperations(recipe.Operations, opMap)

	handlerIDs := make(map[string]bool)
	identifyHandlers(recipe.Operations, handlerIDs)

	if debug {
		fmt.Println("Registered operations:")
		for id := range opMap {
			handlerStatus := ""
			if handlerIDs[id] {
				handlerStatus = " (handler)"
			}
			fmt.Printf("  - %s%s\n", id, handlerStatus)
		}
	}

	var executeOp func(op Operation, depth int) (bool, error)
	executeOp = func(op Operation, depth int) (bool, error) {
		if depth > 50 {
			return false, fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		if op.ControlFlow != nil {
			flowMap, ok := op.ControlFlow.(map[string]interface{})
			if !ok {
				return false, fmt.Errorf("invalid control_flow structure")
			}

			typeVal, ok := flowMap["type"].(string)
			if !ok {
				return false, fmt.Errorf("control_flow requires a 'type' field")
			}

			switch typeVal {

			case "foreach":
				forEach, err := op.GetForEachFlow()
				if err != nil {
					return false, err
				}
				err = ExecuteForEach(op, forEach, &ctx, depth, executeOp, debug)
				return op.Exit, err

			case "while":
				whileFlow, err := op.GetWhileFlow()
				if err != nil {
					return false, err
				}
				err = ExecuteWhile(op, whileFlow, &ctx, depth, executeOp, debug)
				return op.Exit, err

			case "for":
				forFlow, err := op.GetForFlow()
				if err != nil {
					return false, err
				}
				err = ExecuteFor(op, forFlow, &ctx, depth, executeOp, debug)
				return op.Exit, err

			default:
				return false, fmt.Errorf("unknown control_flow type: %s", typeVal)
			}
		}

		if op.Condition != "" {
			if debug {
				fmt.Printf("Evaluating condition: %s\n", op.Condition)
			}
			result, err := evaluateCondition(op.Condition, &ctx)
			if err != nil {
				return false, fmt.Errorf("condition evaluation failed: %w", err)
			}

			if !result {
				if debug {
					fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
				}
				return false, nil
			}
		}

		for _, prompt := range op.Prompts {
			value, err := handlePrompt(prompt, &ctx)
			if err != nil {
				return false, err
			}
			ctx.Vars[prompt.Name] = value
		}

		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render command template: %w", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		ctx.Vars["error"] = ""

		output, err := executeCommand(cmd, ctx.Data, op.ExecutionMode, op.OutputFormat)
		operationSuccess := err == nil

		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
		}

		if err != nil {
			ctx.Vars["error"] = err.Error()

			if debug {
				fmt.Printf("Warning: command execution had errors: %v\n", err)
			}

			if op.OnFailure != "" {
				if debug {
					fmt.Printf("Executing on_failure handler: %s\n", op.OnFailure)
				}

				nextOp, exists := opMap[op.OnFailure]
				if !exists {
					return false, fmt.Errorf("on_failure operation %s not found", op.OnFailure)
				}
				shouldExit, err := executeOp(nextOp, depth+1)
				return shouldExit || op.Exit, err
			}

			fmt.Printf("Error in operation '%s': \n%v\n", op.Name, err)

			var continueExecution bool
			prompt := &survey.Confirm{
				Message: "Continue with recipe execution?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &continueExecution); err != nil {
				return false, err
			}

			if !continueExecution {
				return true, fmt.Errorf("recipe execution aborted by user after command error")
			}
		}

		if op.Transform != "" {
			transformedOutput, err := transformOutput(output, op.Transform, &ctx)
			if err != nil {
				if debug {
					fmt.Printf("Warning: output transformation failed: %v\n", err)
				}
			} else {
				output = transformedOutput
			}
		}

		ctx.Data = output

		if op.ID != "" {
			ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
		}

		if output != "" && !op.Silent {
			fmt.Println(output)
		}

		if op.OnSuccess != "" && operationSuccess {
			nextOp, exists := opMap[op.OnSuccess]
			if !exists {
				return false, fmt.Errorf("on_success operation %s not found", op.OnSuccess)
			}
			shouldExit, err := executeOp(nextOp, depth+1)
			return shouldExit || op.Exit, err
		}

		if debug {
			fmt.Printf("Operation %s result: %v\n", op.ID, ctx.OperationResults[op.ID])
			fmt.Printf("Handler for on_success: '%s'\n", op.OnSuccess)
			fmt.Printf("Handler for on_failure: '%s'\n", op.OnFailure)
			if op.Exit {
				fmt.Printf("Exit flag is set. Will exit after this operation.\n")
			}
		}

		return op.Exit, nil
	}

	for i, op := range recipe.Operations {
		if op.ID != "" && handlerIDs[op.ID] {
			if debug {
				fmt.Printf("Skipping handler operation %d: %s (ID: %s)\n", i+1, op.Name, op.ID)
			}
			continue
		}

		if debug {
			fmt.Printf("Executing operation %d: %s\n", i+1, op.Name)
		}

		shouldExit, err := executeOp(op, 0)
		if err != nil {
			return err
		}

		if shouldExit {
			if debug {
				fmt.Printf("Exiting recipe execution after operation: %s\n", op.Name)
			}
			return nil
		}
	}

	return nil
}

func registerOperations(operations []Operation, opMap map[string]Operation) {
	for _, op := range operations {
		if op.ID != "" {
			opMap[op.ID] = op
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			registerOperations(op.Operations, opMap)
		}
	}
}

func identifyHandlers(operations []Operation, handlerIDs map[string]bool) {
	for _, op := range operations {
		if op.OnSuccess != "" {
			handlerIDs[op.OnSuccess] = true
		}
		if op.OnFailure != "" {
			handlerIDs[op.OnFailure] = true
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			identifyHandlers(op.Operations, handlerIDs)
		}
	}
}

func listRecipes(recipes []Recipe) {
	if len(recipes) == 0 {
		fmt.Println("No recipes found.")
		return
	}

	fmt.Println("Available recipes:")

	categories := make(map[string][]Recipe)
	for _, recipe := range recipes {
		cat := recipe.Category
		if cat == "" {
			cat = "uncategorized"
		}
		categories[cat] = append(categories[cat], recipe)
	}

	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		catRecipes := categories[category]

		sort.Slice(catRecipes, func(i, j int) bool {
			return catRecipes[i].Name < catRecipes[j].Name
		})

		fmt.Printf("\n  [%s]\n", category)
		for _, recipe := range catRecipes {
			fmt.Printf("    - %s: %s\n", recipe.Name, recipe.Description)
		}
	}

	fmt.Printf("\n\n")
}

func findRecipeByName(recipes []Recipe, name string) (*Recipe, error) {
	for _, recipe := range recipes {
		if recipe.Name == name {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

func main() {
	app := &cli.App{
		Name:    "shef",
		Usage:   "Shef is a powerful CLI tool that lets you combine shell commands into reusable recipes.",
		Version: Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug output",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "local",
				Aliases: []string{"L"},
				Usage:   "Force local recipes first",
			},
			&cli.BoolFlag{
				Name:    "user",
				Aliases: []string{"U"},
				Usage:   "Force user recipes first",
			},
			&cli.BoolFlag{
				Name:    "public",
				Aliases: []string{"P"},
				Usage:   "Force public recipes first",
			},
			&cli.StringFlag{
				Name:    "category",
				Aliases: []string{"c"},
				Usage:   "Filter by category",
			},
			&cli.PathFlag{
				Name:    "recipe-file",
				Aliases: []string{"r"},
				Usage:   "Path to the recipe file",
			},
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			if len(args) == 0 {
				err := cli.ShowAppHelp(c)
				if err != nil {
					return err
				}
				return nil
			}

			sourcePriority := getSourcePriority(c)
			return handleRunCommand(c, args, sourcePriority)
		},
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available recipes",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output results in JSON format",
					},
					&cli.StringFlag{
						Name:    "category",
						Aliases: []string{"c"},
						Usage:   "Filter by category",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return handleListCommand(c, args, sourcePriority)
				},
			},
			{
				Name:    "sync",
				Aliases: []string{"s"},
				Usage:   "Sync public recipes locally",
				Action: func(c *cli.Context) error {
					return syncPublicRecipes()
				},
			},
			{
				Name:    "which",
				Aliases: []string{"w"},
				Usage:   "Show the location of a recipe file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "local",
						Aliases: []string{"L"},
						Usage:   "Force local recipes first",
					},
					&cli.BoolFlag{
						Name:    "user",
						Aliases: []string{"U"},
						Usage:   "Force user recipes first",
					},
					&cli.BoolFlag{
						Name:    "public",
						Aliases: []string{"P"},
						Usage:   "Force public recipes first",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return handleWhichCommand(args, sourcePriority)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func getSourcePriority(c *cli.Context) []string {
	if c.Bool("local") {
		return []string{"local", "user", "public"}
	} else if c.Bool("user") {
		return []string{"user", "local", "public"}
	} else if c.Bool("public") {
		return []string{"public", "local", "user"}
	}

	return []string{"local", "user", "public"}
}

func handleListCommand(c *cli.Context, args []string, sourcePriority []string) error {
	category := c.String("category")
	if category == "" && len(args) >= 1 {
		category = args[0]
	}

	var allRecipes []Recipe
	recipeMap := make(map[string]bool)

	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		recipes, _ := loadRecipes(sources, category)

		for _, r := range recipes {
			if !recipeMap[r.Name] {
				allRecipes = append(allRecipes, r)
				recipeMap[r.Name] = true
			}
		}
	}

	if category == "" {
		var filteredRecipes []Recipe
		for _, recipe := range allRecipes {
			if recipe.Category != "demo" {
				filteredRecipes = append(filteredRecipes, recipe)
			}
		}
		allRecipes = filteredRecipes
	}

	if len(allRecipes) == 0 {
		if c.Bool("json") {
			fmt.Println("[]")
			return nil
		} else {
			fmt.Println("No recipes found.")
			return nil
		}
	}

	if c.Bool("json") {
		return outputRecipesAsJSON(allRecipes)
	}

	listRecipes(allRecipes)
	return nil
}

func outputRecipesAsJSON(recipes []Recipe) error {
	type recipeInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Category    string `json:"category,omitempty"`
		Author      string `json:"author,omitempty"`
	}

	result := make([]recipeInfo, len(recipes))
	for i, r := range recipes {
		result[i] = recipeInfo{
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
			Author:      r.Author,
		}
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func handleRunCommand(c *cli.Context, args []string, sourcePriority []string) error {
	debug := c.Bool("debug")
	var recipes []Recipe
	var remainingArgs []string

	recipeFilePath := c.String("recipe-file")
	if recipeFilePath != "" {
		config, err := loadConfig(recipeFilePath)
		if err != nil {
			return fmt.Errorf("failed to load recipe file %s: %w", recipeFilePath, err)
		}
		recipes = config.Recipes
		remainingArgs = args
	} else {
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified. Use shef ls to list available recipes")
		}

		var recipe *Recipe
		var err error

		recipe, err = findRecipeInSources(args[0], "", sourcePriority, false)
		if err == nil {
			remainingArgs = args[1:]
		} else if len(args) > 1 {
			recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, false)
			if err == nil {
				remainingArgs = args[2:]
			}
		}

		if err != nil {
			if len(args) > 1 {
				recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, true)
				if err == nil {
					remainingArgs = args[2:]
				}
			}

			if err != nil {
				recipe, err = findRecipeInSources(args[0], "", sourcePriority, true)
				if err == nil {
					remainingArgs = args[1:]
				}
			}
		}

		if err != nil {
			return err
		}

		recipes = []Recipe{*recipe}
	}

	input, vars := processRemainingArgs(remainingArgs)

	for _, recipe := range recipes {
		if debug {
			fmt.Printf("Running recipe: %s\n", recipe.Name)
			fmt.Printf("With input: %s\n", input)
			fmt.Printf("With vars: %v\n", vars)
			fmt.Printf("Description: %s\n\n", recipe.Description)
		}

		if err := executeRecipe(recipe, input, vars, debug); err != nil {
			return err
		}
	}

	return nil
}

func processRemainingArgs(args []string) (string, map[string]interface{}) {
	vars := make(map[string]interface{})
	var input string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if strings.HasPrefix(arg, "--") {
				arg = arg[2:] // Remove --
				if strings.Contains(arg, "=") {
					parts := strings.SplitN(arg, "=", 2)
					flagName := strings.ReplaceAll(parts[0], "-", "_")
					vars[flagName] = parts[1]
				} else {
					flagName := strings.ReplaceAll(arg, "-", "_")
					vars[flagName] = true
				}
			} else {
				arg = arg[1:] // Remove -
				if strings.Contains(arg, "=") {
					parts := strings.SplitN(arg, "=", 2)
					vars[parts[0]] = parts[1]
				} else {
					for _, c := range arg {
						vars[string(c)] = true
					}
				}
			}
		} else if input == "" {
			input = arg
		}
	}

	return input, vars
}

func findRecipeInSources(recipeName, category string, sourcePriority []string, fuzzyMatch bool) (*Recipe, error) {
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		recipes, _ := loadRecipes(sources, category)

		recipe, err := findRecipeByName(recipes, recipeName)
		if err == nil {
			return recipe, nil
		}

		if category != "" {
			combinedName := fmt.Sprintf("%s-%s", category, recipeName)
			recipe, err = findRecipeByName(recipes, combinedName)
			if err == nil {
				return recipe, nil
			}
		}
	}

	if fuzzyMatch {
		var allRecipes []Recipe
		seenRecipeNames := make(map[string]bool)

		for _, source := range sourcePriority {
			useLocal := source == "local"
			useUser := source == "user"
			usePublic := source == "public"

			sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
			recipes, _ := loadRecipes(sources, "")

			for _, recipe := range recipes {
				if !seenRecipeNames[recipe.Name] {
					allRecipes = append(allRecipes, recipe)
					seenRecipeNames[recipe.Name] = true
				}
			}
		}

		if len(allRecipes) > 0 {
			recipeNames := make([]string, 0, len(allRecipes))
			recipeMap := make(map[string]Recipe)

			for _, recipe := range allRecipes {
				recipeNames = append(recipeNames, recipe.Name)
				recipeMap[recipe.Name] = recipe
			}

			if match, found := fuzzyMatchRecipe(recipeName, recipeNames, recipeMap); found {
				return match, nil
			}
		}
	}

	return nil, fmt.Errorf("recipe not found: %s", recipeName)
}

func fuzzyMatchRecipe(recipeName string, recipeNames []string, recipeMap map[string]Recipe) (*Recipe, bool) {
	if len(recipeNames) == 0 {
		return nil, false
	}

	type nameDistance struct {
		name     string
		distance int
	}
	var matches []nameDistance

	for _, name := range recipeNames {
		distance := levenshtein.ComputeDistance(recipeName, name)
		matches = append(matches, nameDistance{name: name, distance: distance})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	if len(matches) > 0 {
		bestMatch := matches[0]
		recipe := recipeMap[bestMatch.name]

		var confirm bool
		var promptMessage string

		if recipe.Category != "" {
			promptMessage = fmt.Sprintf("Did you mean [%s] '%s'?", recipe.Category, bestMatch.name)
		} else {
			promptMessage = fmt.Sprintf("Did you mean '%s'?", bestMatch.name)
		}

		prompt := &survey.Confirm{
			Message: promptMessage,
			Default: true,
		}

		if err := survey.AskOne(prompt, &confirm); err == nil && confirm {
			return &recipe, true
		}
	}

	return nil, false
}

func handleWhichCommand(args []string, sourcePriority []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify a recipe name")
	}

	var category string
	var recipeName string

	if len(args) >= 2 {
		category = args[0]
		recipeName = args[1]
	} else {
		recipeName = args[0]
	}

	sourcePath, err := findRecipeSourceFile(recipeName, category, sourcePriority)
	if err != nil {
		return err
	}

	fmt.Println(sourcePath)
	return nil
}

func findRecipeSourceFile(recipeName, category string, sourcePriority []string) (string, error) {
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)

		for _, sourceFile := range sources {
			config, err := loadConfig(sourceFile)
			if err != nil {
				continue
			}

			for _, recipe := range config.Recipes {
				if recipe.Name == recipeName {
					return sourceFile, nil
				}

				if category != "" {
					combinedName := fmt.Sprintf("%s-%s", category, recipeName)
					if recipe.Name == combinedName {
						return sourceFile, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("recipe not found: %s", recipeName)
}
