package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"deployctl/internal/rpc"
)

type JobResponse struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	DeploymentName string    `json:"deployment_name"`
	Status         string    `json:"status"`
	Error          string    `json:"error"`
	CreatedAt      time.Time `json:"created_at"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMs     int64     `json:"duration_ms"`
}

func jobToResponse(job *rpc.Job) JobResponse {
	startedAt := timeFromUnix(job.GetStartedAtUnix())
	finishedAt := timeFromUnix(job.GetFinishedAtUnix())
	var durationMs int64
	if !startedAt.IsZero() {
		end := finishedAt
		if end.IsZero() {
			end = time.Now()
		}
		durationMs = end.Sub(startedAt).Milliseconds()
	}

	return JobResponse{
		ID:             job.GetId(),
		Type:           jobType(job.GetType()),
		DeploymentName: job.GetDeploymentName(),
		Status:         jobStatus(job.GetStatus()),
		Error:          job.GetError(),
		CreatedAt:      timeFromUnix(job.GetCreatedAtUnix()),
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
		DurationMs:     durationMs,
	}
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.ListJobs(r.Context(), &rpc.ListJobsRequest{
		DeploymentName: r.URL.Query().Get("deployment"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jobs := make([]JobResponse, 0, len(response.Jobs))
	for _, job := range response.Jobs {
		jobs = append(jobs, jobToResponse(job))
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	job, err := s.service.GetJob(r.Context(), &rpc.GetJobRequest{JobId: id})
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, jobToResponse(job))
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	job, err := s.service.CancelJob(r.Context(), &rpc.CancelJobRequest{JobId: id})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     job.GetId(),
		"status": jobStatus(job.GetStatus()),
	})
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}
	if _, err := s.service.GetJob(r.Context(), &rpc.GetJobRequest{JobId: id}); err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	var after int64
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, err := s.service.ListJobLogs(r.Context(), &rpc.ListJobLogsRequest{
			JobId:         id,
			AfterSequence: after,
		})
		if err != nil {
			return
		}
		for _, log := range logs.Logs {
			after = log.GetSequence()
			logData, _ := json.Marshal(map[string]any{
				"job_id":     log.GetJobId(),
				"sequence":   log.GetSequence(),
				"message":    log.GetMessage(),
				"created_at": timeFromUnix(log.GetCreatedAtUnix()),
			})
			fmt.Fprintf(w, "event: log\ndata: %s\n\n", logData)
			flusher.Flush()
		}

		job, err := s.service.GetJob(r.Context(), &rpc.GetJobRequest{JobId: id})
		if err != nil {
			return
		}
		if isTerminalJobStatus(job.GetStatus()) {
			statusData, _ := json.Marshal(jobToResponse(job))
			fmt.Fprintf(w, "event: status\ndata: %s\n\n", statusData)
			fmt.Fprintf(w, "event: done\ndata: {\"status\": %q}\n\n", jobStatus(job.GetStatus()))
			flusher.Flush()
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func jobType(jobType rpc.JobType) string {
	switch jobType {
	case rpc.JobType_JOB_TYPE_CREATE:
		return "create"
	case rpc.JobType_JOB_TYPE_DELETE:
		return "delete"
	case rpc.JobType_JOB_TYPE_BUILD:
		return "build"
	case rpc.JobType_JOB_TYPE_UPDATE:
		return "update"
	case rpc.JobType_JOB_TYPE_DEPLOY:
		return "deploy"
	case rpc.JobType_JOB_TYPE_RESTART:
		return "restart"
	case rpc.JobType_JOB_TYPE_STOP:
		return "stop"
	case rpc.JobType_JOB_TYPE_ENV_SET:
		return "env.set"
	case rpc.JobType_JOB_TYPE_ENV_IMPORT:
		return "env.import"
	case rpc.JobType_JOB_TYPE_ENV_UNSET:
		return "env.unset"
	default:
		return "unknown"
	}
}

func jobStatus(status rpc.JobStatus) string {
	switch status {
	case rpc.JobStatus_JOB_STATUS_QUEUED:
		return "queued"
	case rpc.JobStatus_JOB_STATUS_RUNNING:
		return "running"
	case rpc.JobStatus_JOB_STATUS_SUCCEEDED:
		return "succeeded"
	case rpc.JobStatus_JOB_STATUS_FAILED:
		return "failed"
	case rpc.JobStatus_JOB_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
	}
}

func isTerminalJobStatus(status rpc.JobStatus) bool {
	return status == rpc.JobStatus_JOB_STATUS_SUCCEEDED ||
		status == rpc.JobStatus_JOB_STATUS_FAILED ||
		status == rpc.JobStatus_JOB_STATUS_CANCELLED
}

func timeFromUnix(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}
