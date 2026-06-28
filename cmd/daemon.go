package cmd

import (
	"fmt"

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
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.System.Health(cmd.Context(), &rpc.HealthRequest{})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deployctld %s\n", response.Status)
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd, daemonStatusCmd)
	daemonStartCmd.Flags().String("socket", "", "Unix socket path")
}
