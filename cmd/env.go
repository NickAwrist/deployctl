package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deployctl/internal"
	"deployctl/internal/envfile"
	internalfile "deployctl/internal/file"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage deployment env variables",
}

var envSetCmd = &cobra.Command{
	Use:               "set [repository-name] [env-file] KEY=VALUE...|ENV_FILE",
	Aliases:           []string{"add"},
	Short:             "Set deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		targetEnvPath, values := resolveEnvSetTarget(repository, args[1:])

		if len(values) == 1 && !strings.Contains(values[0], "=") {
			source, ok := internalfile.ExistingFile(values[0])
			if ok {
				if err := copyEnvFile(targetEnvPath, source); err != nil {
					return err
				}
				if isDefaultEnvPath(repository, targetEnvPath) {
					repository.EnvPath = targetEnvPath
					if err := repositories.Update(cmd.Context(), repository); err != nil {
						return err
					}
				}

				internal.Info("Updated %s for %s", displayEnvPath(repository, targetEnvPath), repository.Name)
				return nil
			}
		}

		variables, err := envfile.Read(targetEnvPath)
		if err != nil {
			return err
		}

		for _, assignment := range values {
			name, value, err := envfile.ParseAssignment(assignment)
			if err != nil {
				return err
			}
			variables[name] = value
		}

		if err := envfile.Write(targetEnvPath, variables); err != nil {
			return err
		}
		if isDefaultEnvPath(repository, targetEnvPath) {
			repository.EnvPath = targetEnvPath
			if err := repositories.Update(cmd.Context(), repository); err != nil {
				return err
			}
		}

		internal.Info("Updated %d env variable(s) in %s for %s", len(values), displayEnvPath(repository, targetEnvPath), repository.Name)
		return nil
	},
}

var envUnsetCmd = &cobra.Command{
	Use:               "unset [repository-name] [env-file] KEY...",
	Aliases:           []string{"delete", "remove", "rm"},
	Short:             "Delete deployment env variables",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		targetEnvPath, names := resolveEnvUnsetTarget(repository, args[1:])

		variables, err := envfile.Read(targetEnvPath)
		if err != nil {
			return err
		}

		deleted := 0
		for _, name := range names {
			if err := envfile.ValidateName(name); err != nil {
				return err
			}
			if _, ok := variables[name]; ok {
				delete(variables, name)
				deleted++
			}
		}

		if err := envfile.Write(targetEnvPath, variables); err != nil {
			return err
		}
		if isDefaultEnvPath(repository, targetEnvPath) {
			repository.EnvPath = targetEnvPath
			if err := repositories.Update(cmd.Context(), repository); err != nil {
				return err
			}
		}

		internal.Info("Deleted %d env variable(s) from %s for %s", deleted, displayEnvPath(repository, targetEnvPath), repository.Name)
		return nil
	},
}

var envListCmd = &cobra.Command{
	Use:               "list [repository-name] [env-file]",
	Short:             "List deployment env variables",
	Args:              cobra.RangeArgs(1, 2),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		envFile := ""
		if len(args) == 2 {
			envFile = args[1]
		}
		targetEnvPath := resolveEnvTargetPath(repository, envFile)

		variables, err := envfile.Read(targetEnvPath)
		if err != nil {
			return err
		}
		if len(variables) == 0 {
			internal.Warning("No env variables found in %s for %s", displayEnvPath(repository, targetEnvPath), repository.Name)
			return nil
		}

		names := make([]string, 0, len(variables))
		for name := range variables {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			internal.Info("%s=*****", name)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envSetCmd, envUnsetCmd, envListCmd)
}

func defaultEnvPath(repositoryLocation string) string {
	return filepath.Join(repositoryLocation, ".env")
}

func copyEnvFile(destination string, source string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create env file directory: %w", err)
	}
	if err := os.WriteFile(destination, contents, 0600); err != nil {
		return fmt.Errorf("copy env file: %w", err)
	}

	return nil
}

func resolveEnvSetTarget(repository store.Repository, values []string) (string, []string) {
	if len(values) >= 2 && !strings.Contains(values[0], "=") {
		return resolveEnvTargetPath(repository, values[0]), values[1:]
	}
	return resolveEnvTargetPath(repository, ""), values
}

func resolveEnvUnsetTarget(repository store.Repository, names []string) (string, []string) {
	if len(names) >= 2 && !looksLikeEnvName(names[0]) {
		return resolveEnvTargetPath(repository, names[0]), names[1:]
	}
	return resolveEnvTargetPath(repository, ""), names
}

func looksLikeEnvName(value string) bool {
	return envfile.ValidateName(value) == nil
}

func resolveEnvTargetPath(repository store.Repository, envFile string) string {
	if envFile == "" {
		return defaultEnvPath(repository.Location)
	}
	if filepath.IsAbs(envFile) {
		return filepath.Clean(envFile)
	}
	return filepath.Join(envFileBaseDir(repository), envFile)
}

func envFileBaseDir(repository store.Repository) string {
	if repository.ComposePath != "" {
		return filepath.Dir(repository.ComposePath)
	}
	return repository.Location
}

func isDefaultEnvPath(repository store.Repository, path string) bool {
	return filepath.Clean(path) == filepath.Clean(defaultEnvPath(repository.Location))
}

func displayEnvPath(repository store.Repository, path string) string {
	base := envFileBaseDir(repository)
	if relative, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(relative, "..") && relative != "." {
		return relative
	}
	return path
}
