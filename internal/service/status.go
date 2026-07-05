package service

import (
	"context"
	"time"

	"deployctl/internal/docker"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

const (
	deploymentStateNotConfigured = "not_configured"
	deploymentStateNotCreated    = "not_created"
	deploymentStateRunning       = "running"
	deploymentStatePartial       = "partial"
	deploymentStateStopped       = "stopped"
)

func (s *Server) GetDeploymentStatus(ctx context.Context, req *rpc.GetDeploymentStatusRequest) (*rpc.DeploymentStatus, error) {
	repository, err := s.getRepository(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	return s.deploymentStatus(ctx, repository, req.EnvFile)
}

func (s *Server) ListDeploymentStatuses(ctx context.Context, _ *rpc.ListDeploymentStatusesRequest) (*rpc.ListDeploymentStatusesResponse, error) {
	repositories, err := s.repositories.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListDeploymentStatusesResponse{Deployments: make([]*rpc.DeploymentListItem, 0, len(repositories))}
	for _, repository := range repositories {
		item, err := deploymentListItem(ctx, repository)
		if err != nil {
			return nil, err
		}
		response.Deployments = append(response.Deployments, item)
	}
	return response, nil
}

func (s *Server) deploymentStatus(ctx context.Context, repository store.Repository, envFile string) (*rpc.DeploymentStatus, error) {
	envNames, err := listEnvNames(repository, envFile)
	if err != nil {
		return nil, err
	}

	response := &rpc.DeploymentStatus{
		Deployment: deploymentFromRepository(repository),
		State:      deploymentStateNotConfigured,
		EnvNames:   envNames,
	}

	jobs, err := s.jobs.List(ctx, repository.Name)
	if err != nil {
		return nil, err
	}
	response.LatestJob = latestJob(jobs)
	response.LatestUpdateJob = latestJobByType(jobs, "update")

	if repository.ComposePath == "" {
		return response, nil
	}

	composeStatus, err := docker.ComposeStatus(ctx, &repository)
	if err != nil {
		return nil, err
	}
	response.State = deploymentState(composeStatus)
	response.MissingServices = composeStatus.Missing
	response.Containers = make([]*rpc.ContainerStatus, 0, len(composeStatus.Containers))

	runningSince := earliestRunningStart(composeStatus.Containers)
	stoppedAt := latestStoppedFinish(composeStatus.Containers)
	response.RunningSinceUnix = unix(runningSince)
	response.StoppedAtUnix = unix(stoppedAt)

	for _, container := range composeStatus.Containers {
		response.Containers = append(response.Containers, &rpc.ContainerStatus{
			Service:        container.Service,
			Name:           container.Name,
			Status:         container.Status,
			State:          container.State,
			Image:          container.Image,
			ImageId:        container.ImageID,
			CreatedAtUnix:  unix(container.CreatedAt),
			StartedAtUnix:  unix(container.StartedAt),
			FinishedAtUnix: unix(container.FinishedAt),
		})
	}

	return response, nil
}

func deploymentListItem(ctx context.Context, repository store.Repository) (*rpc.DeploymentListItem, error) {
	item := &rpc.DeploymentListItem{
		Deployment: deploymentFromRepository(repository),
		State:      deploymentStateNotConfigured,
	}
	if repository.ComposePath == "" {
		return item, nil
	}
	composeStatus, err := docker.ComposeStatus(ctx, &repository)
	if err != nil {
		return nil, err
	}
	item.State = deploymentState(composeStatus)
	return item, nil
}

func deploymentState(status docker.DeploymentStatus) string {
	if status.AllRunning() {
		return deploymentStateRunning
	}
	if status.AnyRunning() {
		return deploymentStatePartial
	}
	if len(status.Containers) > 0 {
		return deploymentStateStopped
	}
	return deploymentStateNotCreated
}

func latestJob(jobs []store.Job) *rpc.Job {
	if len(jobs) == 0 {
		return nil
	}
	return statusJobToRPC(jobs[0])
}

func latestJobByType(jobs []store.Job, jobType string) *rpc.Job {
	for _, job := range jobs {
		if job.Type == jobType {
			return statusJobToRPC(job)
		}
	}
	return nil
}

func statusJobToRPC(job store.Job) *rpc.Job {
	response := jobToRPC(job)
	response.Error = ""
	return response
}

func earliestRunningStart(containers []docker.ContainerStatus) time.Time {
	var startedAt time.Time
	for _, container := range containers {
		if container.State != "running" || container.StartedAt.IsZero() {
			continue
		}
		if startedAt.IsZero() || container.StartedAt.Before(startedAt) {
			startedAt = container.StartedAt
		}
	}
	return startedAt
}

func latestStoppedFinish(containers []docker.ContainerStatus) time.Time {
	var finishedAt time.Time
	for _, container := range containers {
		if container.State == "running" || container.FinishedAt.IsZero() {
			continue
		}
		if finishedAt.IsZero() || container.FinishedAt.After(finishedAt) {
			finishedAt = container.FinishedAt
		}
	}
	return finishedAt
}
