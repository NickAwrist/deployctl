package cmd

import (
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl build <deployment-name>

Builds deployment images without starting the deployment.

Arguments:

	<deployment-name> The name of the deployment to build
*/
var buildCmd = &cobra.Command{
	Use:               "build [deployment-name]",
	Short:             "Build deployment images",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploymentJob(cmd, args, "built", func(client *daemonClient, deploymentName string) (*rpc.JobResponse, error) {
			return client.Deployment.BuildDeployment(cmd.Context(), &rpc.BuildDeploymentRequest{DeploymentName: deploymentName})
		})
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	addJobFlags(buildCmd)
}
