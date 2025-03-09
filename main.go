package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
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
	Silent        bool        `yaml:"silent,omitempty"`
	Condition     string      `yaml:"condition,omitempty"`
	OnSuccess     string      `yaml:"on_success,omitempty"`
	OnFailure     string      `yaml:"on_failure,omitempty"`
	Transform     string      `yaml:"transform,omitempty"`
	Prompts       []Prompt    `yaml:"prompts,omitempty"`
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

	return buf.String(), nil
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
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}
			return nil
		})
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
		}
	}

	return sources, nil
}

func syncPublicRecipes() error {
	if _, err := os.Stat("recipes"); os.IsNotExist(err) {
		return fmt.Errorf("recipes directory not found - please run this command from the Shef repository root")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	shefDir := filepath.Join(homeDir, ".shef")
	publicDir := filepath.Join(shefDir, "public")
	userDir := filepath.Join(shefDir, "user")

	for _, dir := range []string{publicDir, userDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	fmt.Println("Copying public recipes...")
	if err := copyDir("recipes", publicDir); err != nil {
		return fmt.Errorf("failed to copy recipes: %w", err)
	}

	fmt.Printf("Public recipes copied to %s\n", publicDir)
	return nil
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	dir, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {

		}
	}(dir)

	items, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	for _, item := range items {
		srcPath := filepath.Join(src, item.Name())
		dstPath := filepath.Join(dst, item.Name())

		if item.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func(srcFile *os.File) {
		err := srcFile.Close()
		if err != nil {

		}
	}(srcFile)

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(dstFile *os.File) {
		err := dstFile.Close()
		if err != nil {

		}
	}(dstFile)

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
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

func executeCommand(cmdStr string, input string, executionMode string) (string, error) {
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

		return strings.TrimSpace(stdout.String()), nil
	}

	// Execute command with interactive or streaming mode
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: false,
	}

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
			return false, fmt.Errorf("operation %s result not found", opID)
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
			return false, fmt.Errorf("operation %s result not found", opID)
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

	varName := strings.TrimSpace(parts[0])
	if strings.HasPrefix(varName, "$") {
		varName = varName[1:]
	}
	if strings.HasPrefix(varName, ".") {
		varName = varName[1:]
	}

	expectedValue := strings.TrimSpace(parts[1])
	expectedValue = strings.Trim(expectedValue, "\"'")

	if value, exists := ctx.Vars[varName]; exists {
		strValue := fmt.Sprintf("%v", value)
		if op == "==" {
			return strValue == expectedValue, nil
		}
		return strValue != expectedValue, nil
	}

	if value, exists := ctx.OperationOutputs[varName]; exists {
		if op == "==" {
			return value == expectedValue, nil
		}
		return value != expectedValue, nil
	}

	return false, fmt.Errorf("variable %s not found", varName)
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
		return "", fmt.Errorf("variable %s not found", varName)
	}

	return value, nil
}

func executeRecipe(recipe Recipe, debug bool) error {
	ctx := ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
	}

	opMap := make(map[string]Operation)
	registerOperations(recipe.Operations, opMap)

	if debug {
		fmt.Println("Registered operations:")
		for id := range opMap {
			fmt.Printf("  - %s\n", id)
		}
	}

	var executeOp func(op Operation, depth int) error
	executeOp = func(op Operation, depth int) error {
		if depth > 50 {
			return fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		if op.ControlFlow != nil {
			flowMap, ok := op.ControlFlow.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid control_flow structure")
			}

			typeVal, ok := flowMap["type"].(string)
			if !ok {
				return fmt.Errorf("control_flow requires a 'type' field")
			}

			switch typeVal {

			case "foreach":
				forEach, err := op.GetForEachFlow()
				if err != nil {
					return err
				}
				return ExecuteForEach(op, forEach, &ctx, depth, executeOp, debug)

			case "while":
				whileFlow, err := op.GetWhileFlow()
				if err != nil {
					return err
				}
				return ExecuteWhile(op, whileFlow, &ctx, depth, executeOp, debug)

			case "for":
				forFlow, err := op.GetForFlow()
				if err != nil {
					return err
				}
				return ExecuteFor(op, forFlow, &ctx, depth, executeOp, debug)

			default:
				return fmt.Errorf("unknown control_flow type: %s", typeVal)
			}
		}

		if op.Condition != "" {
			if debug {
				fmt.Printf("Evaluating condition: %s\n", op.Condition)
			}
			result, err := evaluateCondition(op.Condition, &ctx)
			if err != nil {
				return fmt.Errorf("condition evaluation failed: %w", err)
			}

			if !result {
				if debug {
					fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
				}
				if op.ID != "" {
					ctx.OperationResults[op.ID] = false
				}

				if op.OnFailure != "" {
					nextOp, exists := opMap[op.OnFailure]
					if !exists {
						return fmt.Errorf("on_failure operation %s not found", op.OnFailure)
					}
					return executeOp(nextOp, depth+1)
				}

				return nil
			}
		}

		for _, prompt := range op.Prompts {
			value, err := handlePrompt(prompt, &ctx)
			if err != nil {
				return err
			}
			ctx.Vars[prompt.Name] = value
		}

		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return fmt.Errorf("failed to render command template: %w", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		output, err := executeCommand(cmd, ctx.Data, op.ExecutionMode)
		operationSuccess := err == nil

		if err != nil {
			fmt.Printf("Warning: command execution had errors: %v\n", err)

			var continueExecution bool
			prompt := &survey.Confirm{
				Message: "Command had errors. Continue with recipe execution?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &continueExecution); err != nil {
				return err
			}

			if !continueExecution {
				return fmt.Errorf("recipe execution aborted by user after command error")
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

		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
			ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
		}

		ctx.Data = output

		if output != "" && !op.Silent {
			fmt.Println(output)
		}

		if op.OnSuccess != "" && ctx.OperationResults[op.ID] {
			nextOp, exists := opMap[op.OnSuccess]
			if !exists {
				return fmt.Errorf("on_success operation %s not found", op.OnSuccess)
			}
			return executeOp(nextOp, depth+1)
		} else if op.OnFailure != "" && !ctx.OperationResults[op.ID] {
			nextOp, exists := opMap[op.OnFailure]
			if !exists {
				return fmt.Errorf("on_failure operation %s not found", op.OnFailure)
			}
			return executeOp(nextOp, depth+1)
		}

		return nil
	}

	for i, op := range recipe.Operations {
		if debug {
			fmt.Printf("Executing operation %d: %s\n", i+1, op.Name)
		}

		if err := executeOp(op, 0); err != nil {
			return err
		}

		if op.OnSuccess != "" || op.OnFailure != "" {
			break
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

		fmt.Printf("\n[%s]\n", category)
		for i, recipe := range catRecipes {
			fmt.Printf("%d. %s: %s\n", i+1, recipe.Name, recipe.Description)
		}
	}
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
		Version: "0.1.0",
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
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
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
				Name:  "sync",
				Usage: "Sync public recipes locally",
				Action: func(c *cli.Context) error {
					return syncPublicRecipes()
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
	if len(args) == 0 {
		return fmt.Errorf("no recipe specified. Use shef -l to list available recipes")
	}

	var category, recipeName string
	if len(args) == 1 {
		recipeName = args[0]
		category = c.String("category")
	} else {
		category = args[0]
		recipeName = args[1]
	}

	recipe, err := findRecipeInSources(recipeName, category, sourcePriority)
	if err != nil {
		return err
	}

	debug := c.Bool("debug")

	if debug {
		fmt.Printf("Running recipe: %s\n", recipe.Name)
		fmt.Printf("Description: %s\n\n", recipe.Description)
	}

	return executeRecipe(*recipe, debug)
}

func findRecipeInSources(recipeName, category string, sourcePriority []string) (*Recipe, error) {
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

	return nil, fmt.Errorf("recipe not found: %s", recipeName)
}
