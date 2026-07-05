package docker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"deployctl/internal/envfile"
	"deployctl/internal/store"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	containerpkg "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ConnectionStatus struct {
	Connected     bool
	Host          string
	ServerVersion string
	APIVersion    string
	OSType        string
	Error         string
}

type Logger func(string)

type UnavailableError struct {
	Host string
	Err  error
}

func (e *UnavailableError) Error() string {
	if e.Host == "" {
		return "Docker is unavailable: is the Docker daemon running?"
	}
	return fmt.Sprintf("Docker is unavailable at %s: is the Docker daemon running?", e.Host)
}

func (e *UnavailableError) Unwrap() error {
	return e.Err
}

func IsUnavailable(err error) bool {
	var unavailable *UnavailableError
	return errors.As(err, &unavailable)
}

func UnavailableMessage(err error) (string, bool) {
	var unavailable *UnavailableError
	if !errors.As(err, &unavailable) {
		return "", false
	}
	return unavailable.Error(), true
}

type BuildCache struct {
	Tags    []string
	Missing []string
}

type DeploymentStatus struct {
	Containers []ContainerStatus
	Missing    []string
}

type ContainerStatus struct {
	Service    string
	Name       string
	Status     string
	State      string
	Image      string
	ImageID    string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}

func (s DeploymentStatus) AllRunning() bool {
	return len(s.Containers) > 0 && len(s.Missing) == 0
}

func (s DeploymentStatus) AnyRunning() bool {
	for _, container := range s.Containers {
		if container.State == string(containerpkg.StateRunning) {
			return true
		}
	}
	return false
}

func (s DeploymentStatus) Summary() string {
	parts := make([]string, 0, len(s.Containers))
	for _, container := range s.Containers {
		if container.State != string(containerpkg.StateRunning) {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", container.Name, container.Status))
	}

	return strings.Join(parts, ", ")
}

func CheckConnection(ctx context.Context) ConnectionStatus {
	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return ConnectionStatus{Error: fmt.Sprintf("create Docker CLI: %v", err)}
	}

	if err := dockerCLI.Initialize(&flags.ClientOptions{}); err != nil {
		return ConnectionStatus{Error: fmt.Sprintf("initialize Docker CLI: %v", err)}
	}

	host := dockerCLI.Client().DaemonHost()
	ping, err := dockerCLI.Client().Ping(ctx, client.PingOptions{})
	if err != nil {
		if unavailable := dockerUnavailableError(dockerCLI, err); unavailable != nil {
			return ConnectionStatus{
				Host:  host,
				Error: unavailable.Error(),
			}
		}
		return ConnectionStatus{
			Host:  host,
			Error: fmt.Sprintf("ping Docker daemon: %v", err),
		}
	}

	version, err := dockerCLI.Client().ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return ConnectionStatus{
			Connected:  true,
			Host:       host,
			APIVersion: ping.APIVersion,
			OSType:     ping.OSType,
			Error:      fmt.Sprintf("read Docker server version: %v", err),
		}
	}

	return ConnectionStatus{
		Connected:     true,
		Host:          host,
		ServerVersion: version.Version,
		APIVersion:    firstNonEmpty(version.APIVersion, ping.APIVersion),
		OSType:        firstNonEmpty(version.Os, ping.OSType),
	}
}

func ComposeUp(ctx context.Context, repository *store.Repository, log Logger) error {
	service, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	logf(log, "Starting Compose project")
	if err := service.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{},
		Start:  api.StartOptions{},
	}); err != nil {
		return composeOperationError(dockerCLI, "start compose project", err)
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
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
		return DeploymentStatus{}, composeOperationError(dockerCLI, "list compose containers", err)
	}

	var status DeploymentStatus
	running := make(map[string]bool)

	for _, item := range containers.Items {
		serviceName := item.Labels[api.ServiceLabel]
		if serviceName == "" {
			continue
		}

		if item.State == containerpkg.StateRunning {
			running[serviceName] = true
		}

		startedAt, finishedAt := inspectContainerTimes(ctx, dockerCLI, item.ID)
		status.Containers = append(status.Containers, ContainerStatus{
			Service:    serviceName,
			Name:       containerName(item.Names),
			Status:     item.Status,
			State:      string(item.State),
			Image:      item.Image,
			ImageID:    item.ImageID,
			CreatedAt:  fromUnix(item.Created),
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
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

func inspectContainerTimes(ctx context.Context, dockerCLI *command.DockerCli, id string) (time.Time, time.Time) {
	inspected, err := dockerCLI.Client().ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil || inspected.Container.State == nil {
		return time.Time{}, time.Time{}
	}

	startedAt, _ := time.Parse(time.RFC3339Nano, inspected.Container.State.StartedAt)
	finishedAt, _ := time.Parse(time.RFC3339Nano, inspected.Container.State.FinishedAt)
	return startedAt, finishedAt
}

func fromUnix(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

func ComposeBuild(ctx context.Context, repository *store.Repository, log Logger) error {
	service, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	logf(log, "Building Compose images")
	if err := service.Build(ctx, project, api.BuildOptions{}); err != nil {
		return composeOperationError(dockerCLI, "build compose project", err)
	}

	return nil
}

func EnsureBuildAvailable(ctx context.Context, repository *store.Repository, log Logger) error {
	cache, err := ComposeBuildCache(ctx, repository)
	if err != nil {
		return err
	}
	if len(cache.Tags) > 0 && len(cache.Missing) == 0 {
		logf(log, "Using cached build: %s", strings.Join(cache.Tags, ", "))
	}
	if len(cache.Missing) > 0 {
		logf(log, "No cached build found for %s. Building now.", strings.Join(cache.Missing, ", "))
		return ComposeBuild(ctx, repository, log)
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
			return BuildCache{}, composeOperationError(dockerCLI, fmt.Sprintf("inspect build image %s", tag), err)
		}
	}

	sort.Strings(cache.Tags)
	sort.Strings(cache.Missing)
	return cache, nil
}

func ComposeDown(ctx context.Context, repository *store.Repository, log Logger) error {
	service, project, dockerCLI, err := loadProject(ctx, repository)
	if err != nil {
		return err
	}

	logf(log, "Stopping Compose project")
	if err := service.Down(ctx, project.Name, api.DownOptions{
		Project: project,
	}); err != nil {
		return composeOperationError(dockerCLI, "stop compose project", err)
	}

	return nil
}

func logf(log Logger, format string, args ...any) {
	if log == nil {
		return
	}
	log(fmt.Sprintf(format, args...))
}

func composeOperationError(dockerCLI *command.DockerCli, operation string, err error) error {
	if unavailable := dockerUnavailableError(dockerCLI, err); unavailable != nil {
		return unavailable
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func loadProject(ctx context.Context, repository *store.Repository) (api.Compose, *types.Project, *command.DockerCli, error) {
	if repository.ComposePath == "" {
		return nil, nil, nil, errors.New("repository does not have a compose file configured")
	}

	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create docker CLI: %w", err)
	}

	if err := dockerCLI.Initialize(&flags.ClientOptions{}); err != nil {
		return nil, nil, nil, fmt.Errorf("initialize docker CLI: %w", err)
	}

	service, err := compose.NewComposeService(dockerCLI)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create compose service: %w", err)
	}

	loadOptions := api.ProjectLoadOptions{
		ConfigPaths: []string{repository.ComposePath},
		WorkingDir:  repository.Location,
		ProjectName: repository.Name,
		ProjectOptionsFns: []composecli.ProjectOptionsFn{
			composecli.WithLoadOptions(func(options *loader.Options) {
				options.SkipResolveEnvironment = true
			}),
		},
	}
	if repository.EnvPath != "" {
		if err := mirrorDefaultEnvFile(repository); err != nil {
			return nil, nil, nil, err
		}

		loadOptions.EnvFiles = []string{repository.EnvPath}
		loadOptions.ProjectOptionsFns = append(loadOptions.ProjectOptionsFns,
			composecli.WithEnv([]string{"DEPLOYCTL_ENV_FILE=" + repository.EnvPath}),
		)
	}

	project, err := service.LoadProject(ctx, loadOptions)
	if err != nil {
		if unavailable := dockerUnavailableError(dockerCLI, err); unavailable != nil {
			return nil, nil, nil, unavailable
		}
		return nil, nil, nil, fmt.Errorf("load compose project: %w", err)
	}
	project, err = resolveServiceEnvFiles(project)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load compose project: %w", err)
	}

	return service, project, dockerCLI, nil
}

func resolveServiceEnvFiles(project *types.Project) (*types.Project, error) {
	for i, service := range project.Services {
		for j, envFile := range service.EnvFiles {
			if _, err := os.Stat(envFile.Path); err == nil {
				continue
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			service.EnvFiles[j].Required = types.OptOut(false)
		}
		project.Services[i] = service
	}
	return project.WithServicesEnvironmentResolved(false)
}

func containerName(names []string) string {
	if len(names) == 0 {
		return "unknown"
	}

	return strings.TrimPrefix(names[0], "/")
}

func dockerUnavailableError(dockerCLI *command.DockerCli, err error) *UnavailableError {
	if err == nil || !isDockerConnectionError(err) {
		return nil
	}

	host := ""
	if dockerCLI != nil && dockerCLI.Client() != nil {
		host = dockerCLI.Client().DaemonHost()
	}
	return &UnavailableError{Host: host, Err: err}
}

func isDockerConnectionError(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ENOTCONN) {
		return true
	}

	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"cannot connect to the docker daemon",
		"failed to connect to the docker api",
		"is the docker daemon running",
		"connection refused",
		"connect: no such file or directory",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
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
