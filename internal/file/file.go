package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExistingFile(path string) (string, bool) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil || info.IsDir() {
		return "", false
	}

	absolutePath, err := filepath.Abs(cleanPath)
	if err != nil {
		return cleanPath, true
	}

	return absolutePath, true
}

func Copy(source string, destination string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destination)
	if err != nil {
		return err
	}

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		_ = destinationFile.Close()
		return err
	}

	return destinationFile.Close()
}

func RemoveAllInside(baseDirectory string, target string) error {
	if target == "" {
		return errors.New("target path is empty")
	}

	baseDirectory, err := filepath.Abs(baseDirectory)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}

	relativePath, err := filepath.Rel(baseDirectory, target)
	if err != nil {
		return err
	}
	if relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relativePath) {
		return fmt.Errorf("refusing to delete path outside base directory: %s", target)
	}

	return os.RemoveAll(target)
}
