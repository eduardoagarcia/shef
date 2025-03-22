package internal

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// handleSyncCommand downloads and installs public recipes from the repository
func handleSyncCommand() error {
	dirs, err := setupDirectories()
	if err != nil {
		return err
	}

	tempDir, err := createTempDirectory()
	if err != nil {
		return err
	}
	defer cleanupTempDirectory(tempDir)

	tarballPath, err := downloadRecipes(tempDir)
	if err != nil {
		return err
	}

	extractedDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractedDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := extractTarGz(tarballPath, extractedDir); err != nil {
		return fmt.Errorf("failed to extract recipes: %w", err)
	}

	return installRecipes(extractedDir, dirs.publicDir)
}

// setupDirectories creates the necessary directories for Shef
func setupDirectories() (struct {
	shefDir   string
	publicDir string
	userDir   string
}, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return struct {
			shefDir   string
			publicDir string
			userDir   string
		}{}, fmt.Errorf("failed to determine home directory: %w", err)
	}

	shefDir := filepath.Join(homeDir, ".shef")
	publicDir := filepath.Join(shefDir, "public")
	userDir := filepath.Join(shefDir, "user")

	for _, dir := range []string{publicDir, userDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return struct {
				shefDir   string
				publicDir string
				userDir   string
			}{}, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return struct {
		shefDir   string
		publicDir string
		userDir   string
	}{
		shefDir:   shefDir,
		publicDir: publicDir,
		userDir:   userDir,
	}, nil
}

// createTempDirectory creates a temporary directory for downloading and extracting recipes
func createTempDirectory() (string, error) {
	tempDir, err := os.MkdirTemp("", "shef-recipes")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	return tempDir, nil
}

// cleanupTempDirectory removes the temporary directory
func cleanupTempDirectory(path string) {
	if err := os.RemoveAll(path); err != nil {
		fmt.Printf("Warning: Failed to clean up temporary directory %s: %v\n", path, err)
	}
}

// downloadRecipes downloads the recipes tarball from the repository
func downloadRecipes(tempDir string) (string, error) {
	downloadURL := fmt.Sprintf("%s/releases/download/%s/%s", GithubRepo, Version, PublicRecipesFilename)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download recipes: %w", err)
	}
	defer safeClose(resp.Body, "response body")

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download recipes: HTTP status %d", resp.StatusCode)
	}

	tarballPath := filepath.Join(tempDir, PublicRecipesFilename)
	tarballFile, err := os.Create(tarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer safeClose(tarballFile, "tarball file")

	if _, err = io.Copy(tarballFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save downloaded file: %w", err)
	}

	return tarballPath, nil
}

// installRecipes installs the extracted recipes to the public directory
func installRecipes(extractedDir, publicDir string) error {
	fmt.Println("Installing public recipes...")
	if err := os.RemoveAll(publicDir); err != nil {
		return fmt.Errorf("failed to clean public recipes directory: %w", err)
	}

	recipesDir := filepath.Join(extractedDir, PublicRecipesFolder)
	if err := copyDir(recipesDir, publicDir); err != nil {
		return fmt.Errorf("failed to copy recipes: %w", err)
	}

	fmt.Printf("Success! Public recipes installed to %s\n", publicDir)
	return nil
}

// extractTarGz extracts a tar.gz file to a destination directory
func extractTarGz(tarballPath string, destDir string) error {
	tarFile, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open downloaded tarball: %w", err)
	}
	defer safeClose(tarFile, "tar file")

	gzipReader, err := gzip.NewReader(tarFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer safeClose(gzipReader, "gzip reader")

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar file: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := extractFile(tarReader, header, target); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractFile extracts a single file from a tar archive
func extractFile(tarReader io.Reader, header *tar.Header, target string) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", target, err)
	}
	defer safeClose(file, "extracted file")

	if _, err := io.Copy(file, tarReader); err != nil {
		return fmt.Errorf("failed to write file %s: %w", target, err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	dir, err := os.Open(src)
	if err != nil {
		return err
	}
	defer safeClose(dir, "directory")

	items, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	for _, item := range items {
		srcPath := filepath.Join(src, item.Name())
		dstPath := filepath.Join(dst, item.Name())

		if item.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst, preserving file permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer safeClose(srcFile, "source file")

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer safeClose(dstFile, "destination file")

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// safeClose safely closes an io.Closer and logs any errors
func safeClose(c io.Closer, name string) {
	if err := c.Close(); err != nil {
		fmt.Printf("Warning: Failed to close %s: %v\n", name, err)
	}
}
