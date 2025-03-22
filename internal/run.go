package internal

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/agnivade/levenshtein"
	"github.com/urfave/cli/v2"
	"os"
	"sort"
	"strings"
)

func dispatch(c *cli.Context, args []string, sourcePriority []string) error {
	debug := c.Bool("debug")
	var recipes []Recipe
	var remainingArgs []string

	recipeFilePath := c.String("recipe-file")
	if recipeFilePath != "" {
		file, err := loadFile(recipeFilePath)
		if err != nil {
			return fmt.Errorf("failed to load recipe file %s: %w", recipeFilePath, err)
		}
		recipes = file.Recipes
		remainingArgs = args
	} else {
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified. Use shef ls to list available recipes")
		}

		var recipe *Recipe
		var err error

		recipe, remainingArgs, err = findRecipeWithOptions(args, sourcePriority, debug)
		if err != nil {
			return err
		}

		for _, arg := range remainingArgs {
			if arg == "-h" || arg == "--help" {
				displayRecipeHelp(recipe)
				return nil
			}
		}

		recipes = []Recipe{*recipe}
	}

	input, vars := processRemainingArgs(remainingArgs)

	if help, ok := vars["help"]; ok && help == true {
		displayRecipeHelp(&recipes[0])
		return nil
	}
	if h, ok := vars["h"]; ok && h == true {
		displayRecipeHelp(&recipes[0])
		return nil
	}

	for _, recipe := range recipes {
		if debug {
			fmt.Printf("Running recipe: %s\n", recipe.Name)
			fmt.Printf("With input: %s\n", input)
			fmt.Printf("With vars: %v\n", vars)
			fmt.Printf("Description: %s\n\n", recipe.Description)
		}

		if err := evaluateRecipe(recipe, input, vars, debug); err != nil {
			return err
		}
	}

	return nil
}

func findRecipeWithOptions(args []string, sourcePriority []string, debug bool) (*Recipe, []string, error) {
	var recipe *Recipe
	var err error

	// 1. try to match a recipe with the given name
	recipe, err = findRecipeInSources(args[0], "", sourcePriority, false)
	if err == nil {
		return recipe, args[1:], nil
	}

	// 2. try to match a recipe with the given category and name
	if len(args) > 1 {
		recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, false)
		if err == nil {
			return recipe, args[2:], nil
		}
	}

	// 3. try to fuzzy match with the given category
	if len(args) > 1 {
		recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, true)
		if err == nil {
			return recipe, args[2:], nil
		}
	}

	// 4. try to match category to prompt for selection
	recipe, err = handleCategorySelection(args[0], sourcePriority, debug)
	if err == nil {
		return recipe, args[1:], nil
	} else if err.Error() == "recipe selection aborted by user" {
		os.Exit(0)
	}

	// 5. try to fuzzy match without category
	recipe, err = findRecipeInSources(args[0], "", sourcePriority, true)
	if err == nil {
		return recipe, args[1:], nil
	}

	return nil, nil, fmt.Errorf("recipe not found: %s", args[0])
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

func findRecipeByName(recipes []Recipe, name string) (*Recipe, error) {
	lowerName := strings.ToLower(name)
	for _, recipe := range recipes {
		if strings.ToLower(recipe.Name) == lowerName {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
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

func handleCategorySelection(categoryName string, sourcePriority []string, debug bool) (*Recipe, error) {
	var allRecipes []Recipe
	recipeMap := make(map[string]Recipe)

	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		recipes, _ := loadRecipes(sources, categoryName)

		for _, recipe := range recipes {
			if _, exists := recipeMap[recipe.Name]; !exists {
				recipeMap[recipe.Name] = recipe
			}
		}
	}

	if len(recipeMap) == 0 {
		return nil, fmt.Errorf("no recipes found in category: %s", categoryName)
	}

	for _, recipe := range recipeMap {
		allRecipes = append(allRecipes, recipe)
	}
	sort.Slice(allRecipes, func(i, j int) bool {
		return allRecipes[i].Name < allRecipes[j].Name
	})

	options := make([]string, len(allRecipes)+1)
	for i, recipe := range allRecipes {
		options[i] = recipe.Name
	}
	options[len(allRecipes)] = ExitPrompt

	prompt := &survey.Select{
		Message: fmt.Sprintf("Choose a recipe from %s:", categoryName),
		Options: options,
	}

	var selected string
	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	if selected == ExitPrompt {
		return nil, fmt.Errorf("recipe selection aborted by user")
	}

	selectedRecipe := recipeMap[selected]
	return &selectedRecipe, nil
}
