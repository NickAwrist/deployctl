package cmd

import (
	"fmt"
	"io"

	"deployctl/internal"
	"deployctl/internal/rpc"
	"deployctl/internal/service"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the deployctl daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start deployctld in the foreground",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		socketPath, err := cmd.Flags().GetString("socket")
		if err != nil {
			return err
		}
		if socketPath == "" {
			socketPath, err = internal.SocketPath()
			if err != nil {
				return err
			}
		}

		listener, err := service.ListenUnix(socketPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deployctld listening on %s\n", socketPath)
		logger, err := service.NewDaemonLogger()
		if err != nil {
			_ = listener.Close()
			return err
		}
		server, err := service.NewServerWithLogger(logger)
		if err != nil {
			_ = listener.Close()
			return err
		}
		defer server.Close()
		return server.Serve(listener)
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check deployctld health",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.System.Health(cmd.Context(), &rpc.HealthRequest{})
			if err != nil {
				return err
			}
			socketPath, err := internal.SocketPath()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Daemon")
			fmt.Fprintln(cmd.OutOrStdout(), "  Status: reachable")
			fmt.Fprintf(cmd.OutOrStdout(), "  Socket: %s\n\n", socketPath)
			printDockerHealth(cmd.OutOrStdout(), response.GetDocker())
			return nil
		})
	},
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the deployctl daemon service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		useUser, err := cmd.Flags().GetBool("user")
		if err != nil {
			return err
		}
		useSystem, err := cmd.Flags().GetBool("system")
		if err != nil {
			return err
		}
		if useUser && useSystem {
			return fmt.Errorf("choose either --user or --system")
		}

		scope, err := restartDaemonService(cmd, useUser, useSystem)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "deployctld restart requested via systemd %s service\n", scope)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd, daemonStatusCmd, daemonRestartCmd)
	daemonStartCmd.Flags().String("socket", "", "Unix socket path")
	daemonRestartCmd.Flags().Bool("user", false, "Restart the user systemd service")
	daemonRestartCmd.Flags().Bool("system", false, "Restart the system systemd service")
}

func printDockerHealth(output io.Writer, health *rpc.DockerHealth) {
	fmt.Fprintln(output, "Docker")
	fmt.Fprintf(output, "  Status: %s\n", formatDockerConnectionState(health.GetState()))
	if health.GetHost() != "" {
		fmt.Fprintf(output, "  Host: %s\n", health.GetHost())
	}
	if health.GetServerVersion() != "" {
		fmt.Fprintf(output, "  Server version: %s\n", health.GetServerVersion())
	}
	if health.GetApiVersion() != "" {
		fmt.Fprintf(output, "  API version: %s\n", health.GetApiVersion())
	}
	if health.GetOsType() != "" {
		fmt.Fprintf(output, "  OS type: %s\n", health.GetOsType())
	}
	if health.GetError() != "" {
		fmt.Fprintf(output, "  Error: %s\n", health.GetError())
	}
}

func formatDockerConnectionState(state rpc.DockerConnectionState) string {
	switch state {
	case rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_CONNECTED:
		return "connected"
	case rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_PARTIALLY_CONNECTED:
		return "partially connected"
	case rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_UNAVAILABLE:
		return "unavailable"
	default:
		return "unknown"
	}
}
