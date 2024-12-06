package main

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/creack/pty"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

type Recipe struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Stages      []Stage        `yaml:"stages"`
	Messages    RecipeMessages `yaml:"messages"`
}

type RecipeMessages struct {
	Success   string `yaml:"success"`
	Error     string `yaml:"error"`
	Cancelled string `yaml:"cancelled"`
}

type Navigation struct {
	OnSuccess     string `yaml:"on_success"`
	OnNo          string `yaml:"on_no"`
	OnFailure     string `yaml:"on_failure"`
	ErrorMessage  string `yaml:"error_message"`
	CancelMessage string `yaml:"cancel_message"`
}

type Stage struct {
	Type       string                 `yaml:"type"`
	Config     map[string]interface{} `yaml:"config"`
	Name       string                 `yaml:"name"`
	Navigation *Navigation            `yaml:"navigation,omitempty"`
}

type RecipeBook struct {
	Recipes []Recipe `yaml:"recipes"`
}

type StageRunner interface {
	Run(input string, config map[string]interface{}) (string, error)
}

type RecipeError struct {
	Stage     string
	Message   string
	CustomMsg string
	Err       error
}

type NextStep struct {
	NextIndex   int
	IsComplete  bool
	IsCancelled bool
}

func (e *RecipeError) Error() string {
	if e.CustomMsg != "" {
		return e.CustomMsg
	}
	if e.Stage != "" {
		return fmt.Sprintf("Error in stage '%s': %s", e.Stage, e.Message)
	}
	return e.Message
}

type OutputCommandStage struct{}

func (c *OutputCommandStage) Run(input string, config map[string]interface{}) (string, error) {
	command, ok := config["command"].(string)
	if !ok {
		return "", fmt.Errorf("command configuration missing")
	}

	command = strings.ReplaceAll(command, "{{input}}", input)
	cmd := exec.Command("bash", "-c", command)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %v", err)
	}
	defer ptmx.Close()

	var output strings.Builder
	go func() {
		mw := io.MultiWriter(os.Stdout, &output)
		io.Copy(mw, ptmx)
	}()

	return output.String(), cmd.Wait()
}

type InteractiveCommandStage struct{}

func (c *InteractiveCommandStage) Run(input string, config map[string]interface{}) (string, error) {
	command, ok := config["command"].(string)
	if !ok {
		return "", fmt.Errorf("command configuration missing")
	}

	cmd := exec.Command("bash", "-c", strings.ReplaceAll(command, "{{input}}", input))
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start pty: %v", err)
	}
	defer ptmx.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	pty.InheritSize(os.Stdin, ptmx)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH
	defer signal.Stop(ch)

	go io.Copy(ptmx, os.Stdin)
	go io.Copy(os.Stdout, ptmx)

	return "", cmd.Wait()
}

type PromptStage struct{}

func (p *PromptStage) Run(input string, config map[string]interface{}) (string, error) {
	message, ok := config["message"].(string)
	if !ok {
		return "", fmt.Errorf("prompt message missing")
	}

	var userInput string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(message).
				Value(&userInput),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return userInput, nil
}

type SelectStage struct{}

func (s *SelectStage) Run(input string, config map[string]interface{}) (string, error) {
	options, ok := config["options"].([]interface{})
	if !ok {
		return "", fmt.Errorf("select options missing")
	}

	var selected string
	opts := make([]string, len(options))
	for i, opt := range options {
		opts[i] = fmt.Sprint(opt)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose an option").
				Options(huh.NewOptions(opts...)...).
				Value(&selected),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}

type RegexStage struct{}

func (r *RegexStage) Run(input string, config map[string]interface{}) (string, error) {
	pattern, ok := config["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("regex pattern missing")
	}

	replacement, ok := config["replacement"].(string)
	if !ok {
		return "", fmt.Errorf("regex replacement missing")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	return re.ReplaceAllString(input, replacement), nil
}

type ConfirmStage struct{}

func (c *ConfirmStage) Run(input string, config map[string]interface{}) (string, error) {
	message, ok := config["message"].(string)
	if !ok {
		message = "Continue?"
	}

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirmed),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	if !confirmed {
		return input, fmt.Errorf("user declined")
	}

	return input, nil
}

func getStageRunner(stageType string) (StageRunner, error) {
	switch stageType {
	case "output_command":
		return &OutputCommandStage{}, nil
	case "interactive_command":
		return &InteractiveCommandStage{}, nil
	case "prompt":
		return &PromptStage{}, nil
	case "select":
		return &SelectStage{}, nil
	case "regex":
		return &RegexStage{}, nil
	case "confirm":
		return &ConfirmStage{}, nil
	default:
		return nil, fmt.Errorf("unknown stage type: %s", stageType)
	}
}

func findStageByName(recipe Recipe, name string) (int, error) {
	for i, stage := range recipe.Stages {
		if stage.Name == name {
			return i, nil
		}
	}
	return -1, fmt.Errorf("stage not found: %s", name)
}

func getEffectiveMessage(stageMsg, recipeMsg string) string {
	if stageMsg != "" {
		return stageMsg
	}
	if recipeMsg != "" {
		return recipeMsg
	}
	return "Operation completed"
}

func printError(err *RecipeError, defaultMsg string) {
	if err.CustomMsg != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.CustomMsg)
	} else if defaultMsg != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", defaultMsg)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}

func determineNextStep(stage Stage, recipe Recipe, err *RecipeError, output string) NextStep {
	result := NextStep{NextIndex: -1}

	if err != nil {
		result.IsComplete = true
		return result
	}

	if stage.Navigation == nil {
		return result
	}

	if stage.Type == "confirm" && err != nil && err.Error() == "user declined" {
		if stage.Navigation.OnNo != "" {
			if idx, err := findStageByName(recipe, stage.Navigation.OnNo); err == nil {
				result.NextIndex = idx
			} else {
				result.IsComplete = true
				result.IsCancelled = true
			}
		} else {
			result.IsComplete = true
			result.IsCancelled = true
		}
		return result
	}

	if stage.Navigation.OnSuccess != "" {
		if idx, err := findStageByName(recipe, stage.Navigation.OnSuccess); err == nil {
			result.NextIndex = idx
		}
	}

	return result
}

func executeRecipe(recipe Recipe) error {
	var input string
	var recipeError *RecipeError
	currentIndex := 0

	fmt.Printf("Executing recipe: %s\n", recipe.Name)
	fmt.Printf("Description: %s\n\n", recipe.Description)

	for currentIndex < len(recipe.Stages) {
		stage := recipe.Stages[currentIndex]
		fmt.Printf("Stage: %s\n", stage.Name)

		runner, err := getStageRunner(stage.Type)
		if err != nil {
			recipeError = &RecipeError{
				Stage:   stage.Name,
				Message: err.Error(),
				Err:     err,
			}
		}

		var output string
		if recipeError == nil {
			output, err = runner.Run(input, stage.Config)
			if err != nil {
				recipeError = &RecipeError{
					Stage:     stage.Name,
					Message:   err.Error(),
					CustomMsg: stage.Navigation.ErrorMessage,
					Err:       err,
				}
			}
		}

		nextStep := determineNextStep(stage, recipe, recipeError, output)

		if nextStep.IsComplete {
			if recipeError != nil {
				printError(recipeError, recipe.Messages.Error)
				return recipeError
			}
			if nextStep.IsCancelled {
				msg := getEffectiveMessage(stage.Navigation.CancelMessage, recipe.Messages.Cancelled)
				fmt.Println(msg)
				return nil
			}
			fmt.Println(recipe.Messages.Success)
			return nil
		}

		input = output
		if nextStep.NextIndex >= 0 {
			currentIndex = nextStep.NextIndex
		} else {
			currentIndex++
		}
	}

	fmt.Println(recipe.Messages.Success)
	return nil
}

func main() {
	data, err := os.ReadFile("recipes.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read recipes file: %v\n", err)
		os.Exit(1)
	}

	var book RecipeBook
	err = yaml.Unmarshal(data, &book)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse recipes: %v\n", err)
		os.Exit(1)
	}

	var selected string
	opts := make([]string, len(book.Recipes))
	for i, recipe := range book.Recipes {
		opts[i] = fmt.Sprintf("%s - %s", recipe.Name, recipe.Description)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a recipe to run").
				Options(huh.NewOptions(opts...)...).
				Value(&selected),
		),
	)

	err = form.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error selecting recipe: %v\n", err)
		os.Exit(1)
	}

	for i, opt := range opts {
		if opt == selected {
			if err := executeRecipe(book.Recipes[i]); err != nil {
				os.Exit(1)
			}
			break
		}
	}
}
