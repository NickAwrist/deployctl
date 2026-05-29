package docker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"deployctl/internal/envfile"
	"deployctl/internal/store"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
)

func ComposeUp(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	// Start the project
	if err := service.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{
			Build: &api.BuildOptions{},
		},
		Start: api.StartOptions{},
	}); err != nil {
		return fmt.Errorf("start compose project: %w", err)
	}

	return nil
}

func ComposeBuild(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	// Rebuild project images
	if err := service.Build(ctx, project, api.BuildOptions{}); err != nil {
		return fmt.Errorf("build compose project: %w", err)
	}

	return nil
}

func ComposeDown(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	// Stop the project
	if err := service.Down(ctx, project.Name, api.DownOptions{
		Project: project,
	}); err != nil {
		return fmt.Errorf("stop compose project: %w", err)
	}

	return nil
}

func loadProject(ctx context.Context, repository *store.Repository) (api.Compose, *types.Project, error) {
	// Check if the repository has a compose file configured
	if repository.ComposePath == "" {
		return nil, nil, errors.New("repository does not have a compose file configured")
	}

	// Create a new docker CLI
	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return nil, nil, fmt.Errorf("create docker CLI: %w", err)
	}

	// Initialize the docker CLI
	if err := dockerCLI.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, nil, fmt.Errorf("initialize docker CLI: %w", err)
	}

	// Create a new compose service
	service, err := compose.NewComposeService(dockerCLI)
	if err != nil {
		return nil, nil, fmt.Errorf("create compose service: %w", err)
	}

	// Load the project
	loadOptions := api.ProjectLoadOptions{
		ConfigPaths: []string{repository.ComposePath},
		WorkingDir:  repository.Location,
		ProjectName: repository.Name,
	}
	if repository.EnvPath != "" {
		if err := mirrorDefaultEnvFile(repository); err != nil {
			return nil, nil, err
		}

		loadOptions.EnvFiles = []string{repository.EnvPath}
		loadOptions.ProjectOptionsFns = []composecli.ProjectOptionsFn{
			composecli.WithEnv([]string{"DEPLOYCTL_ENV_FILE=" + repository.EnvPath}),
		}
	}

	project, err := service.LoadProject(ctx, loadOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("load compose project: %w", err)
	}

	return service, project, nil
}

func mirrorDefaultEnvFile(repository *store.Repository) error {
	defaultEnvPath := filepath.Join(repository.Location, ".env")
	if repository.EnvPath == defaultEnvPath {
		return nil
	}

	variables, err := envfile.Read(repository.EnvPath)
	if err != nil {
		return fmt.Errorf("read deployctl env file: %w", err)
	}
	if err := envfile.Write(defaultEnvPath, variables); err != nil {
		return fmt.Errorf("prepare compose env file: %w", err)
	}

	return nil
}
