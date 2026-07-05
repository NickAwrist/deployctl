package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestDeploymentStatusAllRunningAndSummary(t *testing.T) {
	status := DeploymentStatus{
		Containers: []ContainerStatus{
			{Name: "api-web-1", Status: "Up 20 minutes", State: "running"},
			{Name: "api-worker-1", Status: "Up 10 minutes", State: "running"},
		},
	}

	if !status.AllRunning() {
		t.Fatal("status should be running when containers exist and no services are missing")
	}
	if !status.AnyRunning() {
		t.Fatal("status should have running containers")
	}

	want := "api-web-1 (Up 20 minutes), api-worker-1 (Up 10 minutes)"
	if got := status.Summary(); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestDeploymentStatusAllRunningWithMissingService(t *testing.T) {
	status := DeploymentStatus{
		Containers: []ContainerStatus{{Name: "api-web-1", Status: "Up 20 minutes", State: "running"}},
		Missing:    []string{"worker"},
	}

	if status.AllRunning() {
		t.Fatal("status should not be running when a service is missing")
	}
	if !status.AnyRunning() {
		t.Fatal("status should still have a running container")
	}
}

func TestDeploymentStatusAnyRunningWithNoContainers(t *testing.T) {
	if (DeploymentStatus{}).AnyRunning() {
		t.Fatal("empty status should not have running containers")
	}
}

func TestDeploymentStatusAnyRunningWithStoppedContainer(t *testing.T) {
	status := DeploymentStatus{Containers: []ContainerStatus{{Name: "api-web-1", Status: "Exited", State: "exited"}}}
	if status.AnyRunning() {
		t.Fatal("stopped containers should not count as running")
	}
	if got := status.Summary(); got != "" {
		t.Fatalf("summary = %q, want empty", got)
	}
}

func TestMissingConfigFileIsNotDockerUnavailable(t *testing.T) {
	err := fmt.Errorf("read env file: %w", os.ErrNotExist)
	if isDockerConnectionError(err) {
		t.Fatal("missing config files should not be classified as Docker connection errors")
	}
}

func TestResolveServiceEnvFilesTreatsMissingFilesAsEmpty(t *testing.T) {
	directory := t.TempDir()
	existingEnvPath := filepath.Join(directory, "app.env")
	if err := os.WriteFile(existingEnvPath, []byte("PORT=3000\n"), 0600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	missingEnvPath := filepath.Join(directory, ".env")
	project := &types.Project{
		Services: types.Services{
			"api": {
				Name: "api",
				EnvFiles: []types.EnvFile{
					{Path: missingEnvPath, Required: types.OptOut(true)},
					{Path: existingEnvPath, Required: types.OptOut(true)},
				},
			},
		},
	}

	resolved, err := resolveServiceEnvFiles(project)
	if err != nil {
		t.Fatalf("resolve service env files: %v", err)
	}
	service := resolved.Services["api"]
	if service.EnvFiles[0].Required {
		t.Fatalf("missing env file was left required: %#v", service.EnvFiles[0])
	}
	if !service.EnvFiles[1].Required {
		t.Fatalf("existing env file was marked optional: %#v", service.EnvFiles[1])
	}
	if got := service.Environment["PORT"]; got == nil || *got != "3000" {
		t.Fatalf("PORT = %v, want 3000", got)
	}
}
