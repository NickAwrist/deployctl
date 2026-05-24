package cmd

import (
	"errors"

	"deployctl/internal"
	internalgit "deployctl/internal/git"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [repo-url]",
	Short: "Create a new deployment",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoURL, err := cmd.Flags().GetString("repo-url")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}
		if repoURL == "" && len(args) > 0 {
			repoURL = args[0]
		}
		if repoURL == "" {
			return errors.New("repo URL is required")
		}

		if err := internalgit.CloneRepo(repoURL, name); err != nil {
			return err
		}
		internal.Info("Deployment created successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringP("name", "n", "", "The name of the deployment")
	createCmd.Flags().StringP("repo-url", "r", "", "The URL of the repository to create a new deployment from")
}
