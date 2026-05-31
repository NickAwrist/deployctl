package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"deployctl/internal"
	"deployctl/internal/docker"
	"deployctl/internal/store"

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
		// Get the repository name from the command line arguments or flags
		repositoryName := ""
		if len(args) > 0 {
			repositoryName = args[0]
			if repositoryName == "" {
				return errors.New("repository name is required")
			}
		}

		// Get the repository from the database
		repositories := store.NewRepositoryStore()
		repository, err := repositories.Get(cmd.Context(), repositoryName)
		if err != nil {
			return err
		}

		build, err := cmd.Flags().GetBool("build")
		if err != nil {
			return err
		}

		if !build {
			status, err := docker.ComposeStatus(cmd.Context(), &repository)
			if err != nil {
				return err
			}
			if status.AllRunning() {
				internal.Info("Deployment already running: %s", status.Summary())
				return nil
			}
		}

		ready, err := prepareDeploymentBuild(cmd.InOrStdin(), cmd.Context(), &repository, build)
		if err != nil {
			return err
		}
		if !ready {
			return nil
		}

		// Deploy the repository
		err = docker.ComposeUp(cmd.Context(), &repository)
		if err != nil {
			return err
		}

		internal.Info("Deployment deployed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	deployCmd.Flags().Bool("build", false, "Build deployment images before starting")
}

func prepareDeploymentBuild(input io.Reader, ctx context.Context, repository *store.Repository, build bool) (bool, error) {
	if build {
		if err := docker.ComposeBuild(ctx, repository); err != nil {
			return false, err
		}
		return true, nil
	}

	cache, err := docker.ComposeBuildCache(ctx, repository)
	if err != nil {
		return false, err
	}
	if len(cache.Tags) == 0 {
		return true, nil
	}
	if len(cache.Missing) == 0 {
		internal.Info("Using cached build: %s", strings.Join(cache.Tags, ", "))
		return true, nil
	}

	confirmed, err := confirmBuild(input, cache.Missing)
	if err != nil {
		return false, err
	}
	if !confirmed {
		internal.Warning("Deployment start cancelled")
		return false, nil
	}

	if err := docker.ComposeBuild(ctx, repository); err != nil {
		return false, err
	}
	return true, nil
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
