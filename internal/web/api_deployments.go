package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"deployctl/internal/docker"
	"deployctl/internal/rpc"
	"deployctl/internal/store"
)

type ContainerStatus struct {
	Service string `json:"service"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Image   string `json:"image,omitempty"`
	Created int64  `json:"created,omitempty"`
}

type DeploymentContainersStatus struct {
	Containers []ContainerStatus `json:"containers"`
	Missing    []string          `json:"missing"`
}

type DeploymentListItem struct {
	Name        string                     `json:"name"`
	URL         string                     `json:"url"`
	Location    string                     `json:"location"`
	ComposePath string                     `json:"compose_path"`
	EnvPath     string                     `json:"env_path"`
	Status      DeploymentContainersStatus `json:"status"`
	State       string                     `json:"state"`
	StatusError string                     `json:"status_error,omitempty"`
	Summary     string                     `json:"summary"`
}

type DeploymentDetailResponse struct {
	DeploymentListItem
	BuildCache BuildCacheResponse `json:"build_cache"`
	EnvNames   []string           `json:"env_names"`
}

type BuildCacheResponse struct {
	Tags    []string `json:"tags"`
	Missing []string `json:"missing"`
}

type CreateDeploymentPayload struct {
	RepoURL     string `json:"repo_url"`
	Name        string `json:"name"`
	ComposeFile string `json:"compose_file"`
	EnvFile     string `json:"env_file"`
}

type ActionPayload struct {
	Build bool `json:"build"`
}

type SetEnvPayload struct {
	Variables map[string]string `json:"variables"`
	EnvFile   string            `json:"env_file"`
}

type UnsetEnvPayload struct {
	Names   []string `json:"names"`
	EnvFile string   `json:"env_file"`
}

func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.ListDeployments(r.Context(), &rpc.ListDeploymentsRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]DeploymentListItem, 0, len(response.Deployments))
	for _, deployment := range response.Deployments {
		status, statusErr := s.service.GetDeploymentStatus(r.Context(), &rpc.GetDeploymentStatusRequest{
			DeploymentName: deployment.Name,
		})
		items = append(items, deploymentListItem(deployment, status, statusErr))
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "deployment name is required")
		return
	}

	deployment, err := s.service.GetDeployment(r.Context(), &rpc.GetDeploymentRequest{DeploymentName: name})
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	status, statusErr := s.service.GetDeploymentStatus(r.Context(), &rpc.GetDeploymentStatusRequest{
		DeploymentName: name,
	})
	cache, _ := docker.ComposeBuildCache(r.Context(), repositoryFromDeployment(deployment))

	var envNames []string
	envResponse, err := s.service.ListEnvNames(r.Context(), &rpc.ListEnvNamesRequest{
		DeploymentName: name,
	})
	if err == nil {
		envNames = envResponse.Names
	}

	writeJSON(w, http.StatusOK, DeploymentDetailResponse{
		DeploymentListItem: deploymentListItem(deployment, status, statusErr),
		BuildCache: BuildCacheResponse{
			Tags:    cache.Tags,
			Missing: cache.Missing,
		},
		EnvNames: envNames,
	})
}

func (s *Server) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	var payload CreateDeploymentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if payload.RepoURL == "" {
		writeError(w, http.StatusBadRequest, "repo_url is required")
		return
	}

	response, err := s.service.CreateDeployment(r.Context(), &rpc.CreateDeploymentRequest{
		RepoUrl:        payload.RepoURL,
		DeploymentName: payload.Name,
		ComposeFile:    payload.ComposeFile,
		EnvFile:        payload.EnvFile,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": response.JobId})
}

func (s *Server) handleDeleteDeployment(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.DeleteDeployment(r.Context(), &rpc.DeleteDeploymentRequest{
		DeploymentName: r.PathValue("name"),
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleDeployDeployment(w http.ResponseWriter, r *http.Request) {
	var payload ActionPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)

	response, err := s.service.DeployDeployment(r.Context(), &rpc.DeployDeploymentRequest{
		DeploymentName: r.PathValue("name"),
		Build:          payload.Build,
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleRestartDeployment(w http.ResponseWriter, r *http.Request) {
	var payload ActionPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)

	response, err := s.service.RestartDeployment(r.Context(), &rpc.RestartDeploymentRequest{
		DeploymentName: r.PathValue("name"),
		Build:          payload.Build,
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleStopDeployment(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.StopDeployment(r.Context(), &rpc.StopDeploymentRequest{
		DeploymentName: r.PathValue("name"),
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleUpdateDeployment(w http.ResponseWriter, r *http.Request) {
	var payload ActionPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)

	response, err := s.service.UpdateDeployment(r.Context(), &rpc.UpdateDeploymentRequest{
		DeploymentName: r.PathValue("name"),
		Build:          payload.Build,
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleListEnv(w http.ResponseWriter, r *http.Request) {
	response, err := s.service.ListEnvNames(r.Context(), &rpc.ListEnvNamesRequest{
		DeploymentName: r.PathValue("name"),
		EnvFile:        r.URL.Query().Get("file"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSetEnv(w http.ResponseWriter, r *http.Request) {
	var payload SetEnvPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := s.service.SetEnv(r.Context(), &rpc.SetEnvRequest{
		DeploymentName: r.PathValue("name"),
		Variables:      payload.Variables,
		EnvFile:        payload.EnvFile,
	})
	writeJobResponse(w, response, err)
}

func (s *Server) handleUnsetEnv(w http.ResponseWriter, r *http.Request) {
	var payload UnsetEnvPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := s.service.UnsetEnv(r.Context(), &rpc.UnsetEnvRequest{
		DeploymentName: r.PathValue("name"),
		Names:          payload.Names,
		EnvFile:        payload.EnvFile,
	})
	writeJobResponse(w, response, err)
}

func deploymentListItem(deployment *rpc.Deployment, status *rpc.DeploymentStatus, statusErr error) DeploymentListItem {
	item := DeploymentListItem{
		Name:        deployment.GetName(),
		URL:         deployment.GetRepoUrl(),
		Location:    deployment.GetLocation(),
		ComposePath: deployment.GetComposePath(),
		EnvPath:     deployment.GetEnvPath(),
		Status: DeploymentContainersStatus{
			Containers: []ContainerStatus{},
			Missing:    []string{},
		},
		State: "unknown",
	}
	if statusErr != nil {
		item.State = "unavailable"
		item.StatusError = statusErr.Error()
		if message, ok := docker.UnavailableMessage(statusErr); ok {
			item.StatusError = message
		}
		return item
	}
	if status == nil {
		return item
	}

	item.Status.Missing = status.GetMissingServices()
	for _, container := range status.GetContainers() {
		item.Status.Containers = append(item.Status.Containers, ContainerStatus{
			Service: container.GetService(),
			Name:    container.GetName(),
			Status:  container.GetStatus(),
			State:   containerState(container.GetState()),
			Image:   container.GetImage(),
			Created: container.GetStartedAtUnix(),
		})
	}
	item.State = deploymentStateName(status.GetState())
	item.Summary = deploymentSummary(item.Status.Containers)
	return item
}

func deploymentStateName(state rpc.DeploymentState) string {
	switch state {
	case rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CONFIGURED:
		return "not_configured"
	case rpc.DeploymentState_DEPLOYMENT_STATE_NOT_CREATED:
		return "not_created"
	case rpc.DeploymentState_DEPLOYMENT_STATE_RUNNING:
		return "running"
	case rpc.DeploymentState_DEPLOYMENT_STATE_PARTIAL:
		return "partial"
	case rpc.DeploymentState_DEPLOYMENT_STATE_STOPPED:
		return "stopped"
	default:
		return "unknown"
	}
}

func deploymentSummary(containers []ContainerStatus) string {
	parts := make([]string, 0, len(containers))
	for _, container := range containers {
		if container.State == "running" {
			parts = append(parts, container.Name+" ("+container.Status+")")
		}
	}
	return strings.Join(parts, ", ")
}

func containerState(state rpc.ContainerState) string {
	switch state {
	case rpc.ContainerState_CONTAINER_STATE_CREATED:
		return "created"
	case rpc.ContainerState_CONTAINER_STATE_RESTARTING:
		return "restarting"
	case rpc.ContainerState_CONTAINER_STATE_RUNNING:
		return "running"
	case rpc.ContainerState_CONTAINER_STATE_REMOVING:
		return "removing"
	case rpc.ContainerState_CONTAINER_STATE_PAUSED:
		return "paused"
	case rpc.ContainerState_CONTAINER_STATE_EXITED:
		return "exited"
	case rpc.ContainerState_CONTAINER_STATE_DEAD:
		return "dead"
	default:
		return "unknown"
	}
}

func repositoryFromDeployment(deployment *rpc.Deployment) *store.Repository {
	return &store.Repository{
		Name:        deployment.GetName(),
		URL:         deployment.GetRepoUrl(),
		Location:    deployment.GetLocation(),
		ComposePath: deployment.GetComposePath(),
		EnvPath:     deployment.GetEnvPath(),
	}
}

func writeJobResponse(w http.ResponseWriter, response *rpc.JobResponse, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": response.JobId})
}
