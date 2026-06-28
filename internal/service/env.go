package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"deployctl/internal/envfile"
	internalfile "deployctl/internal/file"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

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
