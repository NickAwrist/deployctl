import { useEffect, useMemo, useState } from 'react';
import {
  ArrowDownToLine,
  ArrowUpRight,
  Box,
  ChevronRight,
  Clock3,
  History,
  MoreHorizontal,
  Play,
  RotateCw,
  Search,
  Square,
} from 'lucide-react';
import { DeploymentListItem, DeploymentState, Job } from '../lib/types';
import {
  deploymentState,
  deploymentUptime,
  fileName,
  formatDuration,
  formatRelativeTime,
  latestJobByDeployment,
  repositoryLabel,
} from '../lib/format';
import { StatusBadge } from './StatusBadge';

type Filter = 'all' | 'running' | 'attention' | 'stopped';

interface DashboardProps {
  deployments: DeploymentListItem[];
  jobs: Job[];
  loading: boolean;
  onSelectDeployment: (name: string) => void;
  onDeploy: (name: string, build?: boolean) => Promise<void>;
  onRestart: (name: string, build?: boolean) => Promise<void>;
  onStop: (name: string) => Promise<void>;
  onUpdate: (name: string, build?: boolean) => Promise<void>;
  onViewJobLogs: (jobId: string) => void;
  onNewDeployment: () => void;
}

export function Dashboard({
  deployments,
  jobs,
  loading,
  onSelectDeployment,
  onDeploy,
  onRestart,
  onStop,
  onUpdate,
  onViewJobLogs,
  onNewDeployment,
}: DashboardProps) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<Filter>('all');
  const [openActions, setOpenActions] = useState<string | null>(null);
  const latestJobs = useMemo(() => latestJobByDeployment(jobs), [jobs]);

  useEffect(() => {
    if (!openActions) return;

    const closeOnOutsideClick = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && !target.closest('[data-deployment-actions]')) {
        setOpenActions(null);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpenActions(null);
    };

    document.addEventListener('pointerdown', closeOnOutsideClick);
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsideClick);
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [openActions]);

  const filtered = deployments.filter((deployment) => {
    const state = deploymentState(deployment);
    const matchesQuery = [deployment.name, deployment.url, deployment.compose_path]
      .join(' ')
      .toLowerCase()
      .includes(query.trim().toLowerCase());
    const matchesFilter =
      filter === 'all' ||
      filter === state ||
      (filter === 'attention' && state !== 'running' && state !== 'stopped');
    return matchesQuery && matchesFilter;
  });

  const runningDeployments = deployments.filter((deployment) => deployment.state === 'running').length;
  const runningServices = deployments.reduce(
    (total, deployment) =>
      total + (deployment.status.containers?.filter((container) => container.state === 'running').length ?? 0),
    0,
  );
  const recentJobs = jobs.slice(0, 9);
  return (
    <div>
      <section className="flex flex-col justify-between gap-5 border-b border-white/[0.08] pb-7 md:flex-row md:items-end">
        <div>
          <h1 className="text-3xl font-semibold tracking-[-0.045em] text-white sm:text-4xl">Deployments</h1>
          <p className="mt-2 text-sm text-stone-500">Container state, runtime, configuration, and job history.</p>
        </div>

        <div className="grid grid-cols-2 divide-x divide-white/[0.08] border-y border-white/[0.08] md:border-y-0">
          <Metric label="Online" value={`${runningDeployments}/${deployments.length}`} note="deployments" />
          <Metric label="Services" value={String(runningServices)} note="running now" />
        </div>
      </section>

      <div className="mt-8 grid gap-8 xl:grid-cols-[minmax(0,1fr)_340px]">
        <section className="min-w-0">
          <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex items-center gap-1 border-b border-white/[0.08]">
              {(['all', 'running', 'attention', 'stopped'] as const).map((value) => (
                <button
                  key={value}
                  onClick={() => setFilter(value)}
                  className={`relative px-3 py-2.5 text-xs capitalize transition-colors ${
                    filter === value ? 'text-white' : 'text-stone-600 hover:text-stone-300'
                  }`}
                >
                  {value}
                  {filter === value && <span className="absolute inset-x-2 -bottom-px h-px bg-amber-300" />}
                </button>
              ))}
            </div>

            <label className="relative block w-full lg:w-64">
              <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-stone-600" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Filter deployments"
                className="h-9 w-full border border-white/[0.08] bg-white/[0.025] pl-9 pr-3 text-xs text-stone-200 outline-none transition-colors placeholder:text-stone-700 focus:border-amber-300/60"
              />
            </label>
          </div>

          <div className="overflow-visible border-y border-white/[0.1]">
            <div className="hidden grid-cols-[minmax(220px,1.5fr)_110px_105px_minmax(145px,0.9fr)_100px_44px] items-center border-b border-white/[0.08] px-4 py-2.5 font-mono text-[9px] uppercase tracking-[0.16em] text-stone-600 md:grid">
              <span>Deployment</span>
              <span>Status</span>
              <span>Uptime</span>
              <span>Last activity</span>
              <span>Services</span>
              <span />
            </div>

            {loading ? (
              <LoadingRows />
            ) : filtered.length === 0 ? (
              <EmptyState hasDeployments={deployments.length > 0} onNewDeployment={onNewDeployment} />
            ) : (
              filtered.map((deployment, index) => {
                const state = deploymentState(deployment);
                const containers = deployment.status.containers ?? [];
                const lastJob = latestJobs.get(deployment.name);

                return (
                  <div
                    key={deployment.name}
                    className={`deployment-row relative grid gap-4 border-b border-white/[0.065] px-4 py-4 last:border-b-0 md:grid-cols-[minmax(220px,1.5fr)_110px_105px_minmax(145px,0.9fr)_100px_44px] md:items-center md:gap-0 ${
                      openActions === deployment.name ? 'z-30' : 'z-0'
                    }`}
                    style={{ animationDelay: `${index * 45}ms` }}
                  >
                    <div className="min-w-0 pr-4">
                      <button
                        onClick={() => onSelectDeployment(deployment.name)}
                        className="group flex max-w-full items-center gap-2 text-left"
                      >
                        <span className="truncate text-sm font-medium text-stone-100 transition-colors group-hover:text-amber-200">
                          {deployment.name}
                        </span>
                        <ArrowUpRight className="h-3 w-3 shrink-0 text-stone-700 transition-colors group-hover:text-amber-300" />
                      </button>
                      <div className="mt-1 flex min-w-0 items-center gap-2 font-mono text-[10px] text-stone-600">
                        <span className="truncate">{repositoryLabel(deployment.url)}</span>
                        <span className="text-stone-800">/</span>
                        <span className="shrink-0">{fileName(deployment.compose_path)}</span>
                      </div>
                    </div>

                    <div title={deployment.status_error}><StatusBadge status={state} /></div>

                    <div className="flex items-center gap-2 text-xs text-stone-300">
                      <Clock3 className="h-3.5 w-3.5 text-stone-700 md:hidden" />
                      <span className="font-mono text-[11px]">{deploymentUptime(deployment.status.containers)}</span>
                    </div>

                    <button
                      disabled={!lastJob}
                      onClick={() => lastJob && onViewJobLogs(lastJob.id)}
                      className="min-w-0 text-left disabled:cursor-default"
                    >
                      {lastJob ? (
                        <>
                          <span className="block truncate text-xs capitalize text-stone-300 hover:text-white">
                            {lastJob.type.replace('.', ' ')}
                          </span>
                          <span className="mt-0.5 block font-mono text-[10px] text-stone-600">
                            {formatRelativeTime(lastJob.created_at)} · {formatDuration(lastJob.duration_ms)}
                          </span>
                        </>
                      ) : (
                        <span className="text-xs text-stone-700">No history</span>
                      )}
                    </button>

                    <span className="font-mono text-[10px] text-stone-500">
                      {containers.length} / {containers.length + (deployment.status.missing?.length ?? 0)}
                    </span>

                    <div data-deployment-actions className="absolute right-3 top-3 md:relative md:right-auto md:top-auto">
                      <button
                        className="grid h-8 w-8 place-items-center text-stone-600 transition-colors hover:bg-white/[0.05] hover:text-white"
                        onClick={() => setOpenActions((name) => (name === deployment.name ? null : deployment.name))}
                        aria-label={`Actions for ${deployment.name}`}
                        aria-expanded={openActions === deployment.name}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </button>
                      {openActions === deployment.name && (
                        <ActionMenu
                          state={state}
                          onClose={() => setOpenActions(null)}
                          onDeploy={() => onDeploy(deployment.name)}
                          onRestart={() => onRestart(deployment.name)}
                          onUpdate={() => onUpdate(deployment.name)}
                          onStop={() => onStop(deployment.name)}
                          onDetails={() => onSelectDeployment(deployment.name)}
                        />
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {!loading && filtered.length > 0 && (
            <p className="mt-3 font-mono text-[10px] text-stone-700">
              Showing {filtered.length} of {deployments.length} deployments
            </p>
          )}
        </section>

        <aside className="xl:border-l xl:border-white/[0.08] xl:pl-8">
          <div className="mb-4 flex items-center border-b border-white/[0.08] pb-3">
            <div className="flex items-center gap-2">
              <History className="h-3.5 w-3.5 text-amber-300" />
              <h2 className="text-xs font-medium text-stone-200">Recent activity</h2>
            </div>
          </div>

          {recentJobs.length === 0 ? (
            <p className="py-8 text-xs text-stone-600">Job history will appear here.</p>
          ) : (
            <ol className="relative before:absolute before:bottom-3 before:left-[5px] before:top-3 before:w-px before:bg-white/[0.08]">
              {recentJobs.map((job) => (
                <li key={job.id}>
                  <button
                    onClick={() => onViewJobLogs(job.id)}
                    className="group relative grid w-full grid-cols-[12px_1fr_auto] gap-3 py-3 text-left"
                  >
                    <span className={`relative z-10 mt-1 h-[11px] w-[11px] rounded-full border-2 border-[#0d0e0d] ${jobDotClass(job.status)}`} />
                    <span className="min-w-0">
                      <span className="block truncate text-xs text-stone-300 transition-colors group-hover:text-white">
                        <span className="font-medium text-stone-100">{job.deployment_name}</span>{' '}
                        {jobVerb(job.type)}
                      </span>
                      {job.status === 'failed' && job.error ? (
                        <span className="mt-1 block truncate text-[10px] text-rose-400/80">{job.error.split('\n')[0]}</span>
                      ) : (
                        <span className="mt-1 block font-mono text-[10px] text-stone-700">
                          {formatDuration(job.duration_ms)} · {job.status}
                        </span>
                      )}
                    </span>
                    <span className="pt-0.5 font-mono text-[9px] text-stone-700">{formatRelativeTime(job.created_at)}</span>
                  </button>
                </li>
              ))}
            </ol>
          )}
        </aside>
      </div>
    </div>
  );
}

function Metric({ label, value, note }: { label: string; value: string; note: string }) {
  return (
    <div className="min-w-24 px-4 py-3 first:pl-0 sm:min-w-32 sm:px-6 md:py-0 md:first:pl-6">
      <p className="font-mono text-[9px] uppercase tracking-[0.15em] text-stone-600">{label}</p>
      <div className="mt-1 flex items-baseline gap-2">
        <span className="text-xl font-medium tracking-[-0.03em] text-stone-100">{value}</span>
        <span className="hidden text-[10px] text-stone-600 lg:inline">{note}</span>
      </div>
    </div>
  );
}

function LoadingRows() {
  return (
    <div className="divide-y divide-white/[0.06]">
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="grid h-[73px] animate-pulse grid-cols-[1.5fr_110px_105px_1fr_100px_44px] items-center gap-4 px-4">
          <span className="h-3 w-36 bg-white/[0.05]" />
          <span className="h-5 w-16 bg-white/[0.05]" />
          <span className="h-3 w-12 bg-white/[0.05]" />
        </div>
      ))}
    </div>
  );
}

function EmptyState({ hasDeployments, onNewDeployment }: { hasDeployments: boolean; onNewDeployment: () => void }) {
  return (
    <div className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
      <Box className="h-6 w-6 text-stone-700" />
      <p className="mt-4 text-sm font-medium text-stone-300">{hasDeployments ? 'No matching deployments' : 'No deployments yet'}</p>
      <p className="mt-1 max-w-sm text-xs leading-5 text-stone-600">
        {hasDeployments ? 'Try a different search or status filter.' : 'Add a Git repository with a Compose file to start tracking it.'}
      </p>
      {!hasDeployments && (
        <button onClick={onNewDeployment} className="mt-4 text-xs font-medium text-amber-300 hover:text-amber-200">
          Create deployment <ChevronRight className="inline h-3 w-3" />
        </button>
      )}
    </div>
  );
}

interface ActionMenuProps {
  state: DeploymentState;
  onClose: () => void;
  onDeploy: () => Promise<void>;
  onRestart: () => Promise<void>;
  onUpdate: () => Promise<void>;
  onStop: () => Promise<void>;
  onDetails: () => void;
}

function ActionMenu({ state, onClose, onDeploy, onRestart, onUpdate, onStop, onDetails }: ActionMenuProps) {
  const run = (action: () => void | Promise<void>) => {
    onClose();
    void action();
  };
  const hasRunningServices = state === 'running' || state === 'partial';

  return (
    <div className="absolute right-0 z-20 mt-1 w-44 border border-white/[0.1] bg-[#181917] py-1 shadow-2xl shadow-black/60">
      {state !== 'running' && <MenuAction icon={Play} label="Deploy" onClick={() => run(onDeploy)} />}
      {hasRunningServices && <MenuAction icon={RotateCw} label="Restart" onClick={() => run(onRestart)} />}
      <MenuAction icon={ArrowDownToLine} label="Pull update" onClick={() => run(onUpdate)} />
      {hasRunningServices && <MenuAction icon={Square} label="Stop" danger onClick={() => run(onStop)} />}
      <div className="my-1 h-px bg-white/[0.08]" />
      <MenuAction icon={ChevronRight} label="View details" onClick={() => run(onDetails)} />
    </div>
  );
}

function MenuAction({
  icon: Icon,
  label,
  danger = false,
  onClick,
}: {
  icon: typeof Play;
  label: string;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex w-full items-center gap-2.5 px-3 py-2 text-left text-xs transition-colors ${
        danger ? 'text-rose-400 hover:bg-rose-500/10' : 'text-stone-300 hover:bg-white/[0.05] hover:text-white'
      }`}
    >
      <Icon className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

function jobDotClass(status: string): string {
  if (status === 'succeeded') return 'bg-emerald-400';
  if (status === 'failed') return 'bg-rose-400';
  if (status === 'running' || status === 'queued') return 'animate-pulse bg-amber-300';
  return 'bg-stone-600';
}

function jobVerb(type: string): string {
  const verbs: Record<string, string> = {
    create: 'was created',
    delete: 'was deleted',
    deploy: 'was deployed',
    restart: 'was restarted',
    stop: 'was stopped',
    update: 'pulled an update',
    'env.set': 'changed environment',
    'env.import': 'imported environment',
    'env.unset': 'removed environment keys',
  };
  return verbs[type] ?? `ran ${type}`;
}
