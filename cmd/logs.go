package cmd

import (
	"errors"
	"fmt"
	"io"
	"math"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

const defaultLogLines = 100

var logsCmd = &cobra.Command{
	Use:               "logs [deployment-name]",
	Short:             "Show deployment logs",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		deploymentName, err := deploymentNameArg(args)
		if err != nil {
			return err
		}

		follow, err := cmd.Flags().GetBool("follow")
		if err != nil {
			return err
		}
		lines, err := cmd.Flags().GetInt("lines")
		if err != nil {
			return err
		}
		if lines < 0 {
			return errors.New("lines must be greater than or equal to 0")
		}
		if lines > math.MaxInt32 {
			return fmt.Errorf("lines must be less than or equal to %d", math.MaxInt32)
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			stream, err := client.Deployment.StreamDeploymentLogs(cmd.Context(), &rpc.StreamDeploymentLogsRequest{
				DeploymentName: deploymentName,
				Follow:         follow,
				Lines:          int32(lines),
			})
			if err != nil {
				return err
			}
			return printDeploymentLogs(cmd.OutOrStdout(), stream, follow)
		})
	},
}

type deploymentLogStream interface {
	Recv() (*rpc.DeploymentLogEntry, error)
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().IntP("lines", "n", defaultLogLines, "Number of lines to show from the end of the logs")
}

func printDeploymentLogs(output io.Writer, stream deploymentLogStream, follow bool) error {
	entries := 0
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			if !follow {
				if entries == 0 {
					fmt.Fprintln(output, "No logs found.")
				}
				fmt.Fprintln(output)
				fmt.Fprintln(output, "Use --follow for live logs or --lines N for more history.")
			}
			return nil
		}
		if err != nil {
			return err
		}
		entries++
		printDeploymentLogEntry(output, entry)
	}
}

func printDeploymentLogEntry(output io.Writer, entry *rpc.DeploymentLogEntry) {
	container := emptyAs(entry.GetContainer(), "container")
	fmt.Fprintf(output, "%s | %s\n", container, entry.GetMessage())
}
