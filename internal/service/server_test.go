package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"deployctl/internal"
	"deployctl/internal/rpc"
	"deployctl/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRunnerSerializesJobsPerDeployment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := internal.InitializeDirectoryStructure(); err != nil {
		t.Fatalf("initialize directory structure: %v", err)
	}

	dataStore, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = dataStore.Close()
	})
	runner := NewRunner(dataStore.Jobs, nil)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	t.Cleanup(func() {
		closeIfOpen(releaseFirst)
	})

	first, err := runner.Enqueue(context.Background(), "deploy", "api", func(context.Context, func(string)) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	})
	if err != nil {
		t.Fatalf("enqueue first job: %v", err)
	}

	second, err := runner.Enqueue(context.Background(), "deploy", "api", func(context.Context, func(string)) error {
		close(secondStarted)
		return nil
	})
	if err != nil {
		t.Fatalf("enqueue second job: %v", err)
	}

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first job did not start")
	}

	select {
	case <-secondStarted:
		t.Fatal("second job started before first job finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("second job did not start after first job finished")
	}

	assertJobStatus(t, first.JobId, store.JobStatusSucceeded)
	assertJobStatus(t, second.JobId, store.JobStatusSucceeded)
}

func TestCancelJobCancelsRunningJob(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := internal.InitializeDirectoryStructure(); err != nil {
		t.Fatalf("initialize directory structure: %v", err)
	}

	server, err := NewServerWithLogger(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
	response, err := server.runner.Enqueue(context.Background(), "deploy", "api", func(ctx context.Context, log func(string)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	job, err := server.CancelJob(context.Background(), &rpc.CancelJobRequest{JobId: response.JobId})
	if err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if job.Status != rpc.JobStatus_JOB_STATUS_CANCELLED {
		t.Fatalf("job status = %s, want %s", job.Status, rpc.JobStatus_JOB_STATUS_CANCELLED)
	}
}

func TestMissingJobUsesScopedNotFoundError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := internal.InitializeDirectoryStructure(); err != nil {
		t.Fatalf("initialize directory structure: %v", err)
	}

	server, err := NewServerWithLogger(nil)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})

	_, err = server.GetJob(context.Background(), &rpc.GetJobRequest{JobId: "missing-job"})
	if err == nil {
		t.Fatal("missing job lookup succeeded")
	}
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing job error = %T %v, want NotFoundError", err, err)
	}
	if got, want := err.Error(), `job "missing-job" not found`; got != want {
		t.Fatalf("missing job error = %q, want %q", got, want)
	}
}

func TestNormalizeRPCErrorMapsScopedErrors(t *testing.T) {
	err := normalizeRPCError(deploymentNotFound("missing-api"))
	got, ok := status.FromError(err)
	if !ok {
		t.Fatalf("normalized error is not a gRPC status: %v", err)
	}
	if got.Code() != codes.NotFound {
		t.Fatalf("normalized code = %s, want %s", got.Code(), codes.NotFound)
	}
	if got.Message() != `deployment "missing-api" not found` {
		t.Fatalf("normalized message = %q", got.Message())
	}

	err = normalizeRPCError(deploymentConflict("api"))
	got, ok = status.FromError(err)
	if !ok {
		t.Fatalf("normalized conflict is not a gRPC status: %v", err)
	}
	if got.Code() != codes.AlreadyExists {
		t.Fatalf("normalized conflict code = %s, want %s", got.Code(), codes.AlreadyExists)
	}
	if got.Message() != `deployment "api" already exists` {
		t.Fatalf("normalized conflict message = %q", got.Message())
	}
}

func assertJobStatus(t *testing.T, id string, status string) {
	t.Helper()

	dataStore, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer dataStore.Close()
	jobs := dataStore.Jobs
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.Get(context.Background(), id)
		if err == nil && job.Status == status {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	job, err := jobs.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get job %s: %v", id, err)
	}
	t.Fatalf("job %s status = %s, want %s", id, job.Status, status)
}

func closeIfOpen(ch chan struct{}) {
	defer func() {
		_ = recover()
	}()
	close(ch)
}
