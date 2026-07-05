package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"deployctl/internal"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/status"
)

var rootCmd = &cobra.Command{
	Use:     "deployctl",
	Short:   "A deployment control CLI",
	Version: internal.GitCommit(),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "deployctl")
	},
}

func init() {
	rootCmd.SetVersionTemplate("deployctl build: git commit {{.Version}}\n")
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}

func Execute() {
	if _, err := executeRootCommand(os.Stderr); err != nil {
		os.Exit(1)
	}
}

func executeRootCommand(errorOutput io.Writer) (*cobra.Command, error) {
	command, err := rootCmd.ExecuteC()
	if err == nil {
		return command, nil
	}

	printCommandError(errorOutput, command, err)
	return command, err
}

func printCommandError(output io.Writer, command *cobra.Command, err error) {
	if isUsageError(err) && command != nil {
		fmt.Fprintf(output, "Error: %s\n\n", err)
		fmt.Fprint(output, command.UsageString())
		return
	}

	fmt.Fprintln(output, formatCommandError(err))
}

func formatCommandError(err error) string {
	if grpcStatus, ok := status.FromError(err); ok {
		return grpcStatus.Message()
	}
	return err.Error()
}

func isUsageError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"accepts ",
		"requires ",
		"unknown command",
		"unknown flag",
		"unknown shorthand flag",
		"required flag",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
