package cmd

import (
	"errors"

	"deployctl/internal"
	"deployctl/internal/docker"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)
/*
deployctl deploy <repository-name>

Deploys a deployment.

Arguments:
  <repository-name> The name of the deployment to deploy
*/
var deployCmd = &cobra.Command{
	Use:   "deploy [repository-name]",
	Short: "Deploy a deployment",
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

		// Get the repository from the database
		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		// Deploy the repository
		err = docker.ComposeUp(cmd.Context(), &repository)
		if err != nil {
			return err
		}

		internal.Info("Deployment deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
