package cmd

import (
	"errors"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl deploy <repository-name>

Deploys a deployment. Use --build to rebuild images before starting.

Arguments:

	<repository-name> The name of the deployment to deploy
*/
var deployCmd = &cobra.Command{
	Use:               "deploy [repository-name]",
	Short:             "Deploy a deployment",
	Aliases:           []string{"start"},
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
			response, err := client.Deployment.DeployDeployment(cmd.Context(), &rpc.DeployDeploymentRequest{
				Name:  repositoryName,
				Build: build,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment deployed successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().Bool("build", false, "Build deployment images before starting")
	addJobFlags(deployCmd)
}
