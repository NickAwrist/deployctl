package service

import (
	"context"
	"time"

	"deployctl/internal/docker"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

func (s *Server) GetDeploymentStatus(ctx context.Context, req *rpc.GetDeploymentStatusRequest) (*rpc.DeploymentStatus, error) {
	repository, err := s.getRepository(ctx, req.DeploymentName)
	if err != nil {
		return nil, err
	}
	return s.deploymentStatus(ctx, repository, req.EnvFile)
}

func (s *Server) ListDeploymentSummaries(ctx context.Context, _ *rpc.ListDeploymentSummariesRequest) (*rpc.ListDeploymentSummariesResponse, error) {
	repositories, err := s.repositories.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	response := &rpc.ListDeploymentSummariesResponse{Deployments: make([]*rpc.DeploymentSummary, 0, len(repositories))}
	for _, repository := range repositories {
		item, err := deploymentSummary(ctx, repository)
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
		State:      rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CONFIGURED,
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
			State:          containerStateToRPC(container.State),
			Image:          container.Image,
			ImageId:        container.ImageID,
			CreatedAtUnix:  unix(container.CreatedAt),
			StartedAtUnix:  unix(container.StartedAt),
			FinishedAtUnix: unix(container.FinishedAt),
		})
	}

	return response, nil
}

func deploymentSummary(ctx context.Context, repository store.Repository) (*rpc.DeploymentSummary, error) {
	item := &rpc.DeploymentSummary{
		Deployment: deploymentFromRepository(repository),
		State:      rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CONFIGURED,
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

func deploymentState(status docker.DeploymentStatus) rpc.DeploymentState {
	if status.AllRunning() {
		return rpc.DeploymentState_DEPLOYMENT_STATE_RUNNING
	}
	if status.AnyRunning() {
		return rpc.DeploymentState_DEPLOYMENT_STATE_PARTIAL
	}
	if len(status.Containers) > 0 {
		return rpc.DeploymentState_DEPLOYMENT_STATE_STOPPED
	}
	return rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CREATED
}

func containerStateToRPC(state string) rpc.ContainerState {
	switch state {
	case "created":
		return rpc.ContainerState_CONTAINER_STATE_CREATED
	case "restarting":
		return rpc.ContainerState_CONTAINER_STATE_RESTARTING
	case "running":
		return rpc.ContainerState_CONTAINER_STATE_RUNNING
	case "removing":
		return rpc.ContainerState_CONTAINER_STATE_REMOVING
	case "paused":
		return rpc.ContainerState_CONTAINER_STATE_PAUSED
	case "exited":
		return rpc.ContainerState_CONTAINER_STATE_EXITED
	case "dead":
		return rpc.ContainerState_CONTAINER_STATE_DEAD
	default:
		return rpc.ContainerState_CONTAINER_STATE_UNSPECIFIED
	}
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
