package cmd

import (
	"deployctl/internal"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		repositories := store.NewRepositoryStore()
		repos, err := repositories.GetAll(cmd.Context())
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			internal.Warning("No deployments found")
			return nil
		}
		for _, repo := range repos {
			internal.Info("%s: \n-%s\n-%s", repo.Name, repo.URL, repo.Location)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
