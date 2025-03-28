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

// dispatch handles recipe discovery, selection, and execution based on CLI arguments
func dispatch(c *cli.Context, args []string, sourcePriority []string) error {
	debug := c.Bool("debug")

	loadComponents(sourcePriority, debug)

	recipes, remainingArgs, err := loadRecipesToExecute(c, args, sourcePriority, debug)
	if err != nil {
		return err
	}

	if shouldShowHelp(recipes[0], remainingArgs) {
		displayRecipeHelp(&recipes[0])
		return nil
	}

	input, vars := processRemainingArgs(remainingArgs)

	for _, recipe := range recipes {
		if debug {
			printDebugInfo(recipe, input, vars)
		}

		if err := evaluateRecipe(recipe, input, vars, debug); err != nil {
			return err
		}
	}

	return nil
}

// loadComponents discovers and loads components from all sources
func loadComponents(sourcePriority []string, debug bool) {
	sources := discoverComponentSources(sourcePriority)

	globalComponentRegistry.Clear()
	if err := LoadComponents(sources, debug); err != nil && debug {
		fmt.Printf("Warning: Error loading components: %v\n", err)
	}

	if debug {
		fmt.Printf("Loaded %d components from all sources\n", len(globalComponentRegistry.components))
	}
}

// discoverComponentSources finds all files that might contain components
func discoverComponentSources(sourcePriority []string) []string {
	var allSources []string
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		allSources = append(allSources, sources...)
	}

	return allSources
}

// loadRecipesToExecute determines which recipes to run based on provided arguments
func loadRecipesToExecute(c *cli.Context, args []string, sourcePriority []string, debug bool) ([]Recipe, []string, error) {
	recipeFilePath := c.String("recipe-file")

	if recipeFilePath != "" {
		return loadRecipesFromFile(recipeFilePath, args)
	}

	if len(args) == 0 {
		return nil, nil, fmt.Errorf("no recipe specified. Use shef ls to list available recipes")
	}

	return loadRecipeFromArgs(args, sourcePriority, debug)
}

// loadRecipesFromFile loads recipes from a specified file path
func loadRecipesFromFile(filePath string, args []string) ([]Recipe, []string, error) {
	file, err := loadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load recipe file %s: %w", filePath, err)
	}

	registerFileComponents(file)

	return file.Recipes, args, nil
}

// registerFileComponents adds components from a file to the registry
func registerFileComponents(file *File) {
	for _, component := range file.Components {
		if component.ID != "" {
			globalComponentRegistry.Register(component)
		}
	}
}

// loadRecipeFromArgs finds a recipe based on command-line arguments
func loadRecipeFromArgs(args []string, sourcePriority []string, debug bool) ([]Recipe, []string, error) {
	recipe, remainingArgs, err := findRecipeWithOptions(args, sourcePriority, debug)
	if err != nil {
		return nil, nil, err
	}

	return []Recipe{*recipe}, remainingArgs, nil
}

// shouldShowHelp checks if help flags are present in arguments or variables
func shouldShowHelp(recipe Recipe, args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}

	_, vars := processRemainingArgs(args)

	if help, ok := vars["help"]; ok && help == true {
		return true
	}
	if h, ok := vars["h"]; ok && h == true {
		return true
	}

	return false
}

// printDebugInfo outputs debug information about the recipe being executed
func printDebugInfo(recipe Recipe, input string, vars map[string]interface{}) {
	fmt.Printf("Running recipe: %s\n", recipe.Name)
	fmt.Printf("With input: %s\n", input)
	fmt.Printf("With vars: %v\n", vars)
	fmt.Printf("Description: %s\n\n", recipe.Description)
}

// findRecipeWithOptions tries different strategies to find a matching recipe
func findRecipeWithOptions(args []string, sourcePriority []string, debug bool) (*Recipe, []string, error) {
	// 1. try exact name match
	recipe, err := findRecipeByExactName(args[0], "", sourcePriority)
	if err == nil {
		return recipe, args[1:], nil
	}

	// 2. try category and name match if we have enough args
	if len(args) > 1 {
		recipe, err = findRecipeByExactName(args[1], args[0], sourcePriority)
		if err == nil {
			return recipe, args[2:], nil
		}

		// 2b. try fuzzy match with category
		recipe, err = findRecipeByFuzzyName(args[1], args[0], sourcePriority)
		if err == nil {
			return recipe, args[2:], nil
		}
	}

	// 3. try category selection
	recipe, err = handleCategorySelection(args[0], sourcePriority, debug)
	if err == nil {
		return recipe, args[1:], nil
	} else if err.Error() == "recipe selection aborted by user" {
		os.Exit(0)
	}

	// 4. try fuzzy match without category
	recipe, err = findRecipeByFuzzyName(args[0], "", sourcePriority)
	if err == nil {
		return recipe, args[1:], nil
	}

	return nil, nil, fmt.Errorf("recipe not found: %s", args[0])
}

// findRecipeByExactName looks for an exact recipe name match
func findRecipeByExactName(recipeName, category string, sourcePriority []string) (*Recipe, error) {
	return findRecipeInSources(recipeName, category, sourcePriority, false)
}

// findRecipeByFuzzyName looks for a recipe with fuzzy name matching
func findRecipeByFuzzyName(recipeName, category string, sourcePriority []string) (*Recipe, error) {
	return findRecipeInSources(recipeName, category, sourcePriority, true)
}

// findRecipeInSources searches for a recipe across various recipe sources
func findRecipeInSources(recipeName, category string, sourcePriority []string, fuzzyMatch bool) (*Recipe, error) {
	for _, source := range sourcePriority {
		recipe, found := searchSourceForRecipe(source, recipeName, category)
		if found {
			return recipe, nil
		}
	}

	if fuzzyMatch {
		allRecipes := collectAllUniqueRecipes(sourcePriority)

		if len(allRecipes) > 0 {
			if match, found := fuzzyMatchRecipe(recipeName, extractRecipeNames(allRecipes), createRecipeMap(allRecipes)); found {
				return match, nil
			}
		}
	}

	return nil, fmt.Errorf("recipe not found: %s", recipeName)
}

// searchSourceForRecipe searches for a recipe in a specific source
func searchSourceForRecipe(source, recipeName, category string) (*Recipe, bool) {
	useLocal := source == "local"
	useUser := source == "user"
	usePublic := source == "public"

	sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
	recipes, _ := loadRecipes(sources, category)

	recipe, err := findRecipeByName(recipes, recipeName)
	if err == nil {
		return recipe, true
	}

	if category != "" {
		combinedName := fmt.Sprintf("%s-%s", category, recipeName)
		recipe, err = findRecipeByName(recipes, combinedName)
		if err == nil {
			return recipe, true
		}
	}

	return nil, false
}

// collectAllUniqueRecipes gathers unique recipes from all sources
func collectAllUniqueRecipes(sourcePriority []string) []Recipe {
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

	return allRecipes
}

// extractRecipeNames gets all recipe names from a slice of recipes
func extractRecipeNames(recipes []Recipe) []string {
	names := make([]string, 0, len(recipes))
	for _, recipe := range recipes {
		names = append(names, recipe.Name)
	}
	return names
}

// createRecipeMap builds a map of recipe names to recipes
func createRecipeMap(recipes []Recipe) map[string]Recipe {
	recipeMap := make(map[string]Recipe)
	for _, recipe := range recipes {
		recipeMap[recipe.Name] = recipe
	}
	return recipeMap
}

// findRecipeByName looks for a recipe with an exact name match
func findRecipeByName(recipes []Recipe, name string) (*Recipe, error) {
	lowerName := strings.ToLower(name)
	for _, recipe := range recipes {
		if strings.ToLower(recipe.Name) == lowerName {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

// fuzzyMatchRecipe finds the closest recipe name and confirms with the user
func fuzzyMatchRecipe(recipeName string, recipeNames []string, recipeMap map[string]Recipe) (*Recipe, bool) {
	if len(recipeNames) == 0 {
		return nil, false
	}

	matches := findClosestRecipeMatches(recipeName, recipeNames)

	if len(matches) > 0 {
		bestMatch := matches[0]
		recipe := recipeMap[bestMatch.name]

		if confirmRecipeMatch(recipe) {
			return &recipe, true
		}
	}

	return nil, false
}

// findClosestRecipeMatches finds recipes with names closest to the search term
func findClosestRecipeMatches(recipeName string, recipeNames []string) []struct {
	name     string
	distance int
} {
	var matches []struct {
		name     string
		distance int
	}

	for _, name := range recipeNames {
		distance := levenshtein.ComputeDistance(recipeName, name)
		matches = append(matches, struct {
			name     string
			distance int
		}{name: name, distance: distance})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	return matches
}

// confirmRecipeMatch asks the user to confirm a fuzzy-matched recipe
func confirmRecipeMatch(recipe Recipe) bool {
	var confirm bool
	var promptMessage string

	if recipe.Category != "" {
		promptMessage = fmt.Sprintf("Did you mean [%s] '%s'?", recipe.Category, recipe.Name)
	} else {
		promptMessage = fmt.Sprintf("Did you mean '%s'?", recipe.Name)
	}

	prompt := &survey.Confirm{
		Message: promptMessage,
		Default: true,
	}

	return survey.AskOne(prompt, &confirm) == nil && confirm
}

// processRemainingArgs converts CLI arguments into input string and variables
func processRemainingArgs(args []string) (string, map[string]interface{}) {
	vars := make(map[string]interface{})
	var input string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			processFlag(arg, vars)
		} else if input == "" {
			input = arg
		}
	}

	return input, vars
}

// processFlag handles CLI flags and adds them to the variables map
func processFlag(arg string, vars map[string]interface{}) {
	if strings.HasPrefix(arg, "--") {
		processLongFlag(arg[2:], vars) // Remove --
	} else {
		processShortFlag(arg[1:], vars) // Remove -
	}
}

// processLongFlag handles --flag style arguments
func processLongFlag(arg string, vars map[string]interface{}) {
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		flagName := strings.ReplaceAll(parts[0], "-", "_")
		vars[flagName] = parts[1]
	} else {
		flagName := strings.ReplaceAll(arg, "-", "_")
		vars[flagName] = true
	}
}

// processShortFlag handles -f style arguments
func processShortFlag(arg string, vars map[string]interface{}) {
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		vars[parts[0]] = parts[1]
	} else {
		for _, c := range arg {
			vars[string(c)] = true
		}
	}
}

// handleCategorySelection prompts the user to select a recipe from a category
func handleCategorySelection(categoryName string, sourcePriority []string, debug bool) (*Recipe, error) {
	recipes := collectRecipesInCategory(categoryName, sourcePriority)

	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipes found in category: %s", categoryName)
	}

	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].Name < recipes[j].Name
	})

	selected, err := promptForRecipeSelection(recipes, categoryName)
	if err != nil {
		return nil, err
	}

	if selected == ExitPrompt {
		return nil, fmt.Errorf("recipe selection aborted by user")
	}

	for _, recipe := range recipes {
		if recipe.Name == selected {
			return &recipe, nil
		}
	}

	return nil, fmt.Errorf("recipe not found: %s", selected)
}

// collectRecipesInCategory gathers all recipes in a specific category
func collectRecipesInCategory(categoryName string, sourcePriority []string) []Recipe {
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
				allRecipes = append(allRecipes, recipe)
			}
		}
	}

	return allRecipes
}

// promptForRecipeSelection shows a selection dialog for recipes
func promptForRecipeSelection(recipes []Recipe, categoryName string) (string, error) {
	options := make([]string, len(recipes)+1)
	for i, recipe := range recipes {
		options[i] = recipe.Name
	}
	options[len(recipes)] = ExitPrompt

	prompt := &survey.Select{
		Message: fmt.Sprintf("Choose a recipe from %s:", categoryName),
		Options: options,
	}

	var selected string
	err := survey.AskOne(prompt, &selected)

	return selected, err
}
