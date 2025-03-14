package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func isLinux() bool {
	return runtime.GOOS == "linux"
}

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

func addXDGRecipeSources(sources []string, userDir, publicRepo bool, findYamlFiles func(string) ([]string, error)) []string {
	if userDir {
		xdgUserRoot := filepath.Join(getXDGConfigHome(), "shef", "user")
		if _, err := os.Stat(xdgUserRoot); err == nil {
			if xdgUserFiles, err := findYamlFiles(xdgUserRoot); err == nil {
				sources = append(sources, xdgUserFiles...)
			}
		}
	}

	if publicRepo {
		xdgPublicRoot := filepath.Join(getXDGDataHome(), "shef", "public")
		if _, err := os.Stat(xdgPublicRoot); err == nil {
			if xdgPublicFiles, err := findYamlFiles(xdgPublicRoot); err == nil {
				sources = append(sources, xdgPublicFiles...)
			}
		}
	}

	return sources
}
