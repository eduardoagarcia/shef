package internal

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"sort"
	"strings"
)

// recipeInfo represents recipe data for JSON output
type recipeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Author      string `json:"author,omitempty"`
}

// handleListCommand processes the list command and displays recipes
func handleListCommand(c *cli.Context, args []string, sourcePriority []string) error {
	category := determineCategory(c, args)
	sourceFlags := determineSourceFlags(c)

	recipes := collectRecipes(sourcePriority, sourceFlags, category)

	if category == "" {
		recipes = filterDemoRecipes(recipes)
	}

	if len(recipes) == 0 {
		return handleEmptyResults(c)
	}

	if c.Bool("json") {
		return outputRecipesAsJSON(recipes)
	}

	listRecipes(recipes)
	return nil
}

// determineCategory extracts the category from flags or arguments
func determineCategory(c *cli.Context, args []string) string {
	category := c.String("category")
	if category == "" && len(args) >= 1 {
		category = args[0]
	}
	return category
}

// determineSourceFlags checks which recipe sources should be used
func determineSourceFlags(c *cli.Context) map[string]bool {
	useLocal := c.Bool("local") || c.Bool("L")
	useUser := c.Bool("user") || c.Bool("U")
	usePublic := c.Bool("public") || c.Bool("P")

	// If no flags specified, use all sources
	if !useLocal && !useUser && !usePublic {
		useLocal, useUser, usePublic = true, true, true
	}

	return map[string]bool{
		"local":  useLocal,
		"user":   useUser,
		"public": usePublic,
	}
}

// collectRecipes gathers recipes from all specified sources
func collectRecipes(sourcePriority []string, sourceFlags map[string]bool, category string) []Recipe {
	var allRecipes []Recipe
	recipeMap := make(map[string]bool)

	for _, source := range sourcePriority {
		if !sourceFlags[source] {
			continue
		}

		sources, _ := findRecipeSourcesByType(source == "local", source == "user", source == "public")
		recipes, _ := loadRecipes(sources, category)

		for _, r := range recipes {
			if !recipeMap[r.Name] {
				allRecipes = append(allRecipes, r)
				recipeMap[r.Name] = true
			}
		}
	}

	return allRecipes
}

// filterDemoRecipes removes recipes with the "demo" category
func filterDemoRecipes(recipes []Recipe) []Recipe {
	var filtered []Recipe
	for _, recipe := range recipes {
		if recipe.Category != "demo" {
			filtered = append(filtered, recipe)
		}
	}
	return filtered
}

// handleEmptyResults returns appropriate response when no recipes are found
func handleEmptyResults(c *cli.Context) error {
	if c.Bool("json") {
		fmt.Println("[]")
	} else {
		fmt.Println("No recipes found.")
	}
	return nil
}

// listRecipes displays recipes grouped by category in a formatted text output
func listRecipes(recipes []Recipe) {
	if len(recipes) == 0 {
		fmt.Println(FormatText("No recipes found.", ColorYellow, StyleNone))
		return
	}

	fmt.Println("\nAvailable recipes:")

	categories := groupRecipesByCategory(recipes)
	categoryNames := getSortedCategoryNames(categories)

	for _, category := range categoryNames {
		printCategoryHeader(category)
		printCategoryRecipes(categories[category])
	}

	fmt.Printf("\n\n")
}

// groupRecipesByCategory organizes recipes into a map keyed by category
func groupRecipesByCategory(recipes []Recipe) map[string][]Recipe {
	categories := make(map[string][]Recipe)
	for _, recipe := range recipes {
		cat := recipe.Category
		if cat == "" {
			cat = "uncategorized"
		}
		categories[cat] = append(categories[cat], recipe)
	}
	return categories
}

// getSortedCategoryNames returns category names in alphabetical order
func getSortedCategoryNames(categories map[string][]Recipe) []string {
	var names []string
	for category := range categories {
		names = append(names, category)
	}
	sort.Strings(names)
	return names
}

// printCategoryHeader displays the formatted category name
func printCategoryHeader(category string) {
	fmt.Printf(
		"\n  %s%s%s\n",
		FormatText("[", ColorNone, StyleDim),
		FormatText(strings.ToLower(category), ColorMagenta, StyleNone),
		FormatText("]", ColorNone, StyleDim),
	)
}

// printCategoryRecipes displays all recipes within a category
func printCategoryRecipes(recipes []Recipe) {
	sort.Slice(recipes, func(i, j int) bool {
		return recipes[i].Name < recipes[j].Name
	})

	for _, recipe := range recipes {
		fmt.Printf(
			"    %s %s: %s\n",
			FormatText("•", ColorNone, StyleDim),
			FormatText(strings.ToLower(recipe.Name), ColorGreen, StyleBold),
			strings.ToLower(recipe.Description),
		)
	}
}

// outputRecipesAsJSON formats and outputs recipes as JSON
func outputRecipesAsJSON(recipes []Recipe) error {
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
