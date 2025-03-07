package main

import (
	"bytes"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
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
)

// Define common template functions to be used across all templates
var templateFuncs = template.FuncMap{
	"split":      strings.Split,
	"join":       strings.Join,
	"trim":       strings.TrimSpace,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"contains":   strings.Contains,
	"replace":    strings.ReplaceAll,
	"filter":     filterLines,
	"grep":       grepLines,
	"cut":        cutFields,
	"exec":       execCommand,
	"atoi": func(s string) int {
		// Simple string to int without error handling
		var i int
		fmt.Sscanf(s, "%d", &i)
		return i
	},
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"mul": func(a, b int) int {
		return a * b
	},
}

// Config represents the top-level configuration
type Config struct {
	Recipes []Recipe `yaml:"recipes"`
}

// Recipe represents a workflow recipe
type Recipe struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Category    string      `yaml:"category,omitempty"`
	Author      string      `yaml:"author,omitempty"`
	Operations  []Operation `yaml:"operations"`
}

// Operation represents a single step in a recipe
type Operation struct {
	Name          string   `yaml:"name"`
	ID            string   `yaml:"id,omitempty"` // Unique identifier for referencing operation output
	Command       string   `yaml:"command"`
	ExecutionMode string   `yaml:"execution_mode,omitempty"` // "standard" [default], "interactive", or "stream"
	Silent        bool     `yaml:"silent,omitempty"`         // When true, output is not displayed but still stored
	Condition     string   `yaml:"condition,omitempty"`      // Conditional expression for if/else branching
	OnSuccess     string   `yaml:"on_success,omitempty"`     // Operation ID to execute on success
	OnFailure     string   `yaml:"on_failure,omitempty"`     // Operation ID to execute on failure
	Transform     string   `yaml:"transform,omitempty"`      // Template to transform output before storing
	Prompts       []Prompt `yaml:"prompts,omitempty"`
}

// Prompt represents an interactive user prompt
type Prompt struct {
	Name            string   `yaml:"name"`
	Type            string   `yaml:"type"` // "input", "select", "confirm"
	Message         string   `yaml:"message"`
	Default         string   `yaml:"default,omitempty"`
	Options         []string `yaml:"options,omitempty"`          // For static select prompts
	SourceOp        string   `yaml:"source_operation,omitempty"` // ID of operation to get dynamic options from
	SourceTransform string   `yaml:"source_transform,omitempty"` // Transform for dynamic options
}

// UserConfig represents the user configuration
type UserConfig struct {
	DefaultCategory string `yaml:"default_category"`
	Editor          string `yaml:"editor"`
	Debug           bool   `yaml:"debug"`
}

// ExecutionContext holds the state during recipe execution
type ExecutionContext struct {
	Data             string                 // Current data being passed through the pipeline
	Vars             map[string]interface{} // Variables from prompts and operations
	OperationOutputs map[string]string      // Outputs from operations with IDs
	OperationResults map[string]bool        // Success/failure status of operations
}

// getTemplateVars creates a combined map of variables for template execution
func (ctx *ExecutionContext) getTemplateVars() map[string]interface{} {
	vars := make(map[string]interface{})

	// Add all normal variables
	for k, v := range ctx.Vars {
		vars[k] = v
	}

	// Add operation outputs directly as variables
	for opID, output := range ctx.OperationOutputs {
		vars[opID] = output
	}

	// Add nested maps for backward compatibility
	vars["operationOutputs"] = ctx.OperationOutputs
	vars["operationResults"] = ctx.OperationResults

	return vars
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadUserConfig loads the user configuration
func LoadUserConfig() (*UserConfig, error) {
	// Default configuration
	config := &UserConfig{
		DefaultCategory: "default",
		Editor:          "vim",
		Debug:           false,
	}

	// Look for user config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userConfig := filepath.Join(homeDir, ".shef", "config.yaml")
		if _, err := os.Stat(userConfig); err == nil {
			data, err := ioutil.ReadFile(userConfig)
			if err == nil {
				yaml.Unmarshal(data, config)
			}
		}
	}

	// Look for project config
	projectConfig := ".shef/config.yaml"
	if _, err := os.Stat(projectConfig); err == nil {
		data, err := ioutil.ReadFile(projectConfig)
		if err == nil {
			yaml.Unmarshal(data, config)
		}
	}

	return config, nil
}

// LoadRecipes loads recipes from multiple sources
func LoadRecipes(sources []string, category string) ([]Recipe, error) {
	var allRecipes []Recipe

	for _, source := range sources {
		config, err := LoadConfig(source)
		if err != nil {
			fmt.Printf("Warning: Failed to load recipes from %s: %v\n", source, err)
			continue
		}

		// Filter by category if specified
		if category != "" {
			var filteredRecipes []Recipe
			for _, recipe := range config.Recipes {
				if recipe.Category == category {
					filteredRecipes = append(filteredRecipes, recipe)
				}
			}
			allRecipes = append(allRecipes, filteredRecipes...)
		} else {
			allRecipes = append(allRecipes, config.Recipes...)
		}
	}

	return allRecipes, nil
}

// UpdatePublicRecipes copies recipes from repo to user's ~/.shef/public
func UpdatePublicRecipes() error {
	// Check if we're in the repo directory (has recipes folder)
	if _, err := os.Stat("recipes"); os.IsNotExist(err) {
		return fmt.Errorf("recipes directory not found - please run this command from the shef repository root")
	}

	// Create user's public and personal recipe directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %v", err)
	}

	// Create the .shef base directory path
	shefDir := filepath.Join(homeDir, ".shef")

	// Create public directory
	publicDir := filepath.Join(shefDir, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		return fmt.Errorf("failed to create public recipes directory: %v", err)
	}

	// Create user directory
	userDir := filepath.Join(shefDir, "user")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("failed to create user recipes directory: %v", err)
	}

	// Copy recipes from repo to user's public directory
	fmt.Println("Copying public recipes...")
	err = copyDir("recipes", publicDir)
	if err != nil {
		return fmt.Errorf("failed to copy recipes: %v", err)
	}

	fmt.Printf("Public recipes copied to %s\n", publicDir)
	return nil
}

// Helper function to copy a directory recursively
func copyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination dir
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	dir, err := os.Open(src)
	if err != nil {
		return err
	}
	defer dir.Close()

	items, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	// Copy each item
	for _, item := range items {
		srcPath := filepath.Join(src, item.Name())
		dstPath := filepath.Join(dst, item.Name())

		if item.IsDir() {
			// Recursively copy subdirectory
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper function to copy a file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// FindRecipeSourcesByType finds recipe files in the specified locations
func FindRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	// Helper function to walk directories and find YAML files
	findYamlFiles := func(root string) ([]string, error) {
		var files []string
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip this file and continue
			}
			// Skip directories themselves
			if info.IsDir() {
				return nil
			}
			// Check if file has .yaml or .yml extension
			if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}
			return nil
		})
		return files, err
	}

	// Check project-local recipes
	if localDir {
		if _, err := os.Stat(".shef"); err == nil {
			localFiles, err := findYamlFiles(".shef")
			if err == nil {
				sources = append(sources, localFiles...)
			}
		}
	}

	// Check user's home directory recipes
	if userDir || publicRepo { // We include both when either is requested
		homeDir, err := os.UserHomeDir()
		if err == nil {
			userRoot := filepath.Join(homeDir, ".shef")
			if _, err := os.Stat(userRoot); err == nil {
				userFiles, err := findYamlFiles(userRoot)
				if err == nil {
					sources = append(sources, userFiles...)
				}
			}
		}
	}

	return sources, nil
}

// HandlePrompt processes a prompt and returns the user's response
func HandlePrompt(p Prompt, ctx *ExecutionContext) (interface{}, error) {
	// Get combined template variables including operation outputs as direct variables
	vars := ctx.getTemplateVars()

	// Process the message template with all variables
	msgTemplate, err := template.New("message").Funcs(templateFuncs).Parse(p.Message)
	if err != nil {
		return nil, err
	}

	var msgBuf bytes.Buffer
	if err := msgTemplate.Execute(&msgBuf, vars); err != nil {
		return nil, err
	}
	message := msgBuf.String()

	// Process default value if present
	var defaultValue string
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

	switch p.Type {
	case "input":
		var answer string
		prompt := &survey.Input{
			Message: message,
			Default: defaultValue,
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	case "select":
		var options []string

		// Handle dynamic options from previous operation output
		if p.SourceOp != "" {
			if output, exists := ctx.OperationOutputs[p.SourceOp]; exists {
				// Apply transformation if specified
				if p.SourceTransform != "" {
					transformedOutput, err := transformOutput(output, p.SourceTransform, ctx)
					if err != nil {
						return nil, fmt.Errorf("transformation failed: %v", err)
					}
					options = parseOptionsFromOutput(transformedOutput)
				} else {
					// Default parsing - split by newlines
					options = parseOptionsFromOutput(output)
				}

				if len(options) == 0 {
					return nil, fmt.Errorf("no options found from source operation %s", p.SourceOp)
				}
			} else {
				return nil, fmt.Errorf("source operation %s not found or has no output", p.SourceOp)
			}
		} else {
			// Use static options defined in YAML
			options = p.Options
		}

		if len(options) == 0 {
			return nil, fmt.Errorf("no options available for select prompt")
		}

		// If no default is provided, use the first option as default
		if defaultValue == "" && len(options) > 0 {
			defaultValue = options[0]
		}

		var answer string
		prompt := &survey.Select{
			Message: message,
			Options: options,
			Default: defaultValue,
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
		}
		if err := survey.AskOne(prompt, &answer); err != nil {
			return nil, err
		}
		return answer, nil

	default:
		return nil, fmt.Errorf("unknown prompt type: %s", p.Type)
	}
}

// transformOutput applies a transformation to an output string
func transformOutput(output, transform string, ctx *ExecutionContext) (string, error) {
	// Get all variables including operation outputs
	vars := ctx.getTemplateVars()

	// Add input as a special variable
	vars["input"] = output

	// Parse and execute the template with all functions
	tmpl, err := template.New("transform").Funcs(templateFuncs).Parse(transform)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// execCommand executes a command and returns its output
// Only used within transformations
func execCommand(cmd string) string {
	command := exec.Command("sh", "-c", cmd)
	output, err := command.Output()
	if err != nil {
		return ""
	}

	return string(output)
}

// Helper functions for transformations
func filterLines(input, pattern string) string {
	var result []string
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func grepLines(input, pattern string) string {
	return filterLines(input, pattern)
}

func cutFields(input string, delimiter string, field int) string {
	var result []string
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		fields := strings.Split(line, delimiter)
		if field < len(fields) {
			result = append(result, strings.TrimSpace(fields[field]))
		}
	}
	return strings.Join(result, "\n")
}

// parseOptionsFromOutput converts command output to a list of options
func parseOptionsFromOutput(output string) []string {
	// Split by newlines and filter out empty lines
	lines := strings.Split(output, "\n")
	var options []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			options = append(options, line)
		}
	}

	return options
}

// ExecuteCommand runs a shell command with the given input and execution mode
// Execution modes: "standard", "interactive", "stream"
func ExecuteCommand(cmdStr string, input string, executionMode string) (string, error) {
	// Default to standard mode if not specified
	if executionMode == "" {
		executionMode = "standard"
	}

	// For standard commands that capture output
	if executionMode == "standard" {
		// Always use a shell to execute commands for consistent behavior
		cmd := exec.Command("sh", "-c", cmdStr)

		// Set up input if provided
		if input != "" {
			cmd.Stdin = strings.NewReader(input)
		}

		// Capture output
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Run the command
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("command failed: %v\nStderr: %s", err, stderr.String())
		}

		return strings.TrimSpace(stdout.String()), nil
	}

	// Create command
	cmd := exec.Command("sh", "-c", cmdStr)

	// Connect directly to the terminal
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// For streaming commands, we need to properly handle signals
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// This ensures the process receives signals properly
		Setpgid: false,
	}

	// Start the command
	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	// Handle graceful termination on SIGINT (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create a channel for command completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for either command completion or interrupt
	select {
	case <-sigChan:
		// User pressed Ctrl+C, try to terminate gracefully
		cmd.Process.Signal(os.Interrupt)

		// Give it a moment to clean up
		select {
		case <-done:
			// Command exited after signal
		case <-time.After(2 * time.Second):
			// Force kill if it doesn't exit quickly
			cmd.Process.Kill()
		}
		return "", nil

	case err := <-done:
		return "", err
	}
}

// RenderTemplate renders a command template with variables
func RenderTemplate(tmplStr string, vars map[string]interface{}) (string, error) {
	tmpl, err := template.New("command").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// parseNumericComparison evaluates conditions with numeric operators (>, <, >=, <=), supporting variables and both integer and float values
func parseNumericComparison(condition string, ctx *ExecutionContext) (bool, error) {
	// Check for numeric comparisons (>, <, >=, <=)
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

	leftStr := strings.TrimSpace(parts[0])
	rightStr := strings.TrimSpace(parts[1])

	// Handle variable references and template expressions on the left side
	if strings.Contains(leftStr, "{{") && strings.Contains(leftStr, "}}") {
		// This is a template expression, process it
		rendered, err := RenderTemplate(leftStr, ctx.getTemplateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render left side template: %v", err)
		}
		leftStr = rendered
	} else if strings.HasPrefix(leftStr, "$") || strings.HasPrefix(leftStr, ".") {
		// This is a variable reference
		varName := leftStr
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:] // Remove $ prefix
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:] // Remove . prefix
		}

		// Check in variables and operation outputs
		if value, exists := ctx.Vars[varName]; exists {
			leftStr = fmt.Sprintf("%v", value)
		} else if value, exists := ctx.OperationOutputs[varName]; exists {
			leftStr = value
		} else {
			return false, fmt.Errorf("variable %s not found", varName)
		}
	}

	// Handle variable references and template expressions on the right side
	if strings.Contains(rightStr, "{{") && strings.Contains(rightStr, "}}") {
		// This is a template expression, process it
		rendered, err := RenderTemplate(rightStr, ctx.getTemplateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render right side template: %v", err)
		}
		rightStr = rendered
	} else if strings.HasPrefix(rightStr, "$") || strings.HasPrefix(rightStr, ".") {
		// This is a variable reference
		varName := rightStr
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:] // Remove $ prefix
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:] // Remove . prefix
		}

		// Check in variables and operation outputs
		if value, exists := ctx.Vars[varName]; exists {
			rightStr = fmt.Sprintf("%v", value)
		} else if value, exists := ctx.OperationOutputs[varName]; exists {
			rightStr = value
		} else {
			return false, fmt.Errorf("variable %s not found", varName)
		}
	}

	// Convert to integers for comparison
	leftVal, leftErr := strconv.Atoi(strings.TrimSpace(leftStr))
	rightVal, rightErr := strconv.Atoi(strings.TrimSpace(rightStr))

	if leftErr != nil || rightErr != nil {
		// Try float conversion if integer conversion fails
		leftFloat, leftErr := strconv.ParseFloat(strings.TrimSpace(leftStr), 64)
		rightFloat, rightErr := strconv.ParseFloat(strings.TrimSpace(rightStr), 64)

		if leftErr != nil || rightErr != nil {
			return false, fmt.Errorf("numeric comparison requires numeric values, got '%s' and '%s'", leftStr, rightStr)
		}

		// Perform float comparison
		switch op {
		case ">":
			return leftFloat > rightFloat, nil
		case "<":
			return leftFloat < rightFloat, nil
		case ">=":
			return leftFloat >= rightFloat, nil
		case "<=":
			return leftFloat <= rightFloat, nil
		}
	}

	// Perform integer comparison
	switch op {
	case ">":
		return leftVal > rightVal, nil
	case "<":
		return leftVal < rightVal, nil
	case ">=":
		return leftVal >= rightVal, nil
	case "<=":
		return leftVal <= rightVal, nil
	}

	return false, fmt.Errorf("unknown operator: %s", op)
}

// EvaluateCondition evaluates a condition expression against the context
func EvaluateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	if condition == "" {
		return true, nil // Empty condition always evaluates to true
	}

	// Process templates in condition first if present
	if strings.Contains(condition, "{{") && strings.Contains(condition, "}}") {
		rendered, err := RenderTemplate(condition, ctx.getTemplateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render condition template: %v", err)
		}
		// Recursive call to evaluate the rendered condition
		return EvaluateCondition(rendered, ctx)
	}

	// Parse the condition
	condition = strings.TrimSpace(condition)

	// Handle AND (&&) conditions
	if strings.Contains(condition, "&&") {
		parts := strings.Split(condition, "&&")
		for _, part := range parts {
			result, err := EvaluateCondition(part, ctx)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil // Short-circuit on first false condition
			}
		}
		return true, nil
	}

	// Handle OR (||) conditions
	if strings.Contains(condition, "||") {
		parts := strings.Split(condition, "||")
		for _, part := range parts {
			result, err := EvaluateCondition(part, ctx)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil // Short-circuit on first true condition
			}
		}
		return false, nil
	}

	// Handle NOT (!) conditions
	if strings.HasPrefix(condition, "!") {
		subCondition := strings.TrimSpace(condition[1:])
		result, err := EvaluateCondition(subCondition, ctx)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Try to parse as a numeric comparison
	numericResult, numericErr := parseNumericComparison(condition, ctx)
	if numericErr == nil {
		return numericResult, nil
	}

	// Check for operation result conditions
	if strings.Contains(condition, ".success") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "success" {
			opID := strings.TrimSpace(parts[0])
			if result, exists := ctx.OperationResults[opID]; exists {
				return result, nil
			}
			return false, fmt.Errorf("operation %s result not found", opID)
		}
	}

	if strings.Contains(condition, ".failure") {
		parts := strings.Split(condition, ".")
		if len(parts) == 2 && parts[1] == "failure" {
			opID := strings.TrimSpace(parts[0])
			if result, exists := ctx.OperationResults[opID]; exists {
				return !result, nil // .failure is the opposite of .success
			}
			return false, fmt.Errorf("operation %s result not found", opID)
		}
	}

	// Check for variable comparisons
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid condition format: %s", condition)
		}

		varName := strings.TrimSpace(parts[0])
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:] // Remove $ prefix
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:] // Remove . prefix
		}

		expectedValue := strings.TrimSpace(parts[1])
		// Remove quotes if present
		expectedValue = strings.Trim(expectedValue, "\"'")

		// First check in regular variables
		if value, exists := ctx.Vars[varName]; exists {
			// Convert value to string for comparison
			strValue := fmt.Sprintf("%v", value)
			return strValue == expectedValue, nil
		}

		// Then check in operation outputs
		if value, exists := ctx.OperationOutputs[varName]; exists {
			return value == expectedValue, nil
		}

		return false, fmt.Errorf("variable %s not found", varName)
	}

	if strings.Contains(condition, "!=") {
		parts := strings.Split(condition, "!=")
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid condition format: %s", condition)
		}

		varName := strings.TrimSpace(parts[0])
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:] // Remove $ prefix
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:] // Remove . prefix
		}

		expectedValue := strings.TrimSpace(parts[1])
		// Remove quotes if present
		expectedValue = strings.Trim(expectedValue, "\"'")

		// First check in regular variables
		if value, exists := ctx.Vars[varName]; exists {
			// Convert value to string for comparison
			strValue := fmt.Sprintf("%v", value)
			return strValue != expectedValue, nil
		}

		// Then check in operation outputs
		if value, exists := ctx.OperationOutputs[varName]; exists {
			return value != expectedValue, nil
		}

		return false, fmt.Errorf("variable %s not found", varName)
	}

	// Handle direct boolean values
	if condition == "true" {
		return true, nil
	}
	if condition == "false" {
		return false, nil
	}

	return false, fmt.Errorf("unsupported condition format: %s", condition)
}

// ExecuteRecipe runs a recipe by executing each operation in sequence
func ExecuteRecipe(recipe Recipe, debug bool) error {
	ctx := ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
	}

	// Build an operation map for quick lookups
	opMap := make(map[string]Operation)
	for _, op := range recipe.Operations {
		if op.ID != "" {
			opMap[op.ID] = op
		}
	}

	// Create a set to track executed operations to prevent infinite loops
	executedOps := make(map[string]bool)

	// Define a recursive function to execute operations with branching
	var executeOp func(op Operation, depth int) error

	executeOp = func(op Operation, depth int) error {
		// Check for infinite loops
		if depth > 50 {
			return fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		// Track this operation as executed
		if op.ID != "" {
			executedOps[op.ID] = true
		}

		// Evaluate condition if present
		if op.Condition != "" {
			if debug {
				fmt.Printf("Evaluating condition: %s\n", op.Condition)
			}
			result, err := EvaluateCondition(op.Condition, &ctx)
			if err != nil {
				return fmt.Errorf("condition evaluation failed: %v", err)
			}

			if !result {
				if debug {
					fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
				}
				// Mark as skipped in operation results if it has an ID
				if op.ID != "" {
					ctx.OperationResults[op.ID] = false
				}
				return nil
			}
		}

		// Handle prompts
		for _, prompt := range op.Prompts {
			value, err := HandlePrompt(prompt, &ctx)
			if err != nil {
				return err
			}
			ctx.Vars[prompt.Name] = value
		}

		// Render command template with combined variables
		cmd, err := RenderTemplate(op.Command, ctx.getTemplateVars())
		if err != nil {
			return fmt.Errorf("failed to render command template: %v", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		// Execute the command
		output, err := ExecuteCommand(cmd, ctx.Data, op.ExecutionMode)
		operationSuccess := err == nil

		if err != nil {
			fmt.Printf("Warning: command execution had errors: %v\n", err)

			// Ask if user wants to continue
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

		// Apply transformation if specified
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

		// Store operation result and output
		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
			ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
		}

		// Update context with output
		ctx.Data = output

		// Only print output if there's something to show
		if output != "" && !op.Silent {
			fmt.Println(output)
		}

		// Handle success/failure branching
		if operationSuccess && op.OnSuccess != "" {
			// Check if we've already executed this target to prevent loops
			if executedOps[op.OnSuccess] {
				return fmt.Errorf("operation branching would create a loop with %s", op.OnSuccess)
			}

			// Find and execute the success branch
			successOp, exists := opMap[op.OnSuccess]
			if !exists {
				return fmt.Errorf("on_success operation %s not found", op.OnSuccess)
			}

			if debug {
				fmt.Printf("Branching to success operation: %s\n", successOp.Name)
			}
			return executeOp(successOp, depth+1)
		} else if !operationSuccess && op.OnFailure != "" {
			// Check if we've already executed this target to prevent loops
			if executedOps[op.OnFailure] {
				return fmt.Errorf("operation branching would create a loop with %s", op.OnFailure)
			}

			// Find and execute the failure branch
			failureOp, exists := opMap[op.OnFailure]
			if !exists {
				return fmt.Errorf("on_failure operation %s not found", op.OnFailure)
			}

			if debug {
				fmt.Printf("Branching to failure operation: %s\n", failureOp.Name)
			}
			return executeOp(failureOp, depth+1)
		}

		return nil
	}

	// Execute operations in sequence, respecting branching
	for _, op := range recipe.Operations {
		// Skip operations that were already executed via branching
		if op.ID != "" && executedOps[op.ID] {
			continue
		}

		err := executeOp(op, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

// truncateOutput limits output lines for display purposes
func truncateOutput(output string, maxLines int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + fmt.Sprintf("\n... (and %d more lines)", len(lines)-maxLines)
}

// ListRecipes displays all available recipes
func ListRecipes(recipes []Recipe) {
	if len(recipes) == 0 {
		fmt.Println("No recipes found.")
		return
	}

	fmt.Println("Available recipes:")

	// Group recipes by category
	categories := make(map[string][]Recipe)
	for _, recipe := range recipes {
		cat := recipe.Category
		if cat == "" {
			cat = "uncategorized"
		}
		categories[cat] = append(categories[cat], recipe)
	}

	// Get sorted category names
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	// Print recipes by category
	for _, category := range categoryNames {
		catRecipes := categories[category]

		// Sort recipes by name within each category
		sort.Slice(catRecipes, func(i, j int) bool {
			return catRecipes[i].Name < catRecipes[j].Name
		})

		fmt.Printf("\n[%s]\n", category)
		for i, recipe := range catRecipes {
			fmt.Printf("%d. %s: %s\n", i+1, recipe.Name, recipe.Description)
		}
	}
}

// FindRecipeByName finds a recipe by its name
func FindRecipeByName(recipes []Recipe, name string) (*Recipe, error) {
	for _, recipe := range recipes {
		if recipe.Name == name {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

func main() {
	// Load user configuration
	userConfig, _ := LoadUserConfig()

	// Set up CLI app with standard conventions
	app := &cli.App{
		Name:    "shef",
		Usage:   "Run shell recipes",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug output",
				Value:   userConfig.Debug,
			},
			&cli.BoolFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "List available recipes",
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
		// Default action handles both listing and running recipes
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()

			// Determine source priorities based on flags
			sourcePriority := []string{"local", "user", "public"} // Default priority

			if c.Bool("local") {
				sourcePriority = []string{"local", "user", "public"}
			} else if c.Bool("user") {
				sourcePriority = []string{"user", "local", "public"}
			} else if c.Bool("public") {
				sourcePriority = []string{"public", "local", "user"}
			}

			// If list flag is set, show recipe list
			if c.Bool("list") {
				// Determine which category to filter by
				category := c.String("category")
				if category == "" && len(args) >= 1 {
					category = args[0]
				}

				// Find and load recipes in priority order
				var allRecipes []Recipe
				for _, source := range sourcePriority {
					useLocal := source == "local"
					useUser := source == "user"
					usePublic := source == "public"

					sources, _ := FindRecipeSourcesByType(useLocal, useUser, usePublic)
					recipes, _ := LoadRecipes(sources, category)

					// Avoid duplicates by name
					recipeMap := make(map[string]bool)
					for _, r := range allRecipes {
						recipeMap[r.Name] = true
					}

					for _, r := range recipes {
						if !recipeMap[r.Name] {
							allRecipes = append(allRecipes, r)
							recipeMap[r.Name] = true
						}
					}
				}

				ListRecipes(allRecipes)
				return nil
			}

			// Not listing, so must be running a recipe
			if len(args) == 0 {
				return fmt.Errorf("no recipe specified. Use shef -l to list available recipes")
			}

			// Parse category and recipe name
			var category, recipeName string

			if len(args) == 1 {
				// Just the recipe name
				recipeName = args[0]
				category = c.String("category") // May be empty
			} else {
				// Category and recipe name
				category = args[0]
				recipeName = args[1]
			}

			// Try to find the recipe in each source according to priority
			var recipe *Recipe
			var recipeErr error

			for _, source := range sourcePriority {
				useLocal := source == "local"
				useUser := source == "user"
				usePublic := source == "public"

				sources, _ := FindRecipeSourcesByType(useLocal, useUser, usePublic)
				recipes, _ := LoadRecipes(sources, category)

				recipe, recipeErr = FindRecipeByName(recipes, recipeName)
				if recipeErr == nil {
					break // Found it!
				}

				// If category provided, also try with combined name
				if category != "" {
					combinedName := fmt.Sprintf("%s-%s", category, recipeName)
					recipe, recipeErr = FindRecipeByName(recipes, combinedName)
					if recipeErr == nil {
						break // Found it!
					}
				}
			}

			if recipeErr != nil {
				return fmt.Errorf("recipe not found: %s", recipeName)
			}

			fmt.Printf("Running recipe: %s\n", recipe.Name)
			fmt.Printf("Description: %s\n\n", recipe.Description)

			return ExecuteRecipe(*recipe, c.Bool("debug"))
		},
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: "Update public recipes",
				Action: func(c *cli.Context) error {
					return UpdatePublicRecipes()
				},
			},
		},
	}

	// Run the application
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
