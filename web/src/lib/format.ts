import { ContainerStatus, DeploymentListItem, Job } from './types';

export type DeploymentState = 'running' | 'degraded' | 'stopped';

export function deploymentState(deployment: DeploymentListItem): DeploymentState {
  if (deployment.all_running) return 'running';
  if (deployment.any_running) return 'degraded';
  return 'stopped';
}

export function formatDuration(durationMs: number): string {
  if (durationMs < 1_000) return '<1s';
  if (durationMs < 60_000) return `${Math.round(durationMs / 1_000)}s`;
  return `${Math.round(durationMs / 60_000)}m`;
}

export function formatRelativeTime(value: string): string {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp) || timestamp <= 0) return 'Not started';

  const seconds = Math.round((timestamp - Date.now()) / 1_000);
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
  if (Math.abs(seconds) < 60) return formatter.format(seconds, 'second');
  const minutes = Math.round(seconds / 60);
  if (Math.abs(minutes) < 60) return formatter.format(minutes, 'minute');
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, 'hour');
  const days = Math.round(hours / 24);
  if (Math.abs(days) < 30) return formatter.format(days, 'day');
  return new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function deploymentUptime(containers: ContainerStatus[] | null): string {
  if (!containers?.length) return 'Offline';
  const newestContainer = Math.max(...containers.map((container) => container.created || 0));
  if (newestContainer > 0) return formatUptimeSeconds(Date.now() / 1_000 - newestContainer);

  const statusUptime = containers[0]?.status.match(/Up\s+(.+?)(?:\s+\(|$)/i)?.[1];
  return statusUptime ?? 'Running';
}

export function formatUptimeSeconds(rawSeconds: number): string {
  const seconds = Math.max(0, Math.floor(rawSeconds));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h ${minutes % 60}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

export function latestJobByDeployment(jobs: Job[]): Map<string, Job> {
  const result = new Map<string, Job>();
  for (const job of jobs) {
    if (!result.has(job.deployment_name)) result.set(job.deployment_name, job);
  }
  return result;
}

export function repositoryLabel(url: string): string {
  const normalized = url.replace(/\.git$/, '').replace(/\\/g, '/');
  const parts = normalized.split(/[/:]/).filter(Boolean);
  return parts.slice(-2).join('/');
}

export function fileName(path: string): string {
  return path.split(/[\\/]/).filter(Boolean).at(-1) ?? path;
}
