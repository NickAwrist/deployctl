package cmd

import (
	"errors"

	"deployctl/internal/rpc"

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
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.RestartDeployment(cmd.Context(), &rpc.RestartDeploymentRequest{
				Name:  repositoryName,
				Build: build,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment restarted successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().Bool("build", false, "Build deployment images before restarting")
	addJobFlags(restartCmd)
}
