import { FormEvent, useCallback, useEffect, useState } from 'react';
import {
  ArrowDownToLine,
  ArrowLeft,
  Box,
  Clock3,
  FileCode2,
  FolderGit2,
  KeyRound,
  LoaderCircle,
  Play,
  Plus,
  RotateCw,
  Square,
  Terminal,
  Trash2,
  X,
} from 'lucide-react';
import { api } from '../lib/api';
import { DeploymentDetail, Job } from '../lib/types';
import { deploymentState, deploymentUptime, formatDuration, formatRelativeTime } from '../lib/format';
import { useModalDismiss } from '../lib/useModalDismiss';
import { StatusBadge } from './StatusBadge';

type DetailTab = 'services' | 'history' | 'environment';

interface DeploymentDetailViewProps {
  deploymentName: string;
  onBack: () => void;
  onDeploy: (name: string, build?: boolean) => Promise<void>;
  onRestart: (name: string, build?: boolean) => Promise<void>;
  onStop: (name: string) => Promise<void>;
  onUpdate: (name: string, build?: boolean) => Promise<void>;
  onDelete: (name: string) => Promise<void>;
  onViewJobLogs: (jobId: string) => void;
}

export function DeploymentDetailView({
  deploymentName,
  onBack,
  onDeploy,
  onRestart,
  onStop,
  onUpdate,
  onDelete,
  onViewJobLogs,
}: DeploymentDetailViewProps) {
  const [deployment, setDeployment] = useState<DeploymentDetail | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<DetailTab>('services');
  const [rebuild, setRebuild] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [envKey, setEnvKey] = useState('');
  const [envValue, setEnvValue] = useState('');
  const [envLoading, setEnvLoading] = useState(false);

  const fetchDetails = useCallback(async (background = false) => {
    if (!background) setLoading(true);
    try {
      const [deploymentData, jobsData] = await Promise.all([
        api.getDeployment(deploymentName),
        api.listJobs(deploymentName),
      ]);
      setDeployment(deploymentData);
      setJobs(jobsData);
      setError(null);
    } catch (requestError: unknown) {
      setError(requestError instanceof Error ? requestError.message : 'Could not load this deployment.');
    } finally {
      setLoading(false);
    }
  }, [deploymentName]);

  useEffect(() => {
    void fetchDetails();
    const interval = window.setInterval(() => void fetchDetails(true), 10_000);
    return () => window.clearInterval(interval);
  }, [fetchDetails]);

  const setEnvironmentVariable = async (event: FormEvent) => {
    event.preventDefault();
    if (!envKey.trim()) return;

    setEnvLoading(true);
    try {
      const response = await api.setEnv(deploymentName, { [envKey.trim()]: envValue });
      setEnvKey('');
      setEnvValue('');
      onViewJobLogs(response.job_id);
    } finally {
      setEnvLoading(false);
    }
  };

  const removeEnvironmentVariable = async (name: string) => {
    setEnvLoading(true);
    try {
      const response = await api.unsetEnv(deploymentName, [name]);
      onViewJobLogs(response.job_id);
    } finally {
      setEnvLoading(false);
    }
  };

  if (loading && !deployment) {
    return <DetailLoading />;
  }

  if (!deployment) {
    return (
      <div className="grid min-h-80 place-items-center border-y border-white/[0.08] text-center">
        <div>
          <p className="text-sm text-stone-300">{error ?? 'Deployment not found'}</p>
          <button onClick={onBack} className="mt-4 text-xs text-amber-300 hover:text-amber-200">Return to deployments</button>
        </div>
      </div>
    );
  }

  const state = deploymentState(deployment);
  const hasRunningServices = state === 'running' || state === 'partial';
  const containers = deployment.status.containers ?? [];
  const missing = deployment.status.missing ?? [];

  return (
    <div>
      <button onClick={onBack} className="mb-6 flex items-center gap-2 text-xs text-stone-600 transition-colors hover:text-stone-200">
        <ArrowLeft className="h-3.5 w-3.5" />
        All deployments
      </button>

      <section className="flex flex-col gap-6 border-b border-white/[0.1] pb-7 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0">
          <div className="mb-3 flex items-center gap-3">
            <StatusBadge status={state} size="md" />
            <span className="font-mono text-[10px] text-stone-700">{containers.length} services</span>
          </div>
          <h1 className="truncate text-3xl font-semibold tracking-[-0.045em] text-white sm:text-4xl">{deployment.name}</h1>
          <p className="mt-2 truncate font-mono text-[11px] text-stone-600">{deployment.url}</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <label className="mr-1 flex h-9 cursor-pointer items-center gap-2 border border-white/[0.08] px-3 text-xs text-stone-400 hover:border-white/[0.16]">
            <input
              type="checkbox"
              checked={rebuild}
              onChange={(event) => setRebuild(event.target.checked)}
              className="accent-amber-300"
            />
            Rebuild images
          </label>
          {state !== 'running' && <ActionButton icon={Play} label="Deploy" primary onClick={() => void onDeploy(deployment.name, rebuild)} />}
          {hasRunningServices && <ActionButton icon={RotateCw} label="Restart" onClick={() => void onRestart(deployment.name, rebuild)} />}
          <ActionButton icon={ArrowDownToLine} label="Pull" onClick={() => void onUpdate(deployment.name, rebuild)} />
          {hasRunningServices && <ActionButton icon={Square} label="Stop" danger onClick={() => void onStop(deployment.name)} />}
        </div>
      </section>

      {(error || deployment.status_error) && (
        <p className="mt-4 border-l-2 border-rose-500 pl-3 text-xs text-rose-300">
          {error ?? deployment.status_error}
        </p>
      )}

      <div className="mt-8 grid gap-10 lg:grid-cols-[270px_minmax(0,1fr)]">
        <aside>
          <h2 className="mb-4 font-mono text-[9px] uppercase tracking-[0.16em] text-stone-600">Metadata</h2>
          <dl className="divide-y divide-white/[0.07] border-y border-white/[0.08]">
            <MetadataRow icon={Clock3} label="Uptime" value={deploymentUptime(deployment.status.containers)} />
            <MetadataRow icon={FolderGit2} label="Repository" value={deployment.location} mono />
            <MetadataRow icon={FileCode2} label="Compose" value={deployment.compose_path} mono />
            <MetadataRow icon={KeyRound} label="Environment" value={deployment.env_path || 'Default .env'} mono />
          </dl>

          <div className="mt-7">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="font-mono text-[9px] uppercase tracking-[0.16em] text-stone-600">Build images</h2>
              <span className="font-mono text-[9px] text-stone-700">{deployment.build_cache.tags?.length ?? 0}</span>
            </div>
            {deployment.build_cache.tags?.length ? (
              <ul className="space-y-2">
                {deployment.build_cache.tags.map((tag) => (
                  <li key={tag} className="flex items-center gap-2 font-mono text-[10px] text-stone-500">
                    <Box className="h-3 w-3 shrink-0 text-stone-700" />
                    <span className="truncate" title={tag}>{tag}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-stone-700">No local build images.</p>
            )}
          </div>

          <button onClick={() => setDeleteOpen(true)} className="mt-10 flex items-center gap-2 text-xs text-stone-700 transition-colors hover:text-rose-400">
            <Trash2 className="h-3.5 w-3.5" />
            Delete deployment
          </button>
        </aside>

        <section className="min-w-0">
          <div className="flex gap-1 border-b border-white/[0.08]">
            {(['services', 'history', 'environment'] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`relative px-3 pb-3 text-xs capitalize transition-colors ${activeTab === tab ? 'text-white' : 'text-stone-600 hover:text-stone-300'}`}
              >
                {tab}
                {tab === 'services' && ` ${containers.length}`}
                {tab === 'history' && ` ${jobs.length}`}
                {activeTab === tab && <span className="absolute inset-x-2 -bottom-px h-px bg-amber-300" />}
              </button>
            ))}
          </div>

          <div className="pt-4">
            {activeTab === 'services' && <ServicesTable containers={containers} missing={missing} />}
            {activeTab === 'history' && <HistoryTable jobs={jobs} onViewJobLogs={onViewJobLogs} />}
            {activeTab === 'environment' && (
              <EnvironmentEditor
                names={deployment.env_names ?? []}
                envKey={envKey}
                envValue={envValue}
                loading={envLoading}
                onKeyChange={setEnvKey}
                onValueChange={setEnvValue}
                onSubmit={setEnvironmentVariable}
                onRemove={(name) => void removeEnvironmentVariable(name)}
              />
            )}
          </div>
        </section>
      </div>

      {deleteOpen && (
        <ConfirmDelete
          name={deployment.name}
          onClose={() => setDeleteOpen(false)}
          onConfirm={() => {
            setDeleteOpen(false);
            void onDelete(deployment.name);
          }}
        />
      )}
    </div>
  );
}

function ActionButton({
  icon: Icon,
  label,
  primary = false,
  danger = false,
  onClick,
}: {
  icon: typeof Play;
  label: string;
  primary?: boolean;
  danger?: boolean;
  onClick: () => void;
}) {
  const color = primary
    ? 'bg-amber-300 text-stone-950 hover:bg-amber-200'
    : danger
      ? 'border border-rose-900/70 text-rose-400 hover:bg-rose-500/10'
      : 'border border-white/[0.1] text-stone-300 hover:border-white/[0.2] hover:text-white';
  return (
    <button onClick={onClick} className={`flex h-9 items-center gap-2 px-3 text-xs font-medium transition-colors ${color}`}>
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

function MetadataRow({ icon: Icon, label, value, mono = false }: { icon: typeof Clock3; label: string; value: string; mono?: boolean }) {
  return (
    <div className="py-3.5">
      <dt className="mb-1.5 flex items-center gap-2 text-[10px] text-stone-600">
        <Icon className="h-3 w-3" /> {label}
      </dt>
      <dd className={`break-all text-xs leading-5 text-stone-300 ${mono ? 'font-mono text-[10px]' : ''}`} title={value}>{value}</dd>
    </div>
  );
}

function ServicesTable({ containers, missing }: { containers: DeploymentDetail['status']['containers']; missing: string[] }) {
  const rows = containers ?? [];
  if (!rows.length && !missing.length) return <EmptyTab text="No Compose services found." />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[620px] text-left">
        <thead className="font-mono text-[9px] uppercase tracking-[0.14em] text-stone-700">
          <tr>
            <th className="pb-3 font-normal">Service</th>
            <th className="pb-3 font-normal">Container</th>
            <th className="pb-3 font-normal">Image</th>
            <th className="pb-3 font-normal">Runtime</th>
            <th className="pb-3 text-right font-normal">State</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/[0.07] border-y border-white/[0.08]">
          {rows.map((container) => (
            <tr key={container.name} className="text-xs">
              <td className="py-4 font-medium text-stone-200">{container.service}</td>
              <td className="max-w-44 truncate py-4 font-mono text-[10px] text-stone-500">{container.name}</td>
              <td className="max-w-44 truncate py-4 font-mono text-[10px] text-stone-500">{container.image || 'Compose default'}</td>
              <td className="py-4 font-mono text-[10px] text-stone-400">{container.status.replace(/^Up\s*/i, '')}</td>
              <td className="py-4 text-right"><StatusBadge status={container.state} /></td>
            </tr>
          ))}
          {missing.map((service) => (
            <tr key={service} className="text-xs">
              <td className="py-4 font-medium text-stone-400">{service}</td>
              <td className="py-4 font-mono text-[10px] text-stone-700">Not created</td>
              <td className="py-4 text-stone-700">-</td>
              <td className="py-4 text-stone-700">-</td>
              <td className="py-4 text-right"><StatusBadge status="stopped" /></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function HistoryTable({ jobs, onViewJobLogs }: { jobs: Job[]; onViewJobLogs: (jobId: string) => void }) {
  if (!jobs.length) return <EmptyTab text="No jobs have run for this deployment." />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[600px] text-left">
        <thead className="font-mono text-[9px] uppercase tracking-[0.14em] text-stone-700">
          <tr>
            <th className="pb-3 font-normal">Operation</th>
            <th className="pb-3 font-normal">Status</th>
            <th className="pb-3 font-normal">Started</th>
            <th className="pb-3 font-normal">Duration</th>
            <th className="pb-3 text-right font-normal">Output</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-white/[0.07] border-y border-white/[0.08]">
          {jobs.map((job) => (
            <tr key={job.id} className="text-xs">
              <td className="py-4 capitalize text-stone-200">{job.type.replace('.', ' ')}</td>
              <td className="py-4"><StatusBadge status={job.status} /></td>
              <td className="py-4 font-mono text-[10px] text-stone-500" title={new Date(job.created_at).toLocaleString()}>{formatRelativeTime(job.created_at)}</td>
              <td className="py-4 font-mono text-[10px] text-stone-500">{formatDuration(job.duration_ms)}</td>
              <td className="py-4 text-right">
                <button onClick={() => onViewJobLogs(job.id)} className="inline-flex items-center gap-1.5 text-[10px] text-stone-500 transition-colors hover:text-amber-300">
                  <Terminal className="h-3 w-3" /> View log
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

interface EnvironmentEditorProps {
  names: string[];
  envKey: string;
  envValue: string;
  loading: boolean;
  onKeyChange: (value: string) => void;
  onValueChange: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
  onRemove: (name: string) => void;
}

function EnvironmentEditor({ names, envKey, envValue, loading, onKeyChange, onValueChange, onSubmit, onRemove }: EnvironmentEditorProps) {
  return (
    <div>
      <form onSubmit={onSubmit} className="grid gap-2 border-b border-white/[0.08] pb-5 sm:grid-cols-[minmax(150px,0.7fr)_minmax(180px,1fr)_auto]">
        <input
          value={envKey}
          onChange={(event) => onKeyChange(event.target.value.toUpperCase())}
          placeholder="VARIABLE_NAME"
          className="h-9 border border-white/[0.1] bg-white/[0.025] px-3 font-mono text-[11px] text-stone-200 outline-none placeholder:text-stone-700 focus:border-amber-300/60"
        />
        <input
          type="password"
          value={envValue}
          onChange={(event) => onValueChange(event.target.value)}
          placeholder="Value"
          className="h-9 border border-white/[0.1] bg-white/[0.025] px-3 font-mono text-[11px] text-stone-200 outline-none placeholder:text-stone-700 focus:border-amber-300/60"
        />
        <button disabled={loading || !envKey.trim()} className="flex h-9 items-center justify-center gap-2 bg-amber-300 px-4 text-xs font-medium text-stone-950 disabled:opacity-40">
          {loading ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Plus className="h-3.5 w-3.5" />} Add
        </button>
      </form>

      {names.length ? (
        <ul className="divide-y divide-white/[0.07]">
          {names.map((name) => (
            <li key={name} className="flex items-center justify-between py-3.5">
              <div className="flex items-center gap-3">
                <KeyRound className="h-3.5 w-3.5 text-stone-700" />
                <span className="font-mono text-[11px] text-stone-300">{name}</span>
              </div>
              <button disabled={loading} onClick={() => onRemove(name)} className="text-stone-700 transition-colors hover:text-rose-400 disabled:opacity-40" aria-label={`Remove ${name}`}>
                <X className="h-3.5 w-3.5" />
              </button>
            </li>
          ))}
        </ul>
      ) : <EmptyTab text="No variables in the selected environment file." />}
    </div>
  );
}

function EmptyTab({ text }: { text: string }) {
  return <p className="grid min-h-48 place-items-center border-y border-white/[0.07] text-xs text-stone-700">{text}</p>;
}

function DetailLoading() {
  return (
    <div className="animate-pulse">
      <div className="h-3 w-24 bg-white/[0.05]" />
      <div className="mt-8 h-10 w-72 bg-white/[0.05]" />
      <div className="mt-10 h-px bg-white/[0.08]" />
      <div className="mt-8 grid gap-8 lg:grid-cols-[270px_1fr]">
        <div className="h-80 bg-white/[0.025]" />
        <div className="h-80 bg-white/[0.025]" />
      </div>
    </div>
  );
}

function ConfirmDelete({ name, onClose, onConfirm }: { name: string; onClose: () => void; onConfirm: () => void }) {
  useModalDismiss(true, onClose);

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-black/75 p-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-md border border-white/[0.1] bg-[#151614] p-6 shadow-2xl shadow-black/70" role="dialog" aria-modal="true" aria-labelledby="delete-deployment-title">
        <div className="flex items-start justify-between gap-5">
          <div>
            <p id="delete-deployment-title" className="text-sm font-medium text-white">Delete {name}?</p>
            <p className="mt-2 text-xs leading-5 text-stone-500">This removes the cloned repository and its deployctl record. The operation cannot be undone.</p>
          </div>
          <button onClick={onClose} className="text-stone-600 hover:text-white"><X className="h-4 w-4" /></button>
        </div>
        <div className="mt-6 flex justify-end gap-2 border-t border-white/[0.08] pt-4">
          <button onClick={onClose} className="h-9 border border-white/[0.1] px-4 text-xs text-stone-400 hover:text-white">Cancel</button>
          <button onClick={onConfirm} className="h-9 bg-rose-500 px-4 text-xs font-medium text-white hover:bg-rose-400">Delete deployment</button>
        </div>
      </div>
    </div>
  );
}
