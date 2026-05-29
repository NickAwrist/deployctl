package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"deployctl/internal"
	"deployctl/internal/envfile"
	internalfile "deployctl/internal/file"
	internalgit "deployctl/internal/git"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl create <repo-url> [--name <name>] [--compose-file <compose-file>]

Creates a new deployment from a repository URL.

Arguments:

	<repo-url>   The URL of the repository to create a new deployment from
	--name <name> The name of the deployment
	--compose-file <compose-file> The compose file name in the repository or a local compose file path to copy into the repository
	--env-file <env-file> The env file name in the repository or a local env file path to copy into the repository
*/
var createCmd = &cobra.Command{
	Use:   "create [repo-url]",
	Short: "Create a new deployment",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the repository URL from the command line arguments or flags
		repoURL, err := cmd.Flags().GetString("repo-url")
		if err != nil {
			return err
		}
		if repoURL == "" && len(args) > 0 {
			repoURL = args[0]
		}
		if repoURL == "" {
			return errors.New("repo URL is required")
		}

		// Get the name of the deployment from the command line arguments or flags
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}

		// Get the compose file name from the command line arguments or flags
		composeFile, err := cmd.Flags().GetString("compose-file")
		if err != nil {
			return err
		}

		// Clone the repository
		location, err := internalgit.CloneRepo(repoURL, name)
		if err != nil {
			return err
		}
		if name == "" {
			name = filepath.Base(location)
		}

		// Get the env file name from the command line arguments or flags
		envFile, err := cmd.Flags().GetString("env-file")
		if err != nil {
			return err
		}

		// Resolve the compose file path
		composePath, err := resolveComposePath(location, composeFile)
		if err != nil {
			return err
		}
		if composePath == "" {
			internal.Warning("No compose file found. Deployment will not work until a compose file is configured.")
		}

		// Resolve the env file path
		envFilePath, err := resolveEnvFile(location, envFile)
		if err != nil {
			return err
		}

		// Insert the repository into the database
		repositories := store.NewRepositoryStore()
		if err := repositories.Insert(cmd.Context(), store.Repository{
			Name:        name,
			URL:         repoURL,
			Location:    location,
			ComposePath: composePath,
			EnvPath:     envFilePath,
		}); err != nil {
			return err
		}

		internal.Info("Deployment created successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringP("name", "n", "", "The name of the deployment")
	createCmd.Flags().StringP("repo-url", "r", "", "The URL of the repository to create a new deployment from")
	createCmd.Flags().String("compose-file", "", "The compose file name in the repository or a local compose file path to copy into the repository")
	createCmd.Flags().String("env-file", "", "The env file name in the repository or a local env file path to copy into the repository")
}

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
				internal.Warning("Compose file %q was not found in the repository or on disk.", composeFile)
				return "", nil
			}

			destination := filepath.Join(repositoryLocation, "compose.yml")
			if err := internalfile.Copy(path, destination); err != nil {
				return "", fmt.Errorf("copy compose file: %w", err)
			}
			return destination, nil
		}

		internal.Warning("Compose file %q was not found in the repository or on disk.", composeFile)
		return "", nil
	}

	for _, name := range composeFileNames {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, nil
		}
	}

	return "", nil
}

func resolveEnvFile(repositoryLocation string, envFile string) (string, error) {
	if envFile == "" {
		return "", nil
	}

	source, ok := resolveRepositoryOrLocalFile(repositoryLocation, envFile)
	if !ok {
		internal.Warning("Env file %q was not found in the repository or on disk.", envFile)
		return "", nil
	}

	destination := defaultEnvPath(repositoryLocation)
	variables, err := envfile.Read(source)
	if err != nil {
		return "", fmt.Errorf("read env file: %w", err)
	}
	if err := envfile.Write(destination, variables); err != nil {
		return "", fmt.Errorf("copy env file: %w", err)
	}

	return destination, nil
}

func resolveRepositoryOrLocalFile(repositoryLocation string, name string) (string, bool) {
	if !filepath.IsAbs(name) {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, true
		}
	}

	return internalfile.ExistingFile(name)
}
