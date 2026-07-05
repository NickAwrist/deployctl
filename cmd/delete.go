package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl delete <deployment-name>

Deletes a deployment.

Arguments:

	<deployment-name> The name of the deployment to delete
*/
var deleteCmd = &cobra.Command{
	Use:               "delete [deployment-name]",
	Short:             "Delete a deployment",
	Aliases:           []string{"remove", "rm"},
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		// Get the force flag from the command line arguments or flags
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}

		// Prompt the user for confirmation
		confirmed, err := confirmDelete(cmd.InOrStdin(), cmd.OutOrStdout(), deploymentName, force)
		if err != nil {
			return err
		}
		if !confirmed && !force {
			fmt.Fprintln(cmd.OutOrStdout(), "Delete cancelled")
			return nil
		}

		return runDeploymentJob(cmd, args, fmt.Sprintf("Deleted deployment %s", deploymentName), func(client *daemonClient, deploymentName string) (*rpc.JobResponse, error) {
			return client.Deployment.DeleteDeployment(cmd.Context(), &rpc.DeleteDeploymentRequest{DeploymentName: deploymentName})
		})
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolP("force", "f", false, "Force deletion without confirmation")
	addJobFlags(deleteCmd)
}

func confirmDelete(input io.Reader, output io.Writer, deploymentName string, force bool) (bool, error) {
	if force {
		return true, nil
	}

	fmt.Fprintf(output, "Are you sure you want to permanently delete %s? (Y/n) ", deploymentName)

	reader := bufio.NewReader(input)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
