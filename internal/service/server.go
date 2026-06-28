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
