package cmd

import (
	"fmt"
	"os/exec"
	"strings"

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
			socketPath = internal.GetSocketPath()
		}

		listener, err := service.ListenUnix(socketPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deployctld listening on %s\n", socketPath)
		return service.NewServer().Serve(listener)
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
			fmt.Fprintln(cmd.OutOrStdout(), "Daemon")
			fmt.Fprintln(cmd.OutOrStdout(), "  Status: reachable")
			fmt.Fprintf(cmd.OutOrStdout(), "  Socket: %s\n\n", internal.GetSocketPath())
			if strings.TrimSpace(response.Status) == "ok" {
				fmt.Fprintln(cmd.OutOrStdout(), "Health")
				fmt.Fprintln(cmd.OutOrStdout(), "  Status: ok")
				return nil
			}

			fmt.Fprint(cmd.OutOrStdout(), response.Status)
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

func restartDaemonService(cmd *cobra.Command, useUser bool, useSystem bool) (string, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return "", fmt.Errorf("restart requires the systemd service; start a foreground daemon with: deployctl daemon start")
	}

	candidates := []systemdScope{}
	switch {
	case useUser:
		candidates = []systemdScope{userSystemdScope}
	case useSystem:
		candidates = []systemdScope{systemSystemdScope}
	default:
		candidates = []systemdScope{userSystemdScope, systemSystemdScope}
	}

	var skipped []string
	for _, candidate := range candidates {
		loaded, detail := systemdServiceLoaded(cmd, candidate)
		if !loaded {
			if detail != "" {
				skipped = append(skipped, fmt.Sprintf("%s service: %s", candidate.name, detail))
			}
			continue
		}

		if err := runSystemctl(cmd, candidate, "restart"); err != nil {
			return "", err
		}
		return candidate.name, nil
	}

	if useUser || useSystem {
		return "", fmt.Errorf("deployctld.service is not loaded as a systemd %s service", candidates[0].name)
	}
	if len(skipped) > 0 {
		return "", fmt.Errorf("deployctld.service is not loaded under systemd (%s)", strings.Join(skipped, "; "))
	}
	return "", fmt.Errorf("deployctld.service is not loaded under systemd")
}

type systemdScope struct {
	name string
	args []string
}

var (
	userSystemdScope   = systemdScope{name: "user", args: []string{"--user"}}
	systemSystemdScope = systemdScope{name: "system"}
)

func systemdServiceLoaded(cmd *cobra.Command, scope systemdScope) (bool, string) {
	args := append([]string{}, scope.args...)
	args = append(args, "show", "deployctld.service", "--property=LoadState", "--value")
	output, err := exec.CommandContext(cmd.Context(), "systemctl", args...).CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(output))
	}

	return strings.TrimSpace(string(output)) == "loaded", ""
}

func runSystemctl(cmd *cobra.Command, scope systemdScope, action string) error {
	args := append([]string{}, scope.args...)
	args = append(args, action, "deployctld.service")
	output, err := exec.CommandContext(cmd.Context(), "systemctl", args...).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("systemctl %s %s deployctld.service: %w\n%s", strings.Join(scope.args, " "), action, err, detail)
		}
		return fmt.Errorf("systemctl %s deployctld.service: %w", action, err)
	}

	return nil
}
