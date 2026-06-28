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

	repositories *store.RepositoryStore
	jobs         *store.JobStore
	runner       *Runner
	logger       *Logger
}

func NewServer() *Server {
	return NewServerWithLogger(NewDaemonLogger())
}

func NewServerWithLogger(logger *Logger) *Server {
	jobs := store.NewJobStore()
	return &Server{
		repositories: store.NewRepositoryStore(),
		jobs:         jobs,
		runner:       NewRunner(jobs, logger),
		logger:       logger,
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
	s.logger.Printf("serving deployctld on %s", listener.Addr().String())
	err := NewGRPCServer(s).Serve(listener)
	if err != nil {
		s.logger.Printf("deployctld stopped with error: %v", err)
	}
	return err
}
