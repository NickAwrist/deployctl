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
	jobs    *store.JobStore
	logger  *Logger
	locks   sync.Map
	running sync.Map
}

func NewRunner(jobs *store.JobStore, logger *Logger) *Runner {
	return &Runner{jobs: jobs, logger: logger}
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

	ctx, cancel := context.WithCancel(context.Background())
	r.running.Store(id, cancel)
	go r.run(ctx, job, fn)
	return &rpc.JobResponse{JobId: id}, nil
}

func (r *Runner) run(ctx context.Context, job store.Job, fn jobFunc) {
	defer r.running.Delete(job.ID)

	unlock := r.lock(job.DeploymentName)
	defer unlock()

	job.Status = store.JobStatusRunning
	job.StartedAt = time.Now()
	_ = r.jobs.Update(context.Background(), job)
	r.log(ctx, job.ID, fmt.Sprintf("Started %s job", job.Type))

	err := fn(ctx, func(message string) {
		r.log(ctx, job.ID, message)
	})

	job.FinishedAt = time.Now()
	if ctx.Err() != nil {
		job.Status = store.JobStatusCancelled
		job.Error = ctx.Err().Error()
		r.log(context.Background(), job.ID, fmt.Sprintf("Cancelled: %s", ctx.Err()))
	} else if err != nil {
		job.Status = store.JobStatusFailed
		job.Error = err.Error()
		r.log(ctx, job.ID, fmt.Sprintf("Failed: %s", err))
	} else {
		job.Status = store.JobStatusSucceeded
		r.log(ctx, job.ID, "Succeeded")
	}
	_ = r.jobs.Update(context.Background(), job)
}

func (r *Runner) Cancel(id string) bool {
	value, ok := r.running.Load(id)
	if !ok {
		return false
	}
	cancel := value.(context.CancelFunc)
	cancel()
	return true
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
	r.logger.Printf("[job:%s] %s", jobID, message)
}
