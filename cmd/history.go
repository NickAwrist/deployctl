package cmd

import (
	"fmt"
	"io"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:               "history [repository-name]",
	Short:             "Show deployment job history",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Job.ListJobs(cmd.Context(), &rpc.ListJobsRequest{DeploymentName: repositoryName})
			if err != nil {
				return err
			}
			if len(response.GetJobs()) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No jobs found for deployment %q\n", repositoryName)
				return nil
			}
			printJobHistory(cmd.OutOrStdout(), response.GetJobs())
			return nil
		})
	},
}

func printJobHistory(output io.Writer, jobs []*rpc.Job) {
	table := newTableWriter(output)
	fmt.Fprintln(table, "JOB ID\tTYPE\tSTATUS\tDATE\tERROR")
	for _, job := range jobs {
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\n",
			job.GetId(),
			emptyAs(job.GetType(), "job"),
			emptyAs(job.GetStatus(), "unknown"),
			formatJobDate(job),
			job.GetError(),
		)
	}
	_ = table.Flush()
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
