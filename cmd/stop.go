package cmd

import (
	"errors"

	"deployctl/internal/docker"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)
/*
deployctl stop <repository-name>

Stops a deployment.

Arguments:
  <repository-name> The name of the deployment to stop
*/
var stopCmd = &cobra.Command{
	Use:   "stop [repository-name]",
	Short: "Stop a deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the repository name from the command line arguments or flags
		repositoryName := ""
		if len(args) > 0 {
			repositoryName = args[0]
			if repositoryName == "" {
				return errors.New("repository name is required")
			}
		}

		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		// Get the repository from the database
		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		// Stop the repository
		err = docker.ComposeDown(cmd.Context(), &repository)
		if err != nil {
			return err
		}

		internal.Info("Deployment stopped successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
