package internal

import (
	"os"
	"path/filepath"
)

func MainDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".deployctl"), nil
}

func RepositoryDirectory() (string, error) {
	mainDirectory, err := MainDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainDirectory, "repositories"), nil
}

func InitializeDirectoryStructure() error {
	mainDirectory, err := MainDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(mainDirectory, 0755); err != nil {
		return err
	}
	repositoryDirectory, err := RepositoryDirectory()
	if err != nil {
		return err
	}
	return os.MkdirAll(repositoryDirectory, 0755)
}
