package web

import (
	"net/http"

	"deployctl/internal/docker"
)

type SystemHealthResponse struct {
	Status string                   `json:"status"`
	Docker DockerConnectionResponse `json:"docker"`
}

type DockerConnectionResponse struct {
	Connected     bool   `json:"connected"`
	Host          string `json:"host"`
	ServerVersion string `json:"server_version"`
	APIVersion    string `json:"api_version"`
	OSType        string `json:"os_type"`
	Error         string `json:"error,omitempty"`
}

func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	conn := docker.CheckConnection(r.Context())
	status := "ok"
	if !conn.Connected || conn.Error != "" {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, SystemHealthResponse{
		Status: status,
		Docker: DockerConnectionResponse{
			Connected:     conn.Connected,
			Host:          conn.Host,
			ServerVersion: conn.ServerVersion,
			APIVersion:    conn.APIVersion,
			OSType:        conn.OSType,
			Error:         conn.Error,
		},
	})
}
