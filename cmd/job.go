package cmd

import (
	"errors"
	"fmt"
	"io"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "job [job-id]",
	Short: "Show job details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]
		if jobID == "" {
			return errors.New("job ID is required")
		}
		includeLogs, err := cmd.Flags().GetBool("logs")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			job, err := client.Job.GetJob(cmd.Context(), &rpc.GetJobRequest{Id: jobID})
			if err != nil {
				return err
			}
			printJobDetails(cmd.OutOrStdout(), job)
			if !includeLogs {
				return nil
			}
			response, err := client.Job.ListJobLogs(cmd.Context(), &rpc.ListJobLogsRequest{Id: jobID})
			if err != nil {
				return err
			}
			printJobLogs(cmd.OutOrStdout(), response.GetLogs())
			return nil
		})
	},
}

func printJobDetails(output io.Writer, job *rpc.Job) {
	fmt.Fprintf(output, "Job: %s\n", job.GetId())
	fmt.Fprintf(output, "Type: %s\n", emptyAs(job.GetType(), "job"))
	fmt.Fprintf(output, "Deployment: %s\n", emptyAs(job.GetDeploymentName(), "none"))
	fmt.Fprintf(output, "Status: %s\n", emptyAs(job.GetStatus(), "unknown"))
	fmt.Fprintf(output, "Created: %s\n", formatOptionalUnixTime(job.GetCreatedAtUnix(), "unknown"))
	fmt.Fprintf(output, "Started: %s\n", formatOptionalUnixTime(job.GetStartedAtUnix(), "not started"))
	fmt.Fprintf(output, "Finished: %s\n", formatOptionalUnixTime(job.GetFinishedAtUnix(), "not finished"))
	if job.GetError() != "" {
		fmt.Fprintf(output, "Error: %s\n", job.GetError())
	}
}

func printJobLogs(output io.Writer, logs []*rpc.JobLog) {
	fmt.Fprintln(output, "Logs:")
	if len(logs) == 0 {
		fmt.Fprintln(output, "  none")
		return
	}
	for _, log := range logs {
		fmt.Fprintln(output, log.GetMessage())
	}
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.Flags().Bool("logs", false, "Show stored job logs")
}
