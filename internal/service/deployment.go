package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"deployctl/internal"
	"deployctl/internal/docker"
	internalfile "deployctl/internal/file"
	internalgit "deployctl/internal/git"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

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

func deploymentFromRepository(repository store.Repository) *rpc.Deployment {
	return &rpc.Deployment{
		Name:        repository.Name,
		Url:         repository.URL,
		Location:    repository.Location,
		ComposePath: repository.ComposePath,
		EnvPath:     repository.EnvPath,
	}
}
