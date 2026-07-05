package service

import (
	"context"
	"time"

	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

func (s *Server) GetJob(ctx context.Context, req *rpc.GetJobRequest) (*rpc.Job, error) {
	job, err := s.getJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func (s *Server) ListJobs(ctx context.Context, req *rpc.ListJobsRequest) (*rpc.ListJobsResponse, error) {
	if req.DeploymentName != "" {
		if _, err := s.getRepository(ctx, req.DeploymentName); err != nil {
			return nil, err
		}
	}
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

func (s *Server) ListJobLogs(ctx context.Context, req *rpc.ListJobLogsRequest) (*rpc.ListJobLogsResponse, error) {
	if _, err := s.getJob(ctx, req.JobId); err != nil {
		return nil, err
	}
	logs, err := s.jobs.LogsAfter(ctx, req.JobId, 0)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListJobLogsResponse{Logs: make([]*rpc.JobLog, 0, len(logs))}
	for _, log := range logs {
		response.Logs = append(response.Logs, jobLogToRPC(log))
	}
	return response, nil
}

func (s *Server) WatchJob(req *rpc.WatchJobRequest, stream rpc.JobService_WatchJobServer) error {
	after := req.AfterSequence
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, err := s.jobs.LogsAfter(stream.Context(), req.JobId, after)
		if err != nil {
			return normalizeRPCError(err)
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

		job, err := s.getJob(stream.Context(), req.JobId)
		if err != nil {
			return normalizeRPCError(err)
		}
		if isTerminal(job.Status) {
			return stream.Send(&rpc.JobEvent{JobId: job.ID, Sequence: after, Job: jobToRPC(job)})
		}

		select {
		case <-stream.Context().Done():
			return normalizeRPCError(stream.Context().Err())
		case <-ticker.C:
		}
	}
}

func (s *Server) CancelJob(ctx context.Context, req *rpc.CancelJobRequest) (*rpc.Job, error) {
	job, err := s.getJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	if isTerminal(job.Status) {
		return jobToRPC(job), nil
	}

	if !s.runner.Cancel(req.JobId) {
		return jobToRPC(job), nil
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err = s.getJob(ctx, req.JobId)
		if err != nil {
			return nil, err
		}
		if isTerminal(job.Status) {
			return jobToRPC(job), nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}

	job, err = s.getJob(ctx, req.JobId)
	if err != nil {
		return nil, err
	}
	return jobToRPC(job), nil
}

func jobLogToRPC(log store.JobLog) *rpc.JobLog {
	return &rpc.JobLog{
		JobId:         log.JobID,
		Sequence:      log.Sequence,
		Message:       log.Message,
		CreatedAtUnix: unix(log.CreatedAt),
	}
}

func jobToRPC(job store.Job) *rpc.Job {
	return &rpc.Job{
		Id:             job.ID,
		Type:           jobTypeToRPC(job.Type),
		DeploymentName: job.DeploymentName,
		Status:         jobStatusToRPC(job.Status),
		Error:          job.Error,
		CreatedAtUnix:  unix(job.CreatedAt),
		StartedAtUnix:  unix(job.StartedAt),
		FinishedAtUnix: unix(job.FinishedAt),
	}
}

func jobTypeToRPC(jobType string) rpc.JobType {
	switch jobType {
	case "create":
		return rpc.JobType_JOB_TYPE_CREATE
	case "delete":
		return rpc.JobType_JOB_TYPE_DELETE
	case "build":
		return rpc.JobType_JOB_TYPE_BUILD
	case "update":
		return rpc.JobType_JOB_TYPE_UPDATE
	case "deploy":
		return rpc.JobType_JOB_TYPE_DEPLOY
	case "restart":
		return rpc.JobType_JOB_TYPE_RESTART
	case "stop":
		return rpc.JobType_JOB_TYPE_STOP
	case "env.set":
		return rpc.JobType_JOB_TYPE_ENV_SET
	case "env.import":
		return rpc.JobType_JOB_TYPE_ENV_IMPORT
	case "env.unset":
		return rpc.JobType_JOB_TYPE_ENV_UNSET
	default:
		return rpc.JobType_JOB_TYPE_UNSPECIFIED
	}
}

func jobStatusToRPC(status string) rpc.JobStatus {
	switch status {
	case store.JobStatusQueued:
		return rpc.JobStatus_JOB_STATUS_QUEUED
	case store.JobStatusRunning:
		return rpc.JobStatus_JOB_STATUS_RUNNING
	case store.JobStatusSucceeded:
		return rpc.JobStatus_JOB_STATUS_SUCCEEDED
	case store.JobStatusFailed:
		return rpc.JobStatus_JOB_STATUS_FAILED
	case store.JobStatusCancelled:
		return rpc.JobStatus_JOB_STATUS_CANCELLED
	default:
		return rpc.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func isTerminal(status string) bool {
	return status == store.JobStatusSucceeded || status == store.JobStatusFailed || status == store.JobStatusCancelled
}
