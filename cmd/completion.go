package cmd

import (
	"strings"

	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

func completeDeploymentNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	repositories := store.NewRepositoryStore()
	deployments, err := repositories.GetAll(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	matches := make([]string, 0, len(deployments))
	for _, deployment := range deployments {
		if strings.HasPrefix(deployment.Name, toComplete) {
			matches = append(matches, deployment.Name)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}
