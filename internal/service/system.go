package service

import (
	"context"

	"deployctl/internal/docker"
	"deployctl/internal/rpc"
)

func (s *Server) Health(ctx context.Context, req *rpc.HealthRequest) (*rpc.HealthResponse, error) {
	_ = req
	return &rpc.HealthResponse{
		DaemonReachable: true,
		Docker:          dockerHealthToRPC(docker.CheckConnection(ctx)),
	}, nil
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
		"deployment-status",
	}}, nil
}

func dockerHealthToRPC(status docker.ConnectionStatus) *rpc.DockerHealth {
	return &rpc.DockerHealth{
		State:         dockerConnectionStateToRPC(status),
		Host:          status.Host,
		ServerVersion: status.ServerVersion,
		ApiVersion:    status.APIVersion,
		OsType:        status.OSType,
		Error:         status.Error,
	}
}

func dockerConnectionStateToRPC(status docker.ConnectionStatus) rpc.DockerConnectionState {
	if status.Connected && status.Error == "" {
		return rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_CONNECTED
	}
	if status.Connected {
		return rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_PARTIALLY_CONNECTED
	}
	return rpc.DockerConnectionState_DOCKER_CONNECTION_STATE_UNAVAILABLE
}
