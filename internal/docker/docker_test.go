package docker

import "testing"

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
