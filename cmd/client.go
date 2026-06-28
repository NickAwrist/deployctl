package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	deployclient "deployctl/internal/client"
	"deployctl/internal/rpc"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

type daemonClient = deployclient.Client

func dialClient(ctx context.Context) (*deployclient.Client, error) {
	client, err := deployclient.DialDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w\n\nStart the daemon with: deployctl daemon start", err)
	}
	return client, nil
}

func waitForJob(cmd *cobra.Command, client *deployclient.Client, jobID string) error {
	stream, err := client.Job.WatchJob(cmd.Context(), &rpc.WatchJobRequest{Id: jobID})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if event.Message != "" {
			fmt.Fprintln(cmd.OutOrStdout(), event.Message)
		}
		if event.Job != nil {
			if event.Job.Status == store.JobStatusFailed {
				return errors.New(event.Job.Error)
			}
			if event.Job.Status == store.JobStatusCancelled {
				return fmt.Errorf("job cancelled: %s", event.Job.Error)
			}
			return nil
		}
	}
}

func handleJob(cmd *cobra.Command, client *deployclient.Client, response *rpc.JobResponse, success string) error {
	detach, err := cmd.Flags().GetBool("detach")
	if err != nil {
		return err
	}
	if detach {
		fmt.Fprintf(cmd.OutOrStdout(), "Started job %s\n", response.JobId)
		return nil
	}
	if err := waitForJob(cmd, client, response.JobId); err != nil {
		return err
	}
	if success != "" {
		fmt.Fprintln(cmd.OutOrStdout(), success)
	}
	return nil
}

func addJobFlags(command *cobra.Command) {
	command.Flags().Bool("detach", false, "Start the daemon job and return immediately")
}

func runWithClient(cmd *cobra.Command, fn func(*deployclient.Client) error) error {
	client, err := dialClient(cmd.Context())
	if err != nil {
		return err
	}
	defer client.Close()
	return fn(client)
}

func printMaskedEnv(output io.Writer, names []string) {
	for _, name := range names {
		fmt.Fprintf(output, "%s=*****\n", name)
	}
}

func parseAssignments(assignments []string) (map[string]string, error) {
	variables := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		name, value, ok := strings.Cut(assignment, "=")
		if !ok {
			return nil, fmt.Errorf("env variable %q must use KEY=VALUE", assignment)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("env variable %q must use KEY=VALUE", assignment)
		}
		variables[name] = value
	}
	return variables, nil
}
