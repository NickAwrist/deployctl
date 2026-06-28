package service

import (
	"context"
	"time"

	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

func (s *Server) GetJob(ctx context.Context, req *rpc.GetJobRequest) (*rpc.Job, error) {
	job, err := s.jobs.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func (s *Server) ListJobs(ctx context.Context, req *rpc.ListJobsRequest) (*rpc.ListJobsResponse, error) {
	jobs, err := s.jobs.List(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListJobsResponse{Jobs: make([]*rpc.Job, 0, len(jobs))}
	for _, job := range jobs {
		response.Jobs = append(response.Jobs, jobToRPC(job))
	}
	return response, nil
}

func (s *Server) WatchJob(req *rpc.WatchJobRequest, stream rpc.JobService_WatchJobServer) error {
	after := req.AfterSequence
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, err := s.jobs.LogsAfter(stream.Context(), req.Id, after)
		if err != nil {
			return err
		}
		for _, log := range logs {
			after = log.Sequence
			if err := stream.Send(&rpc.JobEvent{
				JobId:    log.JobID,
				Sequence: log.Sequence,
				Message:  log.Message,
			}); err != nil {
				return err
			}
		}

		job, err := s.jobs.Get(stream.Context(), req.Id)
		if err != nil {
			return err
		}
		if isTerminal(job.Status) {
			return stream.Send(&rpc.JobEvent{JobId: job.ID, Sequence: after, Job: jobToRPC(job)})
		}

		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
		}
	}
}

func (s *Server) CancelJob(ctx context.Context, req *rpc.CancelJobRequest) (*rpc.Job, error) {
	job, err := s.jobs.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if isTerminal(job.Status) {
		return jobToRPC(job), nil
	}
	job.Status = store.JobStatusCancelled
	job.Error = "cancel requested"
	job.FinishedAt = time.Now()
	if err := s.jobs.Update(ctx, job); err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func jobToRPC(job store.Job) *rpc.Job {
	return &rpc.Job{
		Id:             job.ID,
		Type:           job.Type,
		DeploymentName: job.DeploymentName,
		Status:         job.Status,
		Error:          job.Error,
		CreatedAtUnix:  unix(job.CreatedAt),
		StartedAtUnix:  unix(job.StartedAt),
		FinishedAtUnix: unix(job.FinishedAt),
	}
}

func isTerminal(status string) bool {
	return status == store.JobStatusSucceeded || status == store.JobStatusFailed || status == store.JobStatusCancelled
}
