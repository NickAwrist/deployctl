package docker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"deployctl/internal/envfile"
	"deployctl/internal/store"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type BuildCache struct {
	Tags    []string
	Missing []string
}

type DeploymentStatus struct {
	Containers []ContainerStatus
	Missing    []string
}

type ContainerStatus struct {
	Service string
	Name    string
	Status  string
	State   string
}

func (s DeploymentStatus) AllRunning() bool {
	return len(s.Containers) > 0 && len(s.Missing) == 0
}

func (s DeploymentStatus) Summary() string {
	parts := make([]string, 0, len(s.Containers))
	for _, container := range s.Containers {
		parts = append(parts, fmt.Sprintf("%s (%s)", container.Name, container.Status))
	}

	return strings.Join(parts, ", ")
}

func ComposeUp(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, _, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	// Start the project
	if err := service.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{},
		Start:  api.StartOptions{},
	}); err != nil {
		return fmt.Errorf("start compose project: %w", err)
	}

	return nil
}

func ComposeStatus(ctx context.Context, repository *store.Repository) (DeploymentStatus, error) {
	_, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return DeploymentStatus{}, err
	}

	containers, err := dockerCLI.Client().ContainerList(ctx, client.ContainerListOptions{
		All: true,
		Filters: make(client.Filters).
			Add("label", api.ProjectLabel+"="+project.Name).
			Add("label", api.ServiceLabel).
			Add("label", api.ContainerNumberLabel),
	})
	if err != nil {
		return DeploymentStatus{}, fmt.Errorf("list compose containers: %w", err)
	}

	var status DeploymentStatus
	running := make(map[string]bool)

	for _, item := range containers.Items {
		serviceName := item.Labels[api.ServiceLabel]
		if serviceName == "" || item.State != container.StateRunning {
			continue
		}

		running[serviceName] = true
		status.Containers = append(status.Containers, ContainerStatus{
			Service: serviceName,
			Name:    containerName(item.Names),
			Status:  item.Status,
			State:   string(item.State),
		})
	}

	for serviceName, service := range project.Services {
		if service.Provider == nil && !running[serviceName] {
			status.Missing = append(status.Missing, serviceName)
		}
	}

	slices.SortFunc(status.Containers, func(a, b ContainerStatus) int {
		return cmp.Or(
			cmp.Compare(a.Service, b.Service),
			cmp.Compare(a.Name, b.Name),
		)
	})

	sort.Strings(status.Missing)
	return status, nil
}

func ComposeBuild(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, _, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	// Rebuild project images
	if err := service.Build(ctx, project, api.BuildOptions{}); err != nil {
		return fmt.Errorf("build compose project: %w", err)
	}

	return nil
}

func ComposeBuildCache(ctx context.Context, repository *store.Repository) (BuildCache, error) {
	_, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return BuildCache{}, err
	}

	seen := map[string]struct{}{}
	var cache BuildCache
	for _, service := range project.Services {
		if service.Build == nil {
			continue
		}

		tag := api.GetImageNameOrDefault(service, project.Name)
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		cache.Tags = append(cache.Tags, tag)

		if _, err := dockerCLI.Client().ImageInspect(ctx, tag); err != nil {
			if cerrdefs.IsNotFound(err) {
				cache.Missing = append(cache.Missing, tag)
				continue
			}

			return BuildCache{}, fmt.Errorf("inspect build image %s: %w", tag, err)
		}
	}

	sort.Strings(cache.Tags)
	sort.Strings(cache.Missing)
	return cache, nil
}

func ComposeDown(ctx context.Context, repository *store.Repository) error {
	// Load the project
	service, project, _, err := loadProject(ctx, repository)
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

func loadProject(ctx context.Context, repository *store.Repository) (api.Compose, *types.Project, *command.DockerCli, error) {
	// Check if the repository has a compose file configured
	if repository.ComposePath == "" {
		return nil, nil, nil, errors.New("repository does not have a compose file configured")
	}

	// Create a new docker CLI
	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create docker CLI: %w", err)
	}

	// Initialize the docker CLI
	if err := dockerCLI.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, nil, nil, fmt.Errorf("initialize docker CLI: %w", err)
	}

	// Create a new compose service
	service, err := compose.NewComposeService(dockerCLI)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create compose service: %w", err)
	}

	// Load the project
	loadOptions := api.ProjectLoadOptions{
		ConfigPaths: []string{repository.ComposePath},
		WorkingDir:  repository.Location,
		ProjectName: repository.Name,
	}
	if repository.EnvPath != "" {
		if err := mirrorDefaultEnvFile(repository); err != nil {
			return nil, nil, nil, err
		}

		loadOptions.EnvFiles = []string{repository.EnvPath}
		loadOptions.ProjectOptionsFns = []composecli.ProjectOptionsFn{
			composecli.WithEnv([]string{"DEPLOYCTL_ENV_FILE=" + repository.EnvPath}),
		}
	}

	project, err := service.LoadProject(ctx, loadOptions)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load compose project: %w", err)
	}

	return service, project, dockerCLI, nil
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}

	return strings.TrimPrefix(names[0], "/")
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
