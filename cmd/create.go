package cmd

import (
	"errors"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl create <repo-url> [--name <name>] [--compose-file <compose-file>]

Creates a new deployment from a repository URL.

Arguments:

	<repo-url>   The URL of the repository to create a new deployment from
	--name <name> The name of the deployment
	--compose-file <compose-file> The Compose file name in the repository or a local Compose file path to copy into the repository
	--env-file <env-file> The env file name in the repository or a local env file path to copy into the repository
*/
var createCmd = &cobra.Command{
	Use:   "create [repo-url]",
	Short: "Create a new deployment",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoURL, err := cmd.Flags().GetString("repo-url")
		if err != nil {
			return err
		}
		if repoURL == "" && len(args) > 0 {
			repoURL = args[0]
		}
		if repoURL == "" {
			return errors.New("repo URL is required")
		}

		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}

		composeFile, err := cmd.Flags().GetString("compose-file")
		if err != nil {
			return err
		}

		envFile, err := cmd.Flags().GetString("env-file")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.CreateDeployment(cmd.Context(), &rpc.CreateDeploymentRequest{
				RepoUrl:        repoURL,
				DeploymentName: name,
				ComposeFile:    composeFile,
				EnvFile:        envFile,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, deploymentSuccess(name, "created"))
		})
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringP("name", "n", "", "The name of the deployment")
	createCmd.Flags().StringP("repo-url", "r", "", "The URL of the repository to create a new deployment from")
	createCmd.Flags().String("compose-file", "", "The Compose file name in the repository or a local Compose file path to copy into the repository")
	createCmd.Flags().String("env-file", "", "The env file name in the repository or a local env file path to copy into the repository")
	addJobFlags(createCmd)
}
