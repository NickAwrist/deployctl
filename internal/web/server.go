package web

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"

	"deployctl/internal/service"
)

type Server struct {
	service *service.Server
	mux     *http.ServeMux
	dist    fs.FS
}

func NewServer(serviceServer *service.Server) *Server {
	dist, _ := DistFS()
	s := &Server{
		service: serviceServer,
		mux:     http.NewServeMux(),
		dist:    dist,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// System routes
	s.mux.HandleFunc("GET /api/system/health", s.handleSystemHealth)

	// Deployment routes
	s.mux.HandleFunc("GET /api/deployments", s.handleListDeployments)
	s.mux.HandleFunc("POST /api/deployments", s.handleCreateDeployment)
	s.mux.HandleFunc("GET /api/deployments/{name}", s.handleGetDeployment)
	s.mux.HandleFunc("DELETE /api/deployments/{name}", s.handleDeleteDeployment)
	s.mux.HandleFunc("POST /api/deployments/{name}/deploy", s.handleDeployDeployment)
	s.mux.HandleFunc("POST /api/deployments/{name}/restart", s.handleRestartDeployment)
	s.mux.HandleFunc("POST /api/deployments/{name}/stop", s.handleStopDeployment)
	s.mux.HandleFunc("POST /api/deployments/{name}/update", s.handleUpdateDeployment)

	// Environment variable routes
	s.mux.HandleFunc("GET /api/deployments/{name}/env", s.handleListEnv)
	s.mux.HandleFunc("POST /api/deployments/{name}/env", s.handleSetEnv)
	s.mux.HandleFunc("DELETE /api/deployments/{name}/env", s.handleUnsetEnv)

	// Job routes
	s.mux.HandleFunc("GET /api/jobs", s.handleListJobs)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("GET /api/jobs/{id}/events", s.handleJobEvents)
	s.mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleCancelJob)

	// Static assets and SPA fallback
	s.mux.HandleFunc("GET /", s.handleStatic)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if s.dist == nil {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Check if file exists in embedded dist
	f, err := s.dist.Open(path)
	if err == nil {
		_ = f.Close()
		http.FileServer(http.FS(s.dist)).ServeHTTP(w, r)
		return
	}

	if errors.Is(err, os.ErrNotExist) {
		// Fallback to index.html for SPA routes
		indexFile, err := s.dist.Open("index.html")
		if err == nil {
			_ = indexFile.Close()
			r.URL.Path = "/"
			http.FileServer(http.FS(s.dist)).ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) Serve(listener net.Listener) error {
	httpServer := &http.Server{
		Handler: s,
	}
	return httpServer.Serve(listener)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
