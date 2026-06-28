package cmd

import (
	"strings"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

func completeDeploymentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, err := dialClient(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	defer client.Close()

	response, err := client.Deployment.ListDeployments(cmd.Context(), &rpc.ListDeploymentsRequest{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := make([]string, 0, len(response.Deployments))
	for _, deployment := range response.Deployments {
		if strings.HasPrefix(deployment.Name, toComplete) {
			matches = append(matches, deployment.Name)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}
