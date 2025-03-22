package internal

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

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

			for _, recipe := range file.Recipes {
				if recipe.Name == recipeName {
					return sourceFile, nil
				}

				if category != "" {
					combinedName := fmt.Sprintf("%s-%s", category, recipeName)
					if recipe.Name == combinedName {
						return sourceFile, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("recipe not found: %s", recipeName)
}

func findRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	findYamlFiles := func(root string) ([]string, error) {
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
			} else if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}

			return nil
		}

		err := walkDir(root)
		return files, err
	}

	if localDir {
		if _, err := os.Stat(".shef"); err == nil {
			if localFiles, err := findYamlFiles(".shef"); err == nil {
				sources = append(sources, localFiles...)
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		userRoot := filepath.Join(homeDir, ".shef")

		if userDir {
			userSpecificDir := filepath.Join(userRoot, "user")
			if _, err := os.Stat(userSpecificDir); err == nil {
				if userFiles, err := findYamlFiles(userSpecificDir); err == nil {
					sources = append(sources, userFiles...)
				}
			}
		}

		if publicRepo {
			publicSpecificDir := filepath.Join(userRoot, "public")
			if _, err := os.Stat(publicSpecificDir); err == nil {
				if publicFiles, err := findYamlFiles(publicSpecificDir); err == nil {
					sources = append(sources, publicFiles...)
				}
			}
		}

		if isLinux() && (userDir || publicRepo) {
			sources = addXDGRecipeSources(sources, userDir, publicRepo, findYamlFiles)
		}
	}

	return sources, nil
}

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

func which(args []string, sourcePriority []string) error {
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
