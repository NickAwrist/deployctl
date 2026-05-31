package cmd

import (
	"errors"

	"deployctl/internal"
	"deployctl/internal/docker"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl build <repository-name>

Builds deployment images without starting the deployment.

Arguments:

	<repository-name> The name of the deployment to build
*/
var buildCmd = &cobra.Command{
	Use:               "build [repository-name]",
	Short:             "Build deployment images",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := ""
		if len(args) > 0 {
			repositoryName = args[0]
			if repositoryName == "" {
				return errors.New("repository name is required")
			}
		}

		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		if err := docker.ComposeBuild(cmd.Context(), &repository); err != nil {
			return err
		}

		internal.Info("Deployment built successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
