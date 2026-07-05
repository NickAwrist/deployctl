package service

import (
	"context"

	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

type repositoryJobFunc func(context.Context, store.Repository, func(string)) error

func (s *Server) enqueueRepositoryJob(ctx context.Context, jobType string, deploymentName string, fn repositoryJobFunc) (*rpc.JobResponse, error) {
	if deploymentName == "" {
		return nil, invalidArgument("deployment name is required")
	}
	return s.runner.Enqueue(ctx, jobType, deploymentName, func(ctx context.Context, log func(string)) error {
		repository, err := s.getRepository(ctx, deploymentName)
		if err != nil {
			return err
		}
		return fn(ctx, repository, log)
	})
}
