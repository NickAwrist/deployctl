package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"deployctl/internal/rpc"
	"deployctl/internal/service"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	dbPath := filepath.Join(tempDir, ".deployctl", "deployctl.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	svc, err := service.NewServer()
	if err != nil {
		t.Fatalf("new service server: %v", err)
	}
	t.Cleanup(func() {
		_ = svc.Close()
	})
	return NewServer(svc)
}

func TestSystemEndpoints(t *testing.T) {
	server := setupTestServer(t)

	t.Run("health endpoint returns ok status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/health", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp SystemHealthResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status == "" {
			t.Fatal("expected status to be non-empty")
		}
	})
}

func TestDeploymentsEndpoints(t *testing.T) {
	server := setupTestServer(t)

	t.Run("list deployments returns empty list initially", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var items []DeploymentListItem
		if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(items) != 0 {
			t.Fatalf("expected 0 items, got %d", len(items))
		}
	})

	t.Run("get non-existent deployment returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/deployments/nonexistent", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestDeploymentListItemPreservesState(t *testing.T) {
	deployment := &rpc.Deployment{Name: "api"}
	tests := []struct {
		name  string
		state rpc.DeploymentState
		want  string
	}{
		{name: "not configured", state: rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CONFIGURED, want: "not_configured"},
		{name: "not created", state: rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CREATED, want: "not_created"},
		{name: "running", state: rpc.DeploymentState_DEPLOYMENT_STATE_RUNNING, want: "running"},
		{name: "partial", state: rpc.DeploymentState_DEPLOYMENT_STATE_PARTIAL, want: "partial"},
		{name: "stopped", state: rpc.DeploymentState_DEPLOYMENT_STATE_STOPPED, want: "stopped"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := deploymentListItem(deployment, &rpc.DeploymentStatus{State: test.state}, nil)
			if item.State != test.want {
				t.Fatalf("state = %q, want %q", item.State, test.want)
			}
		})
	}

	t.Run("status error", func(t *testing.T) {
		item := deploymentListItem(deployment, nil, errors.New("docker unavailable"))
		if item.State != "unavailable" {
			t.Fatalf("state = %q, want unavailable", item.State)
		}
		if item.StatusError != "docker unavailable" {
			t.Fatalf("status error = %q, want docker unavailable", item.StatusError)
		}
	})
}

func TestJobsEndpoints(t *testing.T) {
	server := setupTestServer(t)

	t.Run("list jobs returns empty list initially", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var jobs []JobResponse
		if err := json.NewDecoder(rec.Body).Decode(&jobs); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(jobs) != 0 {
			t.Fatalf("expected 0 jobs, got %d", len(jobs))
		}
	})

	t.Run("streaming an unknown job returns not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/missing/events", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestStaticFallback(t *testing.T) {
	server := setupTestServer(t)

	t.Run("serves index.html on root path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("falls back to index.html for SPA subpaths", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/deployments/my-app", nil)
		rec := httptest.NewRecorder()

		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})
}
