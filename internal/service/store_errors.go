package service

import (
	"context"

	"deployctl/internal/store"
)

func (s *Server) getRepository(ctx context.Context, name string) (store.Repository, error) {
	repository, err := s.repositories.Get(ctx, name)
	return repository, repositoryError(name, err)
}

func (s *Server) insertRepository(ctx context.Context, repository store.Repository) error {
	return repositoryInsertError(repository.Name, s.repositories.Insert(ctx, repository))
}

func (s *Server) updateRepository(ctx context.Context, repository store.Repository) error {
	return repositoryError(repository.Name, s.repositories.Update(ctx, repository))
}

func (s *Server) deleteRepository(ctx context.Context, name string) error {
	return repositoryError(name, s.repositories.Delete(ctx, name))
}

func (s *Server) getJob(ctx context.Context, id string) (store.Job, error) {
	job, err := s.jobs.Get(ctx, id)
	return job, jobError(id, err)
}
