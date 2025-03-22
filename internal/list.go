package internal

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"sort"
	"strings"
)

func handleListCommand(c *cli.Context, args []string, sourcePriority []string) error {
	category := c.String("category")
	if category == "" && len(args) >= 1 {
		category = args[0]
	}

	useLocal := c.Bool("local") || c.Bool("L")
	useUser := c.Bool("user") || c.Bool("U")
	usePublic := c.Bool("public") || c.Bool("P")

	if !useLocal && !useUser && !usePublic {
		useLocal = true
		useUser = true
		usePublic = true
	}

	var allRecipes []Recipe
	recipeMap := make(map[string]bool)

	for _, source := range sourcePriority {
		if (source == "local" && !useLocal) ||
			(source == "user" && !useUser) ||
			(source == "public" && !usePublic) {
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

func listRecipes(recipes []Recipe) {
	if len(recipes) == 0 {
		fmt.Println(FormatText("No recipes found.", ColorYellow, StyleNone))
		return
	}

	fmt.Println("\nAvailable recipes:")

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

		fmt.Printf(
			"\n  %s%s%s\n",
			FormatText("[", ColorNone, StyleDim),
			FormatText(strings.ToLower(category), ColorMagenta, StyleNone),
			FormatText("]", ColorNone, StyleDim),
		)
		for _, recipe := range catRecipes {
			fmt.Printf(
				"    %s %s: %s\n",
				FormatText("•", ColorNone, StyleDim),
				FormatText(strings.ToLower(recipe.Name), ColorGreen, StyleBold),
				strings.ToLower(recipe.Description),
			)
		}
	}

	fmt.Printf("\n\n")
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
