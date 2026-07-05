package cmd

import (
	"fmt"
	"io"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl list

Lists all deployments.
*/
var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all deployments",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.ListDeploymentSummaries(cmd.Context(), &rpc.ListDeploymentSummariesRequest{})
			if err != nil {
				return err
			}
			if len(response.Deployments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No deployments found.")
				return nil
			}
			printDeploymentList(cmd.OutOrStdout(), response.Deployments)
			return nil
		})
	},
}

func printDeploymentList(output io.Writer, deployments []*rpc.DeploymentSummary) {
	table := newTableWriter(output)
	fmt.Fprintln(table, "NAME\tSTATUS\tREPOSITORY\tLOCATION\tCOMPOSE\tENV")
	for _, item := range deployments {
		deployment := item.GetDeployment()
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			deployment.GetName(),
			formatDeploymentState(item.GetState()),
			emptyAs(deployment.GetRepoUrl(), unknownValue),
			emptyAs(deployment.GetLocation(), unknownValue),
			emptyAs(deployment.GetComposePath(), noneValue),
			emptyAs(deployment.GetEnvPath(), noneValue),
		)
	}
	_ = table.Flush()
}

func init() {
	rootCmd.AddCommand(listCmd)
}
