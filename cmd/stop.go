package cmd

import (
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl stop <deployment-name>

Stops a deployment.

Arguments:

	<deployment-name> The name of the deployment to stop
*/
var stopCmd = &cobra.Command{
	Use:               "stop [deployment-name]",
	Short:             "Stop a deployment",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploymentJob(cmd, args, "stopped", func(client *daemonClient, deploymentName string) (*rpc.JobResponse, error) {
			return client.Deployment.StopDeployment(cmd.Context(), &rpc.StopDeploymentRequest{DeploymentName: deploymentName})
		})
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	addJobFlags(stopCmd)
}
