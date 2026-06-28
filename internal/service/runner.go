package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"deployctl/internal/rpc"
	"deployctl/internal/store"

	"github.com/google/uuid"
)

type Runner struct {
	jobs  *store.JobStore
	locks sync.Map
}

func NewRunner(jobs *store.JobStore) *Runner {
	return &Runner{jobs: jobs}
}

type jobFunc func(context.Context, func(string)) error

func (r *Runner) Enqueue(ctx context.Context, jobType string, deploymentName string, fn jobFunc) (*rpc.JobResponse, error) {
	id := uuid.NewString()
	job := store.Job{
		ID:             id,
		Type:           jobType,
		DeploymentName: deploymentName,
		Status:         store.JobStatusQueued,
		CreatedAt:      time.Now(),
	}
	if err := r.jobs.Insert(ctx, job); err != nil {
		return nil, err
	}

	go r.run(job, fn)
	return &rpc.JobResponse{JobId: id}, nil
}

func (r *Runner) run(job store.Job, fn jobFunc) {
	ctx := context.Background()
	unlock := r.lock(job.DeploymentName)
	defer unlock()

	job.Status = store.JobStatusRunning
	job.StartedAt = time.Now()
	_ = r.jobs.Update(ctx, job)
	r.log(ctx, job.ID, fmt.Sprintf("Started %s job", job.Type))

	err := fn(ctx, func(message string) {
		r.log(ctx, job.ID, message)
	})

	job.FinishedAt = time.Now()
	if err != nil {
		job.Status = store.JobStatusFailed
		job.Error = err.Error()
		r.log(ctx, job.ID, fmt.Sprintf("Failed: %s", err))
	} else {
		job.Status = store.JobStatusSucceeded
		r.log(ctx, job.ID, "Succeeded")
	}
	_ = r.jobs.Update(ctx, job)
}

func (r *Runner) lock(deploymentName string) func() {
	if deploymentName == "" {
		return func() {}
	}
	value, _ := r.locks.LoadOrStore(deploymentName, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (r *Runner) log(ctx context.Context, jobID string, message string) {
	_, _ = r.jobs.AddLog(ctx, jobID, message)
}
