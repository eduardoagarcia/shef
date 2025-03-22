package internal

import (
	"fmt"
	"strings"
)

// displayRecipeHelp renders formatted help information for a given recipe
func displayRecipeHelp(recipe *Recipe) {
	displayNameSection(recipe)

	if recipe.Category != "" {
		displayCategorySection(recipe)
	}

	if recipe.Author != "" {
		displayAuthorSection(recipe)
	}

	displayUsageSection(recipe)
	displayOverviewSection(recipe)

	fmt.Println("")
}

// displayNameSection formats and displays the recipe name and description
func displayNameSection(recipe *Recipe) {
	name := strings.ToLower(recipe.Name)
	description := strings.ToLower(recipe.Description)

	fmt.Printf("%s:\n    %s - %s\n", "NAME", name, description)
}

// displayCategorySection formats and displays the recipe category
func displayCategorySection(recipe *Recipe) {
	category := strings.ToLower(recipe.Category)
	fmt.Printf("\n%s:\n    %s\n", "CATEGORY", category)
}

// displayAuthorSection formats and displays the recipe author
func displayAuthorSection(recipe *Recipe) {
	fmt.Printf("\n%s:\n    %s\n", "AUTHOR", recipe.Author)
}

// displayUsageSection formats and displays usage examples for the recipe
func displayUsageSection(recipe *Recipe) {
	name := strings.ToLower(recipe.Name)

	fmt.Printf("\n%s:\n    shef %s [input] [options]\n", "USAGE", name)

	if recipe.Category != "" {
		category := strings.ToLower(recipe.Category)
		fmt.Printf("    shef %s %s [input] [options]\n", category, name)
	}
}

// displayOverviewSection formats and displays the recipe's detailed help information
func displayOverviewSection(recipe *Recipe) {
	fmt.Printf("\n%s:\n", "OVERVIEW")

	if recipe.Help != "" {
		indentedText := indentText(recipe.Help, 4)
		fmt.Printf("%s\n", indentedText)
	} else {
		fmt.Printf("    %s\n", "No detailed help available for this recipe.")
	}
}

// indentText indents each non-empty line in the given text by a specified number of spaces
func indentText(text string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}

	return strings.Join(lines, "\n")
}
