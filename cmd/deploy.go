package cmd

import (
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
		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runDeploymentJob(cmd, args, "Deployment deployed successfully", func(client *daemonClient, repositoryName string) (*rpc.JobResponse, error) {
			return client.Deployment.DeployDeployment(cmd.Context(), &rpc.DeployDeploymentRequest{
				DeploymentName: repositoryName,
				Build:          build,
			})
		})
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().Bool("build", false, "Build deployment images before starting")
	addJobFlags(deployCmd)
}
