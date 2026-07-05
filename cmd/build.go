package cmd

import (
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl build <repository-name>

Builds deployment images without starting the deployment.

Arguments:

	<repository-name> The name of the deployment to build
*/
var buildCmd = &cobra.Command{
	Use:               "build [repository-name]",
	Short:             "Build deployment images",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploymentJob(cmd, args, "Deployment built successfully", func(client *daemonClient, repositoryName string) (*rpc.JobResponse, error) {
			return client.Deployment.BuildDeployment(cmd.Context(), &rpc.BuildDeploymentRequest{DeploymentName: repositoryName})
		})
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	addJobFlags(buildCmd)
}
