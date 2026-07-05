package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

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
