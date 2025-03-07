package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

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
	Name      string   `yaml:"name"`
	ID        string   `yaml:"id,omitempty"` // Unique identifier for referencing operation output
	Command   string   `yaml:"command"`
	Condition string   `yaml:"condition,omitempty"`  // Conditional expression for if/else branching
	OnSuccess string   `yaml:"on_success,omitempty"` // Operation ID to execute on success
	OnFailure string   `yaml:"on_failure,omitempty"` // Operation ID to execute on failure
	Transform string   `yaml:"transform,omitempty"`  // Template to transform output before storing
	Prompts   []Prompt `yaml:"prompts,omitempty"`
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
	PublicRepo      string `yaml:"public_repo"`
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
		PublicRepo:      "https://github.com/eduardoagarcia/shef",
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

// UpdatePublicRecipes downloads the latest recipes from the public repository
func UpdatePublicRecipes(repoURL string) error {
	// Create temp directory if it doesn't exist
	publicDir := filepath.Join(os.TempDir(), "shef-public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		return err
	}

	fmt.Println("Updating public recipes from repository...")

	// For GitHub repos, try using HTTPS without auth for public repos
	// Convert SSH URL to HTTPS if needed
	httpsURL := repoURL
	if strings.HasPrefix(repoURL, "git@github.com:") {
		// Convert from git@github.com:user/repo.git to https://github.com/user/repo.git
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		httpsURL = "https://github.com/" + path
	}

	// Check if the directory already contains a git repository
	_, err := os.Stat(filepath.Join(publicDir, ".git"))
	isNewClone := os.IsNotExist(err)

	var cmd *exec.Cmd

	if isNewClone {
		// If directory is empty, try to do a fresh clone
		fmt.Println("Cloning repository...")

		// First attempt: shallow clone with HTTPS
		cmd = exec.Command("git", "clone", "--depth=1", httpsURL, publicDir)
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0", // Disable username/password prompt
			"GIT_ASKPASS=echo",      // Force echo as the askpass program
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Warning: Could not clone repository using HTTPS: %v\n", err)

			// Clean up failed attempt
			os.RemoveAll(publicDir)
			if err := os.MkdirAll(publicDir, 0755); err != nil {
				return err
			}

			// Second attempt: Try to download just the recipes folder as a ZIP
			// This works for GitHub repositories without authentication
			if strings.Contains(httpsURL, "github.com") {
				fmt.Println("Attempting to download recipes directly...")

				// Extract user/repo from URL
				parts := strings.Split(httpsURL, "github.com/")
				if len(parts) == 2 {
					repoPath := strings.TrimSuffix(parts[1], ".git")
					zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", repoPath)

					// Download the ZIP file
					zipPath := filepath.Join(os.TempDir(), "shef-repo.zip")
					if err := downloadFile(zipURL, zipPath); err != nil {
						return fmt.Errorf("failed to download repository: %v", err)
					}

					// Extract the recipes directory
					if err := extractRecipesFromZip(zipPath, publicDir, repoPath); err != nil {
						return fmt.Errorf("failed to extract recipes: %v", err)
					}

					fmt.Println("Recipe repository downloaded successfully")
					return nil
				}
			}

			return fmt.Errorf("failed to clone repository: %v\nOutput: %s", err, string(output))
		}
	} else {
		// Pull latest changes if repository already exists
		fmt.Println("Pulling latest changes...")
		cmd = exec.Command("git", "-C", publicDir, "pull", "origin", "main")
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0", // Disable username/password prompt
			"GIT_ASKPASS=echo",      // Force echo as the askpass program
		)

		_, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Warning: Could not pull latest changes: %v\n", err)

			// Attempt to re-clone
			os.RemoveAll(publicDir)
			return UpdatePublicRecipes(repoURL) // Recursive call to try clone instead
		}
	}

	// Verify that the recipes directory exists
	recipesDir := filepath.Join(publicDir, "recipes")
	if _, err := os.Stat(recipesDir); os.IsNotExist(err) {
		return fmt.Errorf("recipes directory not found in the repository")
	}

	fmt.Println("Recipe repository updated successfully")
	return nil
}

// Helper function to download a file from a URL
func downloadFile(url string, filepath string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Helper function to extract recipes from a zip file
func extractRecipesFromZip(zipPath, destDir, repoPath string) error {
	// Open the zip file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Get repository name
	repoParts := strings.Split(repoPath, "/")
	repoName := ""
	if len(repoParts) > 0 {
		repoName = repoParts[len(repoParts)-1]
	}

	// Look for the recipes directory
	recipesPrefix := fmt.Sprintf("%s-main/recipes/", repoName)

	// Create recipes directory
	recipesDir := filepath.Join(destDir, "recipes")
	if err := os.MkdirAll(recipesDir, 0755); err != nil {
		return err
	}

	// Extract files from recipes directory
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, recipesPrefix) {
			// Extract file name from path
			fileName := strings.TrimPrefix(f.Name, recipesPrefix)
			if fileName == "" {
				// Skip the directory itself
				continue
			}

			// Create output file path
			fpath := filepath.Join(recipesDir, fileName)

			// If it's a directory, create it
			if f.FileInfo().IsDir() {
				os.MkdirAll(fpath, 0755)
				continue
			}

			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
				return err
			}

			// Open the file in the zip
			rc, err := f.Open()
			if err != nil {
				return err
			}

			// Create the file on disk
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				rc.Close()
				return err
			}

			// Copy contents
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// FindRecipeSourcesByType finds recipe files with more granular control over source types
func FindRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	// Check project-local recipes
	if localDir {
		localFiles, err := filepath.Glob(".shef/*.yaml")
		if err == nil {
			sources = append(sources, localFiles...)
		}
	}

	// Check user recipes in ~/.shef
	if userDir {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			userFiles, err := filepath.Glob(filepath.Join(homeDir, ".shef", "*.yaml"))
			if err == nil {
				sources = append(sources, userFiles...)
			}
		}
	}

	// Check public recipes - updated to look in the recipes subdirectory
	if publicRepo {
		publicDir := filepath.Join(os.TempDir(), "shef-public")
		recipesDir := filepath.Join(publicDir, "recipes")
		if _, err := os.Stat(recipesDir); err == nil {
			publicFiles, err := filepath.Glob(filepath.Join(recipesDir, "*.yaml"))
			if err == nil {
				sources = append(sources, publicFiles...)
			}
		}
	}

	return sources, nil
}

// HandlePrompt processes a prompt and returns the user's response
func HandlePrompt(p Prompt, ctx *ExecutionContext) (interface{}, error) {
	// Process the message and default value through templates
	msgTemplate, err := template.New("message").Parse(p.Message)
	if err != nil {
		return nil, err
	}

	// Create a combined data map with context variables and operation outputs
	data := make(map[string]interface{})
	for k, v := range ctx.Vars {
		data[k] = v
	}
	data["operationOutputs"] = ctx.OperationOutputs
	data["operationResults"] = ctx.OperationResults

	var msgBuf bytes.Buffer
	if err := msgTemplate.Execute(&msgBuf, data); err != nil {
		return nil, err
	}
	message := msgBuf.String()

	// Process default value if present
	var defaultValue string
	if p.Default != "" {
		defaultTemplate, err := template.New("default").Parse(p.Default)
		if err != nil {
			return nil, err
		}

		var defaultBuf bytes.Buffer
		if err := defaultTemplate.Execute(&defaultBuf, data); err != nil {
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
	// Create a map with the output and context variables
	data := make(map[string]interface{})
	data["input"] = output
	for k, v := range ctx.Vars {
		data[k] = v
	}
	data["operationOutputs"] = ctx.OperationOutputs
	data["operationResults"] = ctx.OperationResults

	// Parse and execute the template
	tmpl, err := template.New("transform").Funcs(template.FuncMap{
		"split":      strings.Split,
		"join":       strings.Join,
		"trim":       strings.TrimSpace,
		"filter":     filterLines,
		"grep":       grepLines,
		"cut":        cutFields,
		"exec":       execCommand,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"contains":   strings.Contains,
		"replace":    strings.ReplaceAll,
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
	}).Parse(transform)

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// execCommand executes a command and returns its output
// Only used within transformations
func execCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	command := exec.Command(parts[0], parts[1:]...)
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

// ExecuteCommand runs a shell command with the given input
func ExecuteCommand(cmdStr string, input string) (string, error) {
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

	return stdout.String(), nil
}

// RenderTemplate renders a command template with variables
func RenderTemplate(tmplStr string, vars map[string]interface{}) (string, error) {
	tmpl, err := template.New("command").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// EvaluateCondition evaluates a condition expression against the context
func EvaluateCondition(condition string, ctx *ExecutionContext) (bool, error) {
	if condition == "" {
		return true, nil // Empty condition always evaluates to true
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

		expectedValue := strings.TrimSpace(parts[1])
		// Remove quotes if present
		expectedValue = strings.Trim(expectedValue, "\"'")

		if value, exists := ctx.Vars[varName]; exists {
			// Convert value to string for comparison
			strValue := fmt.Sprintf("%v", value)
			return strValue == expectedValue, nil
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

		expectedValue := strings.TrimSpace(parts[1])
		// Remove quotes if present
		expectedValue = strings.Trim(expectedValue, "\"'")

		if value, exists := ctx.Vars[varName]; exists {
			// Convert value to string for comparison
			strValue := fmt.Sprintf("%v", value)
			return strValue != expectedValue, nil
		}

		return false, fmt.Errorf("variable %s not found", varName)
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

		// Render command template
		cmd, err := RenderTemplate(op.Command, ctx.Vars)
		if err != nil {
			return fmt.Errorf("failed to render command template: %v", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		// Execute the command
		output, err := ExecuteCommand(cmd, ctx.Data)
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
			ctx.OperationOutputs[op.ID] = output
		}

		// Update context with output
		ctx.Data = output

		// Only print output if there's something to show
		if output != "" {
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

	// Print recipes by category
	for category, catRecipes := range categories {
		fmt.Printf("\n[%s]\n", category)
		for i, recipe := range catRecipes {
			fmt.Printf("%d. %s - %s\n", i+1, recipe.Name, recipe.Description)
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
					return UpdatePublicRecipes(userConfig.PublicRepo)
				},
			},
		},
	}

	// Run the application
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
