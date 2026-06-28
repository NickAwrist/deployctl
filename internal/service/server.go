package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"deployctl/internal"
	"deployctl/internal/docker"
	"deployctl/internal/envfile"
	internalfile "deployctl/internal/file"
	internalgit "deployctl/internal/git"
	"deployctl/internal/rpc"
	"deployctl/internal/store"

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type Server struct {
	rpc.UnimplementedDeploymentServiceServer
	rpc.UnimplementedEnvServiceServer
	rpc.UnimplementedJobServiceServer
	rpc.UnimplementedSystemServiceServer

	repositories *store.RepositoryStore
	jobs         *store.JobStore
	runner       *Runner
}

func NewServer() *Server {
	jobs := store.NewJobStore()
	return &Server{
		repositories: store.NewRepositoryStore(),
		jobs:         jobs,
		runner:       NewRunner(jobs),
	}
}

func NewGRPCServer(server *Server) *grpc.Server {
	grpcServer := grpc.NewServer()
	rpc.RegisterDeploymentServiceServer(grpcServer, server)
	rpc.RegisterEnvServiceServer(grpcServer, server)
	rpc.RegisterJobServiceServer(grpcServer, server)
	rpc.RegisterSystemServiceServer(grpcServer, server)
	return grpcServer
}

func ListenUnix(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		return nil, err
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}

func (s *Server) Serve(listener net.Listener) error {
	return NewGRPCServer(s).Serve(listener)
}

func (s *Server) CreateDeployment(ctx context.Context, req *rpc.CreateDeploymentRequest) (*rpc.JobResponse, error) {
	if req.RepoUrl == "" {
		return nil, errors.New("repo URL is required")
	}
	return s.runner.Enqueue(ctx, "create", req.Name, func(ctx context.Context, log func(string)) error {
		return s.createDeployment(ctx, req, log)
	})
}

func (s *Server) GetDeployment(ctx context.Context, req *rpc.GetDeploymentRequest) (*rpc.Deployment, error) {
	repository, err := s.repositories.Get(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	return deploymentFromRepository(repository), nil
}

func (s *Server) ListDeployments(ctx context.Context, _ *rpc.ListDeploymentsRequest) (*rpc.ListDeploymentsResponse, error) {
	repositories, err := s.repositories.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListDeploymentsResponse{Deployments: make([]*rpc.Deployment, 0, len(repositories))}
	for _, repository := range repositories {
		response.Deployments = append(response.Deployments, deploymentFromRepository(repository))
	}
	return response, nil
}

func (s *Server) DeleteDeployment(ctx context.Context, req *rpc.DeleteDeploymentRequest) (*rpc.JobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "delete", req.Name, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.Name)
		if err != nil {
			return err
		}
		log(fmt.Sprintf("Deleting deployment %s", repository.Name))
		if err := internalfile.RemoveAllInside(internal.GetRepositoryDirectory(), repository.Location); err != nil {
			return err
		}
		return s.repositories.Delete(ctx, repository.Name)
	})
}

func (s *Server) UpdateDeployment(ctx context.Context, req *rpc.UpdateDeploymentRequest) (*rpc.JobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "update", req.Name, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.Name)
		if err != nil {
			return err
		}
		log("Pulling latest repository changes")
		if err := internalgit.PullRepo(repository.Location); err != nil {
			return err
		}
		if req.Build {
			log("Building Compose images")
			return docker.ComposeBuild(ctx, &repository)
		}
		return nil
	})
}

func (s *Server) DeployDeployment(ctx context.Context, req *rpc.DeployDeploymentRequest) (*rpc.JobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "deploy", req.Name, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.Name)
		if err != nil {
			return err
		}
		if !req.Build {
			status, err := docker.ComposeStatus(ctx, &repository)
			if err != nil {
				return err
			}
			if status.AllRunning() {
				log(fmt.Sprintf("Deployment already running: %s", status.Summary()))
				return nil
			}

			cache, err := docker.ComposeBuildCache(ctx, &repository)
			if err != nil {
				return err
			}
			if len(cache.Tags) > 0 && len(cache.Missing) == 0 {
				log(fmt.Sprintf("Using cached build: %s", strings.Join(cache.Tags, ", ")))
			}
			if len(cache.Missing) > 0 {
				log(fmt.Sprintf("No cached build found for %s. Building now.", strings.Join(cache.Missing, ", ")))
				if err := docker.ComposeBuild(ctx, &repository); err != nil {
					return err
				}
			}
		}
		if req.Build {
			log("Building Compose images")
			if err := docker.ComposeBuild(ctx, &repository); err != nil {
				return err
			}
		}
		log("Starting Compose project")
		return docker.ComposeUp(ctx, &repository)
	})
}

func (s *Server) RestartDeployment(ctx context.Context, req *rpc.RestartDeploymentRequest) (*rpc.JobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "restart", req.Name, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.Name)
		if err != nil {
			return err
		}
		status, err := docker.ComposeStatus(ctx, &repository)
		if err != nil {
			return err
		}
		if !status.AnyRunning() {
			log("Deployment is not running. Starting it now.")
		}
		if req.Build {
			log("Building Compose images")
			if err := docker.ComposeBuild(ctx, &repository); err != nil {
				return err
			}
		} else {
			cache, err := docker.ComposeBuildCache(ctx, &repository)
			if err != nil {
				return err
			}
			if len(cache.Tags) > 0 && len(cache.Missing) == 0 {
				log(fmt.Sprintf("Using cached build: %s", strings.Join(cache.Tags, ", ")))
			}
			if len(cache.Missing) > 0 {
				log(fmt.Sprintf("No cached build found for %s. Building now.", strings.Join(cache.Missing, ", ")))
				if err := docker.ComposeBuild(ctx, &repository); err != nil {
					return err
				}
			}
		}
		log("Stopping Compose project")
		if err := docker.ComposeDown(ctx, &repository); err != nil {
			return err
		}
		log("Starting Compose project")
		return docker.ComposeUp(ctx, &repository)
	})
}

func (s *Server) StopDeployment(ctx context.Context, req *rpc.StopDeploymentRequest) (*rpc.JobResponse, error) {
	if req.Name == "" {
		return nil, errors.New("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "stop", req.Name, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.Name)
		if err != nil {
			return err
		}
		status, err := docker.ComposeStatus(ctx, &repository)
		if err != nil {
			return err
		}
		if !status.AnyRunning() {
			log("Deployment is not running")
			return nil
		}
		log("Stopping Compose project")
		return docker.ComposeDown(ctx, &repository)
	})
}

func (s *Server) ListEnvNames(ctx context.Context, req *rpc.ListEnvNamesRequest) (*rpc.ListEnvNamesResponse, error) {
	repository, err := s.repositories.Get(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	targetEnvPath := resolveEnvTargetPath(repository, req.EnvFile)
	variables, err := envfile.Read(targetEnvPath)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	sort.Strings(names)
	return &rpc.ListEnvNamesResponse{Names: names}, nil
}

func (s *Server) SetEnv(ctx context.Context, req *rpc.SetEnvRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, errors.New("deployment name is required")
	}
	for name := range req.Variables {
		if err := envfile.ValidateName(name); err != nil {
			return nil, err
		}
	}
	return s.runner.Enqueue(ctx, "env.set", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		targetEnvPath := resolveEnvTargetPath(repository, req.EnvFile)
		variables, err := envfile.Read(targetEnvPath)
		if err != nil {
			return err
		}
		for name, value := range req.Variables {
			variables[name] = value
		}
		if err := envfile.Write(targetEnvPath, variables); err != nil {
			return err
		}
		if isDefaultEnvPath(repository, targetEnvPath) {
			repository.EnvPath = targetEnvPath
			if err := s.repositories.Update(ctx, repository); err != nil {
				return err
			}
		}
		log(fmt.Sprintf("Updated %d env variable(s)", len(req.Variables)))
		return nil
	})
}

func (s *Server) ImportEnvFile(ctx context.Context, req *rpc.ImportEnvFileRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, errors.New("deployment name is required")
	}
	if req.SourcePath == "" {
		return nil, errors.New("source path is required")
	}
	return s.runner.Enqueue(ctx, "env.import", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		source, ok := internalfile.ExistingFile(req.SourcePath)
		if !ok {
			return fmt.Errorf("env file %q was not found", req.SourcePath)
		}
		targetEnvPath := resolveEnvTargetPath(repository, req.EnvFile)
		if err := copyEnvFile(targetEnvPath, source); err != nil {
			return err
		}
		if isDefaultEnvPath(repository, targetEnvPath) {
			repository.EnvPath = targetEnvPath
			if err := s.repositories.Update(ctx, repository); err != nil {
				return err
			}
		}
		log("Imported env file")
		return nil
	})
}

func (s *Server) UnsetEnv(ctx context.Context, req *rpc.UnsetEnvRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, errors.New("deployment name is required")
	}
	for _, name := range req.Names {
		if err := envfile.ValidateName(name); err != nil {
			return nil, err
		}
	}
	return s.runner.Enqueue(ctx, "env.unset", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.repositories.Get(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		targetEnvPath := resolveEnvTargetPath(repository, req.EnvFile)
		variables, err := envfile.Read(targetEnvPath)
		if err != nil {
			return err
		}
		deleted := 0
		for _, name := range req.Names {
			if _, ok := variables[name]; ok {
				delete(variables, name)
				deleted++
			}
		}
		if err := envfile.Write(targetEnvPath, variables); err != nil {
			return err
		}
		if isDefaultEnvPath(repository, targetEnvPath) {
			repository.EnvPath = targetEnvPath
			if err := s.repositories.Update(ctx, repository); err != nil {
				return err
			}
		}
		log(fmt.Sprintf("Deleted %d env variable(s)", deleted))
		return nil
	})
}

func (s *Server) GetJob(ctx context.Context, req *rpc.GetJobRequest) (*rpc.Job, error) {
	job, err := s.jobs.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func (s *Server) ListJobs(ctx context.Context, req *rpc.ListJobsRequest) (*rpc.ListJobsResponse, error) {
	jobs, err := s.jobs.List(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListJobsResponse{Jobs: make([]*rpc.Job, 0, len(jobs))}
	for _, job := range jobs {
		response.Jobs = append(response.Jobs, jobToRPC(job))
	}
	return response, nil
}

func (s *Server) WatchJob(req *rpc.WatchJobRequest, stream rpc.JobService_WatchJobServer) error {
	after := req.AfterSequence
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, err := s.jobs.LogsAfter(stream.Context(), req.Id, after)
		if err != nil {
			return err
		}
		for _, log := range logs {
			after = log.Sequence
			if err := stream.Send(&rpc.JobEvent{
				JobId:    log.JobID,
				Sequence: log.Sequence,
				Message:  log.Message,
			}); err != nil {
				return err
			}
		}

		job, err := s.jobs.Get(stream.Context(), req.Id)
		if err != nil {
			return err
		}
		if isTerminal(job.Status) {
			return stream.Send(&rpc.JobEvent{JobId: job.ID, Sequence: after, Job: jobToRPC(job)})
		}

		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
		}
	}
}

func (s *Server) CancelJob(ctx context.Context, req *rpc.CancelJobRequest) (*rpc.Job, error) {
	job, err := s.jobs.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if isTerminal(job.Status) {
		return jobToRPC(job), nil
	}
	job.Status = store.JobStatusCancelled
	job.Error = "cancel requested"
	job.FinishedAt = time.Now()
	if err := s.jobs.Update(ctx, job); err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func (s *Server) Health(ctx context.Context, req *rpc.HealthRequest) (*rpc.HealthResponse, error) {
	_ = req
	return &rpc.HealthResponse{Status: dockerStatusReport(docker.CheckConnection(ctx))}, nil
}

func (s *Server) Version(context.Context, *rpc.VersionRequest) (*rpc.VersionResponse, error) {
	return &rpc.VersionResponse{Version: "dev"}, nil
}

func (s *Server) Capabilities(context.Context, *rpc.CapabilitiesRequest) (*rpc.CapabilitiesResponse, error) {
	return &rpc.CapabilitiesResponse{Capabilities: []string{
		"local-unix-socket",
		"jobs",
		"job-log-streaming",
		"per-deployment-locking",
		"masked-env-listing",
	}}, nil
}

func (s *Server) createDeployment(ctx context.Context, req *rpc.CreateDeploymentRequest, log func(string)) error {
	log("Cloning repository")
	location, err := internalgit.CloneRepo(req.RepoUrl, req.Name)
	if err != nil {
		return err
	}
	name := req.Name
	if name == "" {
		name = filepath.Base(location)
	}

	composePath, err := resolveComposePath(location, req.ComposeFile)
	if err != nil {
		return err
	}
	if composePath == "" {
		log("No compose file found. Deployment will not work until a compose file is configured.")
	}

	envPath, err := resolveEnvFile(location, req.EnvFile)
	if err != nil {
		return err
	}

	return s.repositories.Insert(ctx, store.Repository{
		Name:        name,
		URL:         req.RepoUrl,
		Location:    location,
		ComposePath: composePath,
		EnvPath:     envPath,
	})
}

type Runner struct {
	jobs  *store.JobStore
	locks sync.Map
}

func NewRunner(jobs *store.JobStore) *Runner {
	return &Runner{jobs: jobs}
}

type jobFunc func(context.Context, func(string)) error

func (r *Runner) Enqueue(ctx context.Context, jobType string, deploymentName string, fn jobFunc) (*rpc.JobResponse, error) {
	id := uuid.NewString()
	job := store.Job{
		ID:             id,
		Type:           jobType,
		DeploymentName: deploymentName,
		Status:         store.JobStatusQueued,
		CreatedAt:      time.Now(),
	}
	if err := r.jobs.Insert(ctx, job); err != nil {
		return nil, err
	}

	go r.run(job, fn)
	return &rpc.JobResponse{JobId: id}, nil
}

func (r *Runner) run(job store.Job, fn jobFunc) {
	ctx := context.Background()
	unlock := r.lock(job.DeploymentName)
	defer unlock()

	job.Status = store.JobStatusRunning
	job.StartedAt = time.Now()
	_ = r.jobs.Update(ctx, job)
	r.log(ctx, job.ID, fmt.Sprintf("Started %s job", job.Type))

	err := fn(ctx, func(message string) {
		r.log(ctx, job.ID, message)
	})

	job.FinishedAt = time.Now()
	if err != nil {
		job.Status = store.JobStatusFailed
		job.Error = err.Error()
		r.log(ctx, job.ID, fmt.Sprintf("Failed: %s", err))
	} else {
		job.Status = store.JobStatusSucceeded
		r.log(ctx, job.ID, "Succeeded")
	}
	_ = r.jobs.Update(ctx, job)
}

func (r *Runner) lock(deploymentName string) func() {
	if deploymentName == "" {
		return func() {}
	}
	value, _ := r.locks.LoadOrStore(deploymentName, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (r *Runner) log(ctx context.Context, jobID string, message string) {
	_, _ = r.jobs.AddLog(ctx, jobID, message)
}

func deploymentFromRepository(repository store.Repository) *rpc.Deployment {
	return &rpc.Deployment{
		Name:        repository.Name,
		Url:         repository.URL,
		Location:    repository.Location,
		ComposePath: repository.ComposePath,
		EnvPath:     repository.EnvPath,
	}
}

func jobToRPC(job store.Job) *rpc.Job {
	return &rpc.Job{
		Id:             job.ID,
		Type:           job.Type,
		DeploymentName: job.DeploymentName,
		Status:         job.Status,
		Error:          job.Error,
		CreatedAtUnix:  unix(job.CreatedAt),
		StartedAtUnix:  unix(job.StartedAt),
		FinishedAtUnix: unix(job.FinishedAt),
	}
}

func isTerminal(status string) bool {
	return status == store.JobStatusSucceeded || status == store.JobStatusFailed || status == store.JobStatusCancelled
}

func dockerStatusReport(status docker.ConnectionStatus) string {
	var builder strings.Builder
	if status.Connected && status.Error == "" {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: connected\n")
	} else if status.Connected {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: partially connected\n")
	} else {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: unavailable\n")
	}

	if status.Host != "" {
		fmt.Fprintf(&builder, "  Host: %s\n", status.Host)
	}
	if status.ServerVersion != "" {
		fmt.Fprintf(&builder, "  Server version: %s\n", status.ServerVersion)
	}
	if status.APIVersion != "" {
		fmt.Fprintf(&builder, "  API version: %s\n", status.APIVersion)
	}
	if status.OSType != "" {
		fmt.Fprintf(&builder, "  OS type: %s\n", status.OSType)
	}
	if status.Error != "" {
		fmt.Fprintf(&builder, "  Error: %s\n", status.Error)
	}

	return builder.String()
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

var composeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

func resolveComposePath(repositoryLocation string, composeFile string) (string, error) {
	if composeFile != "" {
		isPath := filepath.IsAbs(composeFile) || strings.ContainsAny(composeFile, `/\`)

		if isPath {
			if path, ok := internalfile.ExistingFile(composeFile); ok {
				destination := filepath.Join(repositoryLocation, "compose.yml")
				if err := internalfile.Copy(path, destination); err != nil {
					return "", fmt.Errorf("copy compose file: %w", err)
				}
				return destination, nil
			}
		}

		if !filepath.IsAbs(composeFile) {
			if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, composeFile)); ok {
				return path, nil
			}
		}

		if !isPath {
			path, ok := internalfile.ExistingFile(composeFile)
			if !ok {
				return "", nil
			}

			destination := filepath.Join(repositoryLocation, "compose.yml")
			if err := internalfile.Copy(path, destination); err != nil {
				return "", fmt.Errorf("copy compose file: %w", err)
			}
			return destination, nil
		}

		return "", nil
	}

	for _, name := range composeFileNames {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, nil
		}
	}

	return "", nil
}

func resolveEnvFile(repositoryLocation string, envFile string) (string, error) {
	if envFile == "" {
		return "", nil
	}

	source, ok := resolveRepositoryOrLocalFile(repositoryLocation, envFile)
	if !ok {
		return "", nil
	}

	destination := defaultEnvPath(repositoryLocation)
	variables, err := envfile.Read(source)
	if err != nil {
		return "", fmt.Errorf("read env file: %w", err)
	}
	if err := envfile.Write(destination, variables); err != nil {
		return "", fmt.Errorf("copy env file: %w", err)
	}

	return destination, nil
}

func resolveRepositoryOrLocalFile(repositoryLocation string, name string) (string, bool) {
	if !filepath.IsAbs(name) {
		if path, ok := internalfile.ExistingFile(filepath.Join(repositoryLocation, name)); ok {
			return path, true
		}
	}

	return internalfile.ExistingFile(name)
}

func defaultEnvPath(repositoryLocation string) string {
	return filepath.Join(repositoryLocation, ".env")
}

func copyEnvFile(destination string, source string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create env file directory: %w", err)
	}
	if err := os.WriteFile(destination, contents, 0600); err != nil {
		return fmt.Errorf("copy env file: %w", err)
	}

	return nil
}

func resolveEnvTargetPath(repository store.Repository, envFile string) string {
	if envFile == "" {
		return defaultEnvPath(repository.Location)
	}
	if filepath.IsAbs(envFile) {
		return filepath.Clean(envFile)
	}
	return filepath.Join(envFileBaseDir(repository), envFile)
}

func envFileBaseDir(repository store.Repository) string {
	if repository.ComposePath != "" {
		return filepath.Dir(repository.ComposePath)
	}
	return repository.Location
}

func isDefaultEnvPath(repository store.Repository, path string) bool {
	return filepath.Clean(path) == filepath.Clean(defaultEnvPath(repository.Location))
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
