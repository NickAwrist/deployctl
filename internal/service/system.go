package service

import (
	"context"
	"fmt"
	"strings"

	"deployctl/internal/docker"
	"deployctl/internal/rpc"
)

func (s *Server) Health(ctx context.Context, req *rpc.HealthRequest) (*rpc.HealthResponse, error) {
	_ = req
	return &rpc.HealthResponse{Status: dockerStatusReport(docker.CheckConnection(ctx))}, nil
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
	}}, nil
}

func dockerStatusReport(status docker.ConnectionStatus) string {
	var builder strings.Builder
	if status.Connected && status.Error == "" {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: connected\n")
	} else if status.Connected {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: partially connected\n")
	} else {
		builder.WriteString("Docker\n")
		builder.WriteString("  Status: unavailable\n")
	}

	if status.Host != "" {
		fmt.Fprintf(&builder, "  Host: %s\n", status.Host)
	}
	if status.ServerVersion != "" {
		fmt.Fprintf(&builder, "  Server version: %s\n", status.ServerVersion)
	}
	if status.APIVersion != "" {
		fmt.Fprintf(&builder, "  API version: %s\n", status.APIVersion)
	}
	if status.OSType != "" {
		fmt.Fprintf(&builder, "  OS type: %s\n", status.OSType)
	}
	if status.Error != "" {
		fmt.Fprintf(&builder, "  Error: %s\n", status.Error)
	}

	return builder.String()
}
