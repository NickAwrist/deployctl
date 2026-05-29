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
	Use:               "set [repository-name] KEY=VALUE...|ENV_FILE",
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

		if len(args) == 2 && !strings.Contains(args[1], "=") {
			source, ok := internalfile.ExistingFile(args[1])
			if ok {
				if err := copyEnvFile(&repository, source); err != nil {
					return err
				}
				if err := repositories.Update(cmd.Context(), repository); err != nil {
					return err
				}

				internal.Info("Updated env file for %s", repository.Name)
				return nil
			}
		}

		variables, err := envfile.Read(repository.EnvPath)
		if err != nil {
			return err
		}

		for _, assignment := range args[1:] {
			name, value, err := envfile.ParseAssignment(assignment)
			if err != nil {
				return err
			}
			variables[name] = value
		}

		if repository.EnvPath == "" {
			repository.EnvPath = defaultEnvPath(repository.Location)
		}
		if err := envfile.Write(repository.EnvPath, variables); err != nil {
			return err
		}
		if err := repositories.Update(cmd.Context(), repository); err != nil {
			return err
		}

		internal.Info("Updated %d env variable(s) for %s", len(args)-1, repository.Name)
		return nil
	},
}

var envUnsetCmd = &cobra.Command{
	Use:               "unset [repository-name] KEY...",
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

		variables, err := envfile.Read(repository.EnvPath)
		if err != nil {
			return err
		}

		deleted := 0
		for _, name := range args[1:] {
			if err := envfile.ValidateName(name); err != nil {
				return err
			}
			if _, ok := variables[name]; ok {
				delete(variables, name)
				deleted++
			}
		}

		if repository.EnvPath == "" {
			repository.EnvPath = defaultEnvPath(repository.Location)
		}
		if err := envfile.Write(repository.EnvPath, variables); err != nil {
			return err
		}
		if err := repositories.Update(cmd.Context(), repository); err != nil {
			return err
		}

		internal.Info("Deleted %d env variable(s) from %s", deleted, repository.Name)
		return nil
	},
}

var envListCmd = &cobra.Command{
	Use:               "list [repository-name]",
	Short:             "List deployment env variables",
	Args:              cobra.ExactArgs(1),
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

		variables, err := envfile.Read(repository.EnvPath)
		if err != nil {
			return err
		}
		if len(variables) == 0 {
			internal.Warning("No env variables found for %s", repository.Name)
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

func copyEnvFile(repository *store.Repository, source string) error {
	if repository.EnvPath == "" {
		repository.EnvPath = defaultEnvPath(repository.Location)
	}

	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	if err := os.WriteFile(repository.EnvPath, contents, 0600); err != nil {
		return fmt.Errorf("copy env file: %w", err)
	}

	return nil
}
