package cmd

import (
	"errors"

	"deployctl/internal"
	"deployctl/internal/docker"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl restart <repository-name>

Restarts a deployment. Use --build to rebuild images before restarting.

Arguments:

	<repository-name> The name of the deployment to restart
*/
var restartCmd = &cobra.Command{
	Use:               "restart [repository-name]",
	Short:             "Restart a deployment",
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

		status, err := docker.ComposeStatus(cmd.Context(), &repository)
		if err != nil {
			return err
		}
		if !status.AnyRunning() {
			internal.Info("Deployment is not running. Starting it now...")
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		ready, err := prepareDeploymentBuild(cmd.InOrStdin(), cmd.Context(), &repository, build)
		if err != nil {
			return err
		}
		if !ready {
			return nil
		}

		if err := docker.ComposeDown(cmd.Context(), &repository); err != nil {
			return err
		}
		if err := docker.ComposeUp(cmd.Context(), &repository); err != nil {
			return err
		}

		internal.Info("Deployment restarted successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)

	restartCmd.Flags().Bool("build", false, "Build deployment images before restarting")
}
