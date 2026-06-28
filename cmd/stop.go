package cmd

import (
	"errors"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl stop <repository-name>

Stops a deployment.

Arguments:

	<repository-name> The name of the deployment to stop
*/
var stopCmd = &cobra.Command{
	Use:               "stop [repository-name]",
	Short:             "Stop a deployment",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.StopDeployment(cmd.Context(), &rpc.StopDeploymentRequest{Name: repositoryName})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment stopped successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	addJobFlags(stopCmd)
}
