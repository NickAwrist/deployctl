package cmd

import (
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl restart <deployment-name>

Restarts a deployment. Use --build to rebuild images before restarting.

Arguments:

	<deployment-name> The name of the deployment to restart
*/
var restartCmd = &cobra.Command{
	Use:               "restart [deployment-name]",
	Short:             "Restart a deployment",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runDeploymentJob(cmd, args, "restarted", func(client *daemonClient, deploymentName string) (*rpc.JobResponse, error) {
			return client.Deployment.RestartDeployment(cmd.Context(), &rpc.RestartDeploymentRequest{
				DeploymentName: deploymentName,
				Build:          build,
			})
		})
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
	restartCmd.Flags().Bool("build", false, "Build deployment images before restarting")
	addJobFlags(restartCmd)
}
