package cmd

import (
	"fmt"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl list

Lists all deployments.
*/
var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all deployments",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.ListDeployments(cmd.Context(), &rpc.ListDeploymentsRequest{})
			if err != nil {
				return err
			}
			if len(response.Deployments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No deployments found")
				return nil
			}
			for _, deployment := range response.Deployments {
				composePath := deployment.ComposePath
				if composePath == "" {
					composePath = "none"
				}
				envPath := deployment.EnvPath
				if envPath == "" {
					envPath = "none"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s:\n- %s\n- %s\n- %s\n- %s\n", deployment.Name, deployment.Url, deployment.Location, composePath, envPath)
			}
			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
