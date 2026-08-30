import {
  CreateDeploymentInput,
  DeploymentDetail,
  DeploymentListItem,
  Job,
  JobLog,
  SystemHealth,
} from './types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    let errorMsg = `HTTP ${response.status}: ${response.statusText}`;
    try {
      const body = await response.json();
      if (body.error) errorMsg = body.error;
    } catch {
      // ignore
    }
    throw new Error(errorMsg);
  }

  return response.json() as Promise<T>;
}

export const api = {
  getHealth: () => fetchJSON<SystemHealth>('/system/health'),

  listDeployments: () => fetchJSON<DeploymentListItem[]>('/deployments'),

  getDeployment: (name: string) => fetchJSON<DeploymentDetail>(`/deployments/${encodeURIComponent(name)}`),

  createDeployment: (data: CreateDeploymentInput) =>
    fetchJSON<{ job_id: string }>('/deployments', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  deleteDeployment: (name: string) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),

  deployDeployment: (name: string, build = false) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/deploy`, {
      method: 'POST',
      body: JSON.stringify({ build }),
    }),

  restartDeployment: (name: string, build = false) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/restart`, {
      method: 'POST',
      body: JSON.stringify({ build }),
    }),

  stopDeployment: (name: string) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/stop`, {
      method: 'POST',
    }),

  updateDeployment: (name: string, build = false) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/update`, {
      method: 'POST',
      body: JSON.stringify({ build }),
    }),

  listEnv: (name: string, file?: string) => {
    const q = file ? `?file=${encodeURIComponent(file)}` : '';
    return fetchJSON<{ names: string[] }>(`/deployments/${encodeURIComponent(name)}/env${q}`);
  },

  setEnv: (name: string, variables: Record<string, string>, envFile?: string) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/env`, {
      method: 'POST',
      body: JSON.stringify({ variables, env_file: envFile }),
    }),

  unsetEnv: (name: string, names: string[], envFile?: string) =>
    fetchJSON<{ job_id: string }>(`/deployments/${encodeURIComponent(name)}/env`, {
      method: 'DELETE',
      body: JSON.stringify({ names, env_file: envFile }),
    }),

  listJobs: (deployment?: string) => {
    const q = deployment ? `?deployment=${encodeURIComponent(deployment)}` : '';
    return fetchJSON<Job[]>(`/jobs${q}`);
  },

  getJob: (id: string) => fetchJSON<Job>(`/jobs/${encodeURIComponent(id)}`),

  cancelJob: (id: string) =>
    fetchJSON<{ id: string; status: string }>(`/jobs/${encodeURIComponent(id)}/cancel`, {
      method: 'POST',
    }),

  streamJobLogs: (
    jobId: string,
    callbacks: {
      onLog: (log: JobLog) => void;
      onStatus: (job: Job) => void;
      onDone: (status: string) => void;
      onError?: (err: Event) => void;
    }
  ) => {
    const eventSource = new EventSource(`${API_BASE}/jobs/${encodeURIComponent(jobId)}/events`);

    eventSource.addEventListener('log', (e: MessageEvent) => {
      try {
        const log = JSON.parse(e.data) as JobLog;
        callbacks.onLog(log);
      } catch (err) {
        console.error('Failed to parse log event:', err);
      }
    });

    eventSource.addEventListener('status', (e: MessageEvent) => {
      try {
        const job = JSON.parse(e.data) as Job;
        callbacks.onStatus(job);
      } catch (err) {
        console.error('Failed to parse status event:', err);
      }
    });

    eventSource.addEventListener('done', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data) as { status: string };
        callbacks.onDone(data.status);
      } catch {
        callbacks.onDone('completed');
      }
      eventSource.close();
    });

    eventSource.onerror = (err) => {
      callbacks.onError?.(err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  },
};
