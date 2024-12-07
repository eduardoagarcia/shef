package recipes

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
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

type Stage struct {
	Type       string                 `yaml:"type"`
	Config     map[string]interface{} `yaml:"config"`
	Name       string                 `yaml:"name"`
	Navigation *Navigation            `yaml:"navigation,omitempty"`
}

type Navigation struct {
	OnSuccess     string `yaml:"on_success"`
	OnNo          string `yaml:"on_no"`
	OnFailure     string `yaml:"on_failure"`
	ErrorMessage  string `yaml:"error_message"`
	CancelMessage string `yaml:"cancel_message"`
}

type RecipeBook struct {
	Recipes []Recipe
}

func LoadRecipes(recipesPath string) (*RecipeBook, error) {
	book := &RecipeBook{}

	entries, err := os.ReadDir(recipesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipes directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			data, err := os.ReadFile(filepath.Join(recipesPath, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to read recipe file %s: %v", entry.Name(), err)
			}

			var recipe Recipe
			if err := yaml.Unmarshal(data, &recipe); err != nil {
				return nil, fmt.Errorf("failed to parse recipe file %s: %v", entry.Name(), err)
			}

			book.Recipes = append(book.Recipes, recipe)
		}
	}

	return book, nil
}
