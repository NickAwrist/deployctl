package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"deployctl/internal/rpc"

	"github.com/spf13/cobra"
)

/*
deployctl deploy <repository-name>

Deploys a deployment. Use --build to rebuild images before starting.

Arguments:

	<repository-name> The name of the deployment to deploy
*/
var deployCmd = &cobra.Command{
	Use:               "deploy [repository-name]",
	Short:             "Deploy a deployment",
	Aliases:           []string{"start"},
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeDeploymentNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		repositoryName := args[0]
		if repositoryName == "" {
			return errors.New("repository name is required")
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		return runWithClient(cmd, func(client *daemonClient) error {
			response, err := client.Deployment.DeployDeployment(cmd.Context(), &rpc.DeployDeploymentRequest{
				Name:  repositoryName,
				Build: build,
			})
			if err != nil {
				return err
			}
			return handleJob(cmd, client, response, "Deployment deployed successfully")
		})
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().Bool("build", false, "Build deployment images before starting")
	addJobFlags(deployCmd)
}

func confirmBuild(input io.Reader, missingTags []string) (bool, error) {
	fmt.Fprintf(os.Stdout, "No cached build found for %s. Build now? (Y/n) ", strings.Join(missingTags, ", "))

	reader := bufio.NewReader(input)
	answer, err := reader.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && answer != "") {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
}
