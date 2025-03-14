package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func syncPublicRecipes() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	shefDir := filepath.Join(homeDir, ".shef")
	publicDir := filepath.Join(shefDir, "public")
	userDir := filepath.Join(shefDir, "user")

	for _, dir := range []string{publicDir, userDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	tempDir, err := os.MkdirTemp("", "shef-recipes")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			fmt.Printf("Warning: Failed to clean up temporary directory %s: %v\n", path, err)
		}
	}(tempDir)

	downloadURL := fmt.Sprintf("%s/releases/download/%s/%s", GithubRepo, Version, PublicRecipesFilename)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download recipes: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download recipes: HTTP status %d", resp.StatusCode)
	}

	tarballPath := filepath.Join(tempDir, PublicRecipesFilename)
	tarballFile, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	if _, err = io.Copy(tarballFile, resp.Body); err != nil {
		err := tarballFile.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to save downloaded file: %w", err)
	}
	err = tarballFile.Close()
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

func extractTarGz(tarballPath string, destDir string) error {
	tarFile, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open downloaded tarball: %w", err)
	}
	defer func(tarFile *os.File) {
		err := tarFile.Close()
		if err != nil {

		}
	}(tarFile)

	gzipReader, err := gzip.NewReader(tarFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func(gzipReader *gzip.Reader) {
		err := gzipReader.Close()
		if err != nil {

		}
	}(gzipReader)

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
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}

			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				err := file.Close()
				if err != nil {
					return err
				}
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
			err = file.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

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
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {

		}
	}(dir)

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

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func(srcFile *os.File) {
		err := srcFile.Close()
		if err != nil {

		}
	}(srcFile)

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(dstFile *os.File) {
		err := dstFile.Close()
		if err != nil {

		}
	}(dstFile)

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}
