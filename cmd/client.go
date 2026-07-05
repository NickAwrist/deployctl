package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	deployclient "deployctl/internal/client"
	"deployctl/internal/rpc"

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
	stream, err := client.Job.WatchJob(cmd.Context(), &rpc.WatchJobRequest{JobId: jobID})
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
		if shouldPrintJobMessage(event.Message) {
			fmt.Fprintln(cmd.OutOrStdout(), event.Message)
		}
		if event.Job != nil {
			if event.Job.Status == rpc.JobStatus_JOB_STATUS_FAILED {
				return errors.New(event.Job.Error)
			}
			if event.Job.Status == rpc.JobStatus_JOB_STATUS_CANCELLED {
				return fmt.Errorf("job cancelled: %s", event.Job.Error)
			}
			return nil
		}
	}
}

func shouldPrintJobMessage(message string) bool {
	if message == "" {
		return false
	}
	return !strings.HasPrefix(message, "Failed: ") && !strings.HasPrefix(message, "Cancelled: ")
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

func deploymentNameArg(args []string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return "", errors.New("deployment name is required")
	}
	return args[0], nil
}

func runDeploymentJob(
	cmd *cobra.Command,
	args []string,
	success string,
	call func(*daemonClient, string) (*rpc.JobResponse, error),
) error {
	deploymentName, err := deploymentNameArg(args)
	if err != nil {
		return err
	}

	return runWithClient(cmd, func(client *daemonClient) error {
		response, err := call(client, deploymentName)
		if err != nil {
			return err
		}
		return handleJob(cmd, client, response, success)
	})
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

func printMaskedEnvFile(output io.Writer, envFile string, names []string) {
	fmt.Fprintln(output, envFile)
	printMaskedEnv(output, names)
}

func printEnvFiles(output io.Writer, deploymentName string, explicitFile bool, response *rpc.ListEnvFilesResponse) {
	if len(response.GetMissingEnvFiles()) > 0 {
		if explicitFile {
			fmt.Fprintln(output, "Warning: env file was not found:")
		} else {
			fmt.Fprintln(output, "Warning: compose references missing env files:")
		}
		for _, envFile := range response.GetMissingEnvFiles() {
			fmt.Fprintf(output, "  %s\n", envFile)
		}
	}
	if len(response.GetMissingEnvFiles()) > 0 {
		fmt.Fprintln(output)
	}
	if len(response.GetEnvFiles()) == 0 {
		fmt.Fprintf(output, "No env files found for %s\n", deploymentName)
		return
	}
	for i, envFile := range response.GetEnvFiles() {
		if i > 0 {
			fmt.Fprintln(output)
		}
		printMaskedEnvFile(output, envFile.GetEnvFile(), envFile.GetNames())
	}
}

func formatDeploymentState(state rpc.DeploymentState) string {
	switch state {
	case rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CONFIGURED:
		return "not_configured"
	case rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CREATED:
		return "not_created"
	case rpc.DeploymentState_DEPLOYMENT_STATE_RUNNING:
		return "running"
	case rpc.DeploymentState_DEPLOYMENT_STATE_PARTIAL:
		return "partial"
	case rpc.DeploymentState_DEPLOYMENT_STATE_STOPPED:
		return "stopped"
	default:
		return "unknown"
	}
}

func formatContainerState(state rpc.ContainerState) string {
	switch state {
	case rpc.ContainerState_CONTAINER_STATE_CREATED:
		return "created"
	case rpc.ContainerState_CONTAINER_STATE_RESTARTING:
		return "restarting"
	case rpc.ContainerState_CONTAINER_STATE_RUNNING:
		return "running"
	case rpc.ContainerState_CONTAINER_STATE_REMOVING:
		return "removing"
	case rpc.ContainerState_CONTAINER_STATE_PAUSED:
		return "paused"
	case rpc.ContainerState_CONTAINER_STATE_EXITED:
		return "exited"
	case rpc.ContainerState_CONTAINER_STATE_DEAD:
		return "dead"
	default:
		return "unknown"
	}
}

func formatJobType(jobType rpc.JobType) string {
	switch jobType {
	case rpc.JobType_JOB_TYPE_CREATE:
		return "create"
	case rpc.JobType_JOB_TYPE_DELETE:
		return "delete"
	case rpc.JobType_JOB_TYPE_BUILD:
		return "build"
	case rpc.JobType_JOB_TYPE_UPDATE:
		return "update"
	case rpc.JobType_JOB_TYPE_DEPLOY:
		return "deploy"
	case rpc.JobType_JOB_TYPE_RESTART:
		return "restart"
	case rpc.JobType_JOB_TYPE_STOP:
		return "stop"
	case rpc.JobType_JOB_TYPE_ENV_SET:
		return "env.set"
	case rpc.JobType_JOB_TYPE_ENV_IMPORT:
		return "env.import"
	case rpc.JobType_JOB_TYPE_ENV_UNSET:
		return "env.unset"
	default:
		return "job"
	}
}

func formatJobStatus(status rpc.JobStatus) string {
	switch status {
	case rpc.JobStatus_JOB_STATUS_QUEUED:
		return "queued"
	case rpc.JobStatus_JOB_STATUS_RUNNING:
		return "running"
	case rpc.JobStatus_JOB_STATUS_SUCCEEDED:
		return "succeeded"
	case rpc.JobStatus_JOB_STATUS_FAILED:
		return "failed"
	case rpc.JobStatus_JOB_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
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
