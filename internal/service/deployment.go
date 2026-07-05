package service

import (
	"context"
	"fmt"
	"path/filepath"

	"deployctl/internal"
	"deployctl/internal/docker"
	internalfile "deployctl/internal/file"
	internalgit "deployctl/internal/git"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

func (s *Server) CreateDeployment(ctx context.Context, req *rpc.CreateDeploymentRequest) (*rpc.JobResponse, error) {
	if req.RepoUrl == "" {
		return nil, invalidArgument("repo URL is required")
	}
	return s.runner.Enqueue(ctx, "create", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		return s.createDeployment(ctx, req, log)
	})
}

func (s *Server) GetDeployment(ctx context.Context, req *rpc.GetDeploymentRequest) (*rpc.Deployment, error) {
	repository, err := s.getRepository(ctx, req.DeploymentName)
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
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "delete", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		log(fmt.Sprintf("Deleting deployment %s", repository.Name))
		repositoryDirectory, err := internal.RepositoryDirectory()
		if err != nil {
			return err
		}
		if err := internalfile.RemoveAllInside(repositoryDirectory, repository.Location); err != nil {
			return err
		}
		return s.deleteRepository(ctx, repository.Name)
	})
}

func (s *Server) BuildDeployment(ctx context.Context, req *rpc.BuildDeploymentRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "build", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		return docker.ComposeBuild(ctx, &repository, log)
	})
}

func (s *Server) UpdateDeployment(ctx context.Context, req *rpc.UpdateDeploymentRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "update", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
		if err != nil {
			return err
		}
		log("Pulling latest repository changes")
		if err := internalgit.PullRepo(ctx, repository.Location, log); err != nil {
			return err
		}
		if req.Build {
			return docker.ComposeBuild(ctx, &repository, log)
		}
		return nil
	})
}

func (s *Server) DeployDeployment(ctx context.Context, req *rpc.DeployDeploymentRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "deploy", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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

			if err := docker.EnsureBuildAvailable(ctx, &repository, log); err != nil {
				return err
			}
		}
		if req.Build {
			if err := docker.ComposeBuild(ctx, &repository, log); err != nil {
				return err
			}
		}
		return docker.ComposeUp(ctx, &repository, log)
	})
}

func (s *Server) RestartDeployment(ctx context.Context, req *rpc.RestartDeploymentRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "restart", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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
			if err := docker.ComposeBuild(ctx, &repository, log); err != nil {
				return err
			}
		} else {
			if err := docker.EnsureBuildAvailable(ctx, &repository, log); err != nil {
				return err
			}
		}
		if err := docker.ComposeDown(ctx, &repository, log); err != nil {
			return err
		}
		return docker.ComposeUp(ctx, &repository, log)
	})
}

func (s *Server) StopDeployment(ctx context.Context, req *rpc.StopDeploymentRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, "stop", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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
		return docker.ComposeDown(ctx, &repository, log)
	})
}

func (s *Server) createDeployment(ctx context.Context, req *rpc.CreateDeploymentRequest, log func(string)) error {
	log("Cloning repository")
	location, err := internalgit.CloneRepo(ctx, req.RepoUrl, req.DeploymentName, log)
	if err != nil {
		return err
	}
	name := req.DeploymentName
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

	return s.insertRepository(ctx, store.Repository{
		Name:        name,
		URL:         req.RepoUrl,
		Location:    location,
		ComposePath: composePath,
		EnvPath:     envPath,
	})
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
