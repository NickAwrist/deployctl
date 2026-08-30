export interface DockerConnectionStatus {
  connected: boolean;
  host: string;
  server_version: string;
  api_version: string;
  os_type: string;
  error?: string;
}

export interface SystemHealth {
  status: 'ok' | 'degraded' | 'unavailable';
  docker: DockerConnectionStatus;
}

export interface ContainerStatus {
  service: string;
  name: string;
  status: string;
  state: 'running' | 'exited' | 'paused' | 'restarting' | 'dead' | string;
  image?: string;
  created?: number;
}

export interface DeploymentContainersStatus {
  containers: ContainerStatus[] | null;
  missing: string[] | null;
}

export interface DeploymentListItem {
  name: string;
  url: string;
  location: string;
  compose_path: string;
  env_path: string;
  status: DeploymentContainersStatus;
  all_running: boolean;
  any_running: boolean;
  summary: string;
}

export interface BuildCache {
  tags: string[] | null;
  missing: string[] | null;
}

export interface DeploymentDetail extends DeploymentListItem {
  build_cache: BuildCache;
  env_names: string[] | null;
}

export interface Job {
  id: string;
  type: 'create' | 'delete' | 'deploy' | 'restart' | 'stop' | 'update' | 'env.set' | 'env.import' | 'env.unset' | string;
  deployment_name: string;
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';
  error: string;
  created_at: string;
  started_at: string;
  finished_at: string;
  duration_ms: number;
}

export interface JobLog {
  job_id: string;
  sequence: number;
  message: string;
  created_at: string;
}

export interface CreateDeploymentInput {
  repo_url: string;
  name?: string;
  compose_file?: string;
  env_file?: string;
}
