package service

import (
	"deployctl/internal/docker"
	"deployctl/internal/rpc"
)

func (s *Server) StreamDeploymentLogs(req *rpc.StreamDeploymentLogsRequest, stream rpc.DeploymentService_StreamDeploymentLogsServer) error {
	if req.Lines < 0 {
		return normalizeRPCError(invalidArgument("lines must be greater than or equal to 0"))
	}

	repository, err := s.getRepository(stream.Context(), req.DeploymentName)
	if err != nil {
		return normalizeRPCError(err)
	}

	err = docker.ComposeLogs(stream.Context(), &repository, docker.LogOptions{
		Follow: req.Follow,
		Lines:  req.Lines,
	}, func(entry docker.LogEntry) error {
		return stream.Send(&rpc.DeploymentLogEntry{
			Container: entry.Container,
			Message:   entry.Message,
		})
	})
	return normalizeRPCError(err)
}
