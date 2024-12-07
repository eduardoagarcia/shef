package main

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/eduardoagarcia/cookbook/recipes"
	"github.com/eduardoagarcia/cookbook/stages"
	"os"
	"path/filepath"
)

func executeRecipe(recipe recipes.Recipe) error {
	var input string
	var recipeError *RecipeError
	currentIndex := 0

	fmt.Printf("Executing recipe: %s\n", recipe.Name)
	fmt.Printf("Description: %s\n\n", recipe.Description)

	for currentIndex < len(recipe.Stages) {
		stage := recipe.Stages[currentIndex]
		fmt.Printf("Stage: %s\n", stage.Name)

		runner, err := stages.GetStageRunner(stage.Type)
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

type RecipeError struct {
	Stage     string
	Message   string
	CustomMsg string
	Err       error
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

type NextStep struct {
	NextIndex   int
	IsComplete  bool
	IsCancelled bool
}

func findStageByName(recipe recipes.Recipe, name string) (int, error) {
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

func determineNextStep(stage recipes.Stage, recipe recipes.Recipe, err *RecipeError, output string) NextStep {
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

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	recipesPath := filepath.Join(homeDir, ".recipes")

	book, err := recipes.LoadRecipes(recipesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load recipes: %v\n", err)
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
				Title("Select a recipe").
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
