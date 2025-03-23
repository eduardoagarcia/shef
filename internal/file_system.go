package internal

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

// findRecipeSourceFile locates the file containing a specified recipe
func findRecipeSourceFile(recipeName, category string, sourcePriority []string) (string, error) {
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)

		for _, sourceFile := range sources {
			file, err := loadFile(sourceFile)
			if err != nil {
				continue
			}

			if foundRecipe := findRecipeInFile(file, recipeName, category); foundRecipe {
				return sourceFile, nil
			}
		}
	}

	return "", fmt.Errorf("recipe not found: %s", recipeName)
}

// findRecipeInFile checks if a recipe exists in the given file
func findRecipeInFile(file *File, recipeName, category string) bool {
	for _, recipe := range file.Recipes {
		if recipe.Name == recipeName {
			return true
		}

		if category != "" {
			combinedName := fmt.Sprintf("%s-%s", category, recipeName)
			if recipe.Name == combinedName {
				return true
			}
		}
	}
	return false
}

// findRecipeSourcesByType locates recipe files based on source types
func findRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	// Look for local recipes
	if localDir {
		localFiles := findRecipesInDirectory(".shef")
		sources = append(sources, localFiles...)
	}

	// Look for user and public recipes
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userRoot := filepath.Join(homeDir, ".shef")

		if userDir {
			userFiles := findRecipesInDirectory(filepath.Join(userRoot, "user"))
			sources = append(sources, userFiles...)
		}

		if publicRepo {
			publicFiles := findRecipesInDirectory(filepath.Join(userRoot, "public"))
			sources = append(sources, publicFiles...)
		}

		// Add XDG sources on Linux
		if isLinux() && (userDir || publicRepo) {
			sources = addXDGRecipeSources(sources, userDir, publicRepo, findYamlFiles)
		}
	}

	return sources, nil
}

// findRecipesInDirectory finds all YAML recipe files in a directory
func findRecipesInDirectory(dir string) []string {
	if _, err := os.Stat(dir); err != nil {
		return nil
	}

	files, _ := findYamlFiles(dir)
	return files
}

// findYamlFiles recursively finds all YAML files in a directory
func findYamlFiles(root string) ([]string, error) {
	var files []string
	visited := make(map[string]bool)

	var walkDir func(path string) error
	walkDir = func(path string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil
		}

		if visited[absPath] {
			return nil
		}
		visited[absPath] = true

		// Get file info, will follow symlinks
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil
		}

		if fileInfo.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil
			}

			for _, entry := range entries {
				entryPath := filepath.Join(path, entry.Name())
				if err := walkDir(entryPath); err != nil {
					return err
				}
			}
		} else if isYamlFile(path) {
			files = append(files, path)
		}

		return nil
	}

	err := walkDir(root)
	return files, err
}

// isYamlFile checks if a file has a YAML extension
func isYamlFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}

// getSourcePriority determines the order to search for recipes
func getSourcePriority(c *cli.Context) []string {
	useLocal := c.Bool("local") || c.Bool("L")
	useUser := c.Bool("user") || c.Bool("U")
	usePublic := c.Bool("public") || c.Bool("P")

	if useLocal {
		return []string{"local", "user", "public"}
	} else if useUser {
		return []string{"user", "local", "public"}
	} else if usePublic {
		return []string{"public", "local", "user"}
	}

	return []string{"local", "user", "public"}
}

// loadFile loads and parses a YAML file into a File struct
func loadFile(filename string) (*File, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	return &file, nil
}

// loadRecipes loads recipes from multiple sources with optional category filtering
func loadRecipes(sources []string, category string) ([]Recipe, error) {
	var allRecipes []Recipe
	lowerCategory := strings.ToLower(category)

	for _, source := range sources {
		file, err := loadFile(source)
		if err != nil {
			fmt.Printf("Warning: Failed to load recipes from %s: %v\n", source, err)
			continue
		}

		if category == "" {
			allRecipes = append(allRecipes, file.Recipes...)
			continue
		}

		for _, recipe := range file.Recipes {
			if strings.ToLower(recipe.Category) == lowerCategory {
				allRecipes = append(allRecipes, recipe)
			}
		}
	}

	return allRecipes, nil
}

// handleWhichCommand handles the 'which' command to show recipe file locations
func handleWhichCommand(args []string, sourcePriority []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify a recipe name")
	}

	var category string
	var recipeName string

	if len(args) >= 2 {
		category = args[0]
		recipeName = args[1]
	} else {
		recipeName = args[0]
	}

	sourcePath, err := findRecipeSourceFile(recipeName, category, sourcePriority)
	if err != nil {
		return err
	}

	fmt.Println(sourcePath)
	return nil
}
