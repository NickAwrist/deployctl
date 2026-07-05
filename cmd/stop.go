package cmd

import (
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
		return runDeploymentJob(cmd, args, "Deployment stopped successfully", func(client *daemonClient, repositoryName string) (*rpc.JobResponse, error) {
			return client.Deployment.StopDeployment(cmd.Context(), &rpc.StopDeploymentRequest{DeploymentName: repositoryName})
		})
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	addJobFlags(stopCmd)
}
