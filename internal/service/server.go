package service

import (
	"net"
	"os"
	"path/filepath"

	"deployctl/internal/rpc"
	"deployctl/internal/store"

	"google.golang.org/grpc"
)

type Server struct {
	rpc.UnimplementedDeploymentServiceServer
	rpc.UnimplementedEnvServiceServer
	rpc.UnimplementedJobServiceServer
	rpc.UnimplementedSystemServiceServer

	store        *store.Store
	repositories *store.RepositoryStore
	jobs         *store.JobStore
	runner       *Runner
	logger       *Logger
}

func NewServer() (*Server, error) {
	logger, err := NewDaemonLogger()
	if err != nil {
		return nil, err
	}
	return NewServerWithLogger(logger)
}

func NewServerWithLogger(logger *Logger) (*Server, error) {
	dataStore, err := store.OpenDefault()
	if err != nil {
		return nil, err
	}
	jobs := dataStore.Jobs
	return &Server{
		store:        dataStore,
		repositories: dataStore.Repositories,
		jobs:         jobs,
		runner:       NewRunner(jobs, logger),
		logger:       logger,
	}, nil
}

func (s *Server) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func NewGRPCServer(server *Server) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(normalizeUnaryError))
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
	s.logger.Printf("serving deployctld on %s", listener.Addr().String())
	err := NewGRPCServer(s).Serve(listener)
	if err != nil {
		s.logger.Printf("deployctld stopped with error: %v", err)
	}
	return err
}
