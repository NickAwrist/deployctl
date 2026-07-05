package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:               "status [repository-name]",
	Short:             "Show deployment status",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		envFile, err := cmd.Flags().GetString("env-file")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.GetDeploymentStatus(cmd.Context(), &rpc.GetDeploymentStatusRequest{
				DeploymentName: repositoryName,
				EnvFile:        envFile,
			})
			if err != nil {
				return err
			}
			printDeploymentStatus(cmd.OutOrStdout(), response)
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().String("env-file", "", "Show masked names from a specific env file")
}

func printDeploymentStatus(output io.Writer, status *rpc.DeploymentStatus) {
	deployment := status.GetDeployment()
	fmt.Fprintf(output, "Deployment: %s\n", deployment.GetName())
	fmt.Fprintf(output, "State: %s\n", status.GetState())
	fmt.Fprintf(output, "Repository: %s\n", emptyAs(deployment.GetUrl(), "unknown"))
	fmt.Fprintf(output, "Location: %s\n", emptyAs(deployment.GetLocation(), "unknown"))
	fmt.Fprintf(output, "Compose file: %s\n", emptyAs(deployment.GetComposePath(), "none"))
	fmt.Fprintf(output, "Env file: %s\n", emptyAs(deployment.GetEnvPath(), "none"))

	if status.GetRunningSinceUnix() > 0 {
		fmt.Fprintf(output, "Running since: %s\n", formatUnixTime(status.GetRunningSinceUnix()))
		fmt.Fprintf(output, "Running for: %s\n", formatDuration(time.Since(time.Unix(status.GetRunningSinceUnix(), 0))))
	}
	if status.GetStoppedAtUnix() > 0 {
		fmt.Fprintf(output, "Last stopped: %s\n", formatUnixTime(status.GetStoppedAtUnix()))
	}
	if status.GetLatestUpdateJob() != nil {
		fmt.Fprintf(output, "Latest update: %s\n", formatJob(status.GetLatestUpdateJob()))
	}
	if status.GetLatestJob() != nil {
		fmt.Fprintf(output, "Latest job: %s\n", formatJob(status.GetLatestJob()))
	}

	fmt.Fprintln(output, "Containers:")
	if len(status.GetContainers()) == 0 {
		fmt.Fprintln(output, "  none")
	} else {
		for _, container := range status.GetContainers() {
			fmt.Fprintf(output, "  %s: %s\n", container.GetService(), emptyAs(container.GetName(), "unnamed"))
			fmt.Fprintf(output, "    State: %s\n", emptyAs(container.GetState(), "unknown"))
			fmt.Fprintf(output, "    Status: %s\n", emptyAs(container.GetStatus(), "unknown"))
			fmt.Fprintf(output, "    Image: %s\n", emptyAs(container.GetImage(), "unknown"))
			fmt.Fprintf(output, "    Image ID: %s\n", emptyAs(container.GetImageId(), "unknown"))
			if container.GetStartedAtUnix() > 0 {
				fmt.Fprintf(output, "    Started: %s\n", formatUnixTime(container.GetStartedAtUnix()))
			}
			if container.GetFinishedAtUnix() > 0 {
				fmt.Fprintf(output, "    Finished: %s\n", formatUnixTime(container.GetFinishedAtUnix()))
			}
			if container.GetCreatedAtUnix() > 0 {
				fmt.Fprintf(output, "    Created: %s\n", formatUnixTime(container.GetCreatedAtUnix()))
			}
		}
	}

	if len(status.GetMissingServices()) > 0 {
		fmt.Fprintln(output, "Missing or stopped services:")
		for _, service := range status.GetMissingServices() {
			fmt.Fprintf(output, "  %s\n", service)
		}
	}

	fmt.Fprintln(output, "Env:")
	if len(status.GetEnvNames()) == 0 {
		fmt.Fprintln(output, "  none")
		return
	}
	for _, name := range status.GetEnvNames() {
		fmt.Fprintf(output, "  %s=*****\n", name)
	}
}

func formatJob(job *rpc.Job) string {
	parts := []string{emptyAs(job.GetType(), "job"), emptyAs(job.GetStatus(), "unknown")}
	if at := jobTimestamp(job); at > 0 {
		parts = append(parts, "at "+formatUnixTime(at))
	}
	if job.GetError() != "" {
		parts = append(parts, "error: "+job.GetError())
	}
	return strings.Join(parts, " ")
}

func jobTimestamp(job *rpc.Job) int64 {
	if job.GetFinishedAtUnix() > 0 {
		return job.GetFinishedAtUnix()
	}
	if job.GetStartedAtUnix() > 0 {
		return job.GetStartedAtUnix()
	}
	return job.GetCreatedAtUnix()
}

func formatUnixTime(seconds int64) string {
	return time.Unix(seconds, 0).Local().Format("2006-01-02 15:04:05 MST")
}

func formatOptionalUnixTime(seconds int64, fallback string) string {
	if seconds == 0 {
		return fallback
	}
	return formatUnixTime(seconds)
}

func formatJobDate(job *rpc.Job) string {
	return formatOptionalUnixTime(jobTimestamp(job), "unknown")
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	duration = duration.Round(time.Second)
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func emptyAs(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
