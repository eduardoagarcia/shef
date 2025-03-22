package internal

import (
	"fmt"
	"strings"
)

func displayRecipeHelp(recipe *Recipe) {
	name := strings.ToLower(recipe.Name)
	category := strings.ToLower(recipe.Category)
	description := strings.ToLower(recipe.Description)

	fmt.Printf("%s:\n    %s - %s\n", "NAME", name, description)

	if recipe.Category != "" {
		fmt.Printf("\n%s:\n    %s\n", "CATEGORY", category)
	}

	if recipe.Author != "" {
		fmt.Printf("\n%s:\n    %s\n", "AUTHOR", recipe.Author)
	}

	fmt.Printf("\n%s:\n    shef %s [input] [options]\n", "USAGE", name)
	if recipe.Category != "" {
		fmt.Printf("    shef %s %s [input] [options]\n", category, name)
	}

	if recipe.Help != "" {
		indentedText := indentText(recipe.Help, 4)
		fmt.Printf("\n%s:\n%s\n", "OVERVIEW", indentedText)
	} else {
		fmt.Printf("\n%s:\n    %s\n", "OVERVIEW", "No detailed help available for this recipe.")
	}

	fmt.Println("")
}

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
