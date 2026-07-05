package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deployctl/internal/envfile"
	internalfile "deployctl/internal/file"
	"deployctl/internal/rpc"
	"deployctl/internal/store"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/loader"
)

func (s *Server) ListEnvNames(ctx context.Context, req *rpc.ListEnvNamesRequest) (*rpc.ListEnvNamesResponse, error) {
	repository, err := s.getRepository(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	names, err := listEnvNames(repository, req.EnvFile)
	if err != nil {
		return nil, err
	}
	return &rpc.ListEnvNamesResponse{Names: names}, nil
}

func listEnvNames(repository store.Repository, envFile string) ([]string, error) {
	targetEnvPath := resolveEnvTargetPath(repository, envFile)
	variables, err := envfile.Read(targetEnvPath)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Server) ListEnvFiles(ctx context.Context, req *rpc.ListEnvFilesRequest) (*rpc.ListEnvFilesResponse, error) {
	repository, err := s.getRepository(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	if req.EnvFile != "" {
		return listExplicitEnvFile(repository, req.EnvFile)
	}
	return listDeploymentEnvFiles(ctx, repository)
}

func listExplicitEnvFile(repository store.Repository, envFile string) (*rpc.ListEnvFilesResponse, error) {
	targetEnvPath := resolveEnvTargetPath(repository, envFile)
	if exists, err := regularFileExists(targetEnvPath); err != nil {
		return nil, err
	} else if !exists {
		return &rpc.ListEnvFilesResponse{MissingEnvFiles: []string{envFile}}, nil
	}

	names, err := listEnvNames(repository, envFile)
	if err != nil {
		return nil, err
	}
	return &rpc.ListEnvFilesResponse{
		EnvFiles: []*rpc.EnvFileVariables{{EnvFile: envFile, Names: names}},
	}, nil
}

func listDeploymentEnvFiles(ctx context.Context, repository store.Repository) (*rpc.ListEnvFilesResponse, error) {
	baseDir := envFileBaseDir(repository)
	response := &rpc.ListEnvFilesResponse{}
	candidatesByPath := map[string]string{}
	missingByName := map[string]struct{}{}

	addCandidate := func(path string) error {
		if path == "" {
			return nil
		}
		cleanPath := filepath.Clean(path)
		exists, err := regularFileExists(cleanPath)
		if err != nil {
			return err
		}
		if exists {
			candidatesByPath[cleanPath] = displayEnvPath(repository, baseDir, cleanPath)
		}
		return nil
	}
	addMissing := func(path string) {
		if path == "" {
			return
		}
		missingByName[displayEnvPath(repository, baseDir, filepath.Clean(path))] = struct{}{}
	}

	if err := addCandidate(repository.EnvPath); err != nil {
		return nil, err
	}
	if err := discoverEnvFiles(baseDir, addCandidate); err != nil {
		return nil, err
	}

	composeRefs, err := composeEnvFileReferences(ctx, repository)
	if err != nil {
		return nil, err
	}
	for _, ref := range composeRefs {
		exists, err := regularFileExists(ref.Path)
		if err != nil {
			return nil, err
		}
		if exists {
			candidatesByPath[ref.Path] = displayEnvPath(repository, baseDir, ref.Path)
			continue
		}
		addMissing(ref.Path)
	}

	envPaths := make([]string, 0, len(candidatesByPath))
	for path := range candidatesByPath {
		envPaths = append(envPaths, path)
	}
	sort.Slice(envPaths, func(i, j int) bool {
		return candidatesByPath[envPaths[i]] < candidatesByPath[envPaths[j]]
	})
	for _, path := range envPaths {
		names, err := listEnvNamesFromPath(path)
		if err != nil {
			return nil, err
		}
		response.EnvFiles = append(response.EnvFiles, &rpc.EnvFileVariables{
			EnvFile: candidatesByPath[path],
			Names:   names,
		})
	}

	for name := range missingByName {
		response.MissingEnvFiles = append(response.MissingEnvFiles, name)
	}
	sort.Strings(response.MissingEnvFiles)
	return response, nil
}

func listEnvNamesFromPath(path string) ([]string, error) {
	variables, err := envfile.Read(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Server) SetEnv(ctx context.Context, req *rpc.SetEnvRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	for name := range req.Variables {
		if err := envfile.ValidateName(name); err != nil {
			return nil, invalidArgument(err.Error())
		}
	}
	return s.runner.Enqueue(ctx, "env.set", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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
			if err := s.updateRepository(ctx, repository); err != nil {
				return err
			}
		}
		log(fmt.Sprintf("Updated %d env variable(s)", len(req.Variables)))
		return nil
	})
}

func (s *Server) ImportEnvFile(ctx context.Context, req *rpc.ImportEnvFileRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	if req.SourcePath == "" {
		return nil, invalidArgument("source path is required")
	}
	return s.runner.Enqueue(ctx, "env.import", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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
			if err := s.updateRepository(ctx, repository); err != nil {
				return err
			}
		}
		log("Imported env file")
		return nil
	})
}

func (s *Server) UnsetEnv(ctx context.Context, req *rpc.UnsetEnvRequest) (*rpc.JobResponse, error) {
	if req.DeploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	for _, name := range req.Names {
		if err := envfile.ValidateName(name); err != nil {
			return nil, invalidArgument(err.Error())
		}
	}
	return s.runner.Enqueue(ctx, "env.unset", req.DeploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, req.DeploymentName)
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
			if err := s.updateRepository(ctx, repository); err != nil {
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

func regularFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}

func discoverEnvFiles(baseDir string, addCandidate func(string) error) error {
	if baseDir == "" {
		return nil
	}
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !looksLikeEnvFile(entry.Name()) {
			continue
		}
		if err := addCandidate(filepath.Join(baseDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func looksLikeEnvFile(name string) bool {
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "example") ||
		strings.Contains(lowerName, "sample") ||
		strings.Contains(lowerName, "template") {
		return false
	}
	return name == ".env" ||
		strings.HasPrefix(name, ".env.") ||
		strings.HasPrefix(name, ".env-") ||
		strings.HasPrefix(name, "env.") ||
		strings.HasPrefix(name, "env-") ||
		strings.HasSuffix(name, ".env")
}

type composeEnvFileReference struct {
	Path string
}

func composeEnvFileReferences(ctx context.Context, repository store.Repository) ([]composeEnvFileReference, error) {
	if repository.ComposePath == "" {
		return nil, nil
	}
	if exists, err := regularFileExists(repository.ComposePath); err != nil {
		return nil, fmt.Errorf("inspect compose env files: %w", err)
	} else if !exists {
		return nil, fmt.Errorf("inspect compose env files: compose file %q was not found", repository.ComposePath)
	}

	optionsFns := []composecli.ProjectOptionsFn{
		composecli.WithWorkingDirectory(repository.Location),
		composecli.WithLoadOptions(func(options *loader.Options) {
			options.SkipResolveEnvironment = true
		}),
	}
	if repository.EnvPath != "" {
		optionsFns = append(optionsFns,
			composecli.WithEnvFiles(repository.EnvPath),
			composecli.WithDotEnv,
			composecli.WithEnv([]string{"DEPLOYCTL_ENV_FILE=" + repository.EnvPath}),
		)
	} else {
		optionsFns = append(optionsFns,
			composecli.WithEnvFiles(),
			composecli.WithDotEnv,
		)
	}

	options, err := composecli.NewProjectOptions([]string{repository.ComposePath}, optionsFns...)
	if err != nil {
		return nil, fmt.Errorf("inspect compose env files: %w", err)
	}
	project, err := options.LoadProject(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect compose env files: %w", err)
	}

	refsByPath := map[string]struct{}{}
	for _, service := range project.Services {
		for _, envFile := range service.EnvFiles {
			if envFile.Path == "" {
				continue
			}
			refsByPath[filepath.Clean(envFile.Path)] = struct{}{}
		}
	}

	refs := make([]composeEnvFileReference, 0, len(refsByPath))
	for path := range refsByPath {
		refs = append(refs, composeEnvFileReference{Path: path})
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Path < refs[j].Path
	})
	return refs, nil
}

func displayEnvPath(repository store.Repository, baseDir string, path string) string {
	if display, ok := relativeEnvPath(baseDir, path); ok {
		return display
	}
	if display, ok := relativeEnvPath(repository.Location, path); ok {
		return display
	}
	return path
}

func relativeEnvPath(baseDir string, path string) (string, bool) {
	if baseDir == "" {
		return "", false
	}
	relative, err := filepath.Rel(baseDir, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", false
	}
	return relative, true
}
