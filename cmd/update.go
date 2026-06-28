package cmd

import (
	"errors"

	"deployctl/internal/rpc"

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
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.UpdateDeployment(cmd.Context(), &rpc.UpdateDeploymentRequest{
				Name:  repositoryName,
				Build: build,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment updated successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().Bool("build", false, "Build deployment images after pulling")
	addJobFlags(updateCmd)
}
