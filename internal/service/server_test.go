package service

import (
	"context"
	"testing"
	"time"

	"deployctl/internal"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

func TestRunnerSerializesJobsPerDeployment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	internal.InitializeDirectoryStructure()

	runner := NewRunner(store.NewJobStore(), nil)
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
	internal.InitializeDirectoryStructure()

	server := NewServerWithLogger(nil)
	response, err := server.runner.Enqueue(context.Background(), "deploy", "api", func(ctx context.Context, log func(string)) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}

	job, err := server.CancelJob(context.Background(), &rpc.CancelJobRequest{Id: response.JobId})
	if err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if job.Status != store.JobStatusCancelled {
		t.Fatalf("job status = %s, want %s", job.Status, store.JobStatusCancelled)
	}
}

func assertJobStatus(t *testing.T, id string, status string) {
	t.Helper()

	jobs := store.NewJobStore()
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
