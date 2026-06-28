package service

import (
	"fmt"
	"path/filepath"
	"strings"

	internalfile "deployctl/internal/file"
)

var composeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

func resolveComposePath(repositoryLocation string, composeFile string) (string, error) {
	if composeFile != "" {
		isPath := filepath.IsAbs(composeFile) || strings.ContainsAny(composeFile, `/\`)

		if isPath {
			if path, ok := internalfile.ExistingFile(composeFile); ok {
				destination := filepath.Join(repositoryLocation, "compose.yml")
				if err := internalfile.Copy(path, destination); err != nil {
					return "", fmt.Errorf("copy compose file: %w", err)
				}
				return destination, nil
			}
		}

		if !filepath.IsAbs(composeFile) {
			if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, composeFile)); ok {
				return path, nil
			}
		}

		if !isPath {
			path, ok := internalfile.ExistingFile(composeFile)
			if !ok {
				return "", nil
			}

			destination := filepath.Join(repositoryLocation, "compose.yml")
			if err := internalfile.Copy(path, destination); err != nil {
				return "", fmt.Errorf("copy compose file: %w", err)
			}
			return destination, nil
		}

		return "", nil
	}

	for _, name := range composeFileNames {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, nil
		}
	}

	return "", nil
}

func resolveRepositoryOrLocalFile(repositoryLocation string, name string) (string, bool) {
	if !filepath.IsAbs(name) {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, true
		}
	}

	return internalfile.ExistingFile(name)
}

func defaultEnvPath(repositoryLocation string) string {
	return filepath.Join(repositoryLocation, ".env")
}
