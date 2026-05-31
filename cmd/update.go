package cmd

import (
	"errors"

	"deployctl/internal"
	"deployctl/internal/docker"
	internalgit "deployctl/internal/git"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl update <repository-name>

Pulls the latest repository changes. Use --build to rebuild images after pulling.

Arguments:

	<repository-name> The name of the deployment to update
*/
var updateCmd = &cobra.Command{
	Use:               "update [repository-name]",
	Short:             "Pull latest deployment changes",
	Aliases:           []string{"upgrade", "pull"},
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

		if err := internalgit.PullRepo(repository.Location); err != nil {
			return err
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}
		if build {
			if err := docker.ComposeBuild(cmd.Context(), &repository); err != nil {
				return err
			}
		}

		internal.Info("Deployment updated successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().Bool("build", false, "Build deployment images after pulling")
}
