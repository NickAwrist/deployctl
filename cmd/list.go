package cmd

import (
	"deployctl/internal"
	"deployctl/internal/store"

	"github.com/spf13/cobra"
)

/*
deployctl list

Lists all deployments.
*/
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get all repositories from the database
		repositories := store.NewRepositoryStore()
		repos, err := repositories.GetAll(cmd.Context())
		if err != nil {
			return err
		}

		// If no repositories are found, warn the user and return
		if len(repos) == 0 {
			internal.Warning("No deployments found")
			return nil
		}

		// List the repositories
		for _, repo := range repos {
			composePath := repo.ComposePath
			if composePath == "" {
				composePath = "none"
			}
			envPath := repo.EnvPath
			if envPath == "" {
				envPath = "none"
			}
			internal.Info("%s: \n-%s\n-%s\n-%s\n-%s", repo.Name, repo.URL, repo.Location, composePath, envPath)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
