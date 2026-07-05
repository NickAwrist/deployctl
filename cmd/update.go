package cmd

import (
	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl update <deployment-name>

Pulls the latest repository changes. Use --build to rebuild images after pulling.

Arguments:

	<deployment-name> The name of the deployment to update
*/
var updateCmd = &cobra.Command{
	Use:               "update [deployment-name]",
	Short:             "Pull latest deployment changes",
	Aliases:           []string{"upgrade", "pull"},
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runDeploymentJob(cmd, args, "Deployment updated successfully", func(client *daemonClient, deploymentName string) (*rpc.JobResponse, error) {
			return client.Deployment.UpdateDeployment(cmd.Context(), &rpc.UpdateDeploymentRequest{
				DeploymentName: deploymentName,
				Build:          build,
			})
		})
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().Bool("build", false, "Build deployment images after pulling")
	addJobFlags(updateCmd)
}
