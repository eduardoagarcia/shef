package internal

import (
	"os"
	"path/filepath"
	"runtime"
)

// addXDGRecipeSources adds recipe files from XDG directories to the sources list
func addXDGRecipeSources(sources []string, userDir, publicRepo bool, findYamlFiles func(string) ([]string, error)) []string {
	if userDir {
		sources = appendXDGUserRecipes(sources, findYamlFiles)
	}

	if publicRepo {
		sources = appendXDGPublicRecipes(sources, findYamlFiles)
	}

	return sources
}

// appendXDGUserRecipes adds user recipe files from XDG_CONFIG_HOME/shef/user
func appendXDGUserRecipes(sources []string, findYamlFiles func(string) ([]string, error)) []string {
	xdgUserRoot := filepath.Join(getXDGConfigHome(), "shef", "user")
	return appendRecipesIfDirExists(sources, xdgUserRoot, findYamlFiles)
}

// appendXDGPublicRecipes adds public recipe files from XDG_DATA_HOME/shef/public
func appendXDGPublicRecipes(sources []string, findYamlFiles func(string) ([]string, error)) []string {
	xdgPublicRoot := filepath.Join(getXDGDataHome(), "shef", "public")
	return appendRecipesIfDirExists(sources, xdgPublicRoot, findYamlFiles)
}

// appendRecipesIfDirExists adds recipe files if the directory exists
func appendRecipesIfDirExists(sources []string, dirPath string, findYamlFiles func(string) ([]string, error)) []string {
	if _, err := os.Stat(dirPath); err == nil {
		if files, err := findYamlFiles(dirPath); err == nil {
			return append(sources, files...)
		}
	}
	return sources
}

// getXDGConfigHome returns the XDG_CONFIG_HOME directory path
func getXDGConfigHome() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, ".config")
	}
	return configHome
}

// getXDGDataHome returns the XDG_DATA_HOME directory path
func getXDGDataHome() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, ".local", "share")
	}
	return dataHome
}

// isLinux determines if the current operating system is Linux
func isLinux() bool {
	return runtime.GOOS == "linux"
}
