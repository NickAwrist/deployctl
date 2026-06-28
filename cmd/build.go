package cmd

import (
	"errors"

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
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.BuildDeployment(cmd.Context(), &rpc.BuildDeploymentRequest{Name: repositoryName})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment built successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	addJobFlags(buildCmd)
}
