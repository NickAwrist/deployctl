import { useState } from 'react';
import { Boxes, ChevronDown, Plus, RefreshCw, Server } from 'lucide-react';
import { SystemHealth } from '../lib/types';

interface HeaderProps {
  health: SystemHealth | null;
  refreshing: boolean;
  lastUpdated: Date | null;
  onRefresh: () => void;
  onNewDeployment: () => void;
}

export function Header({ health, refreshing, lastUpdated, onRefresh, onNewDeployment }: HeaderProps) {
  const [showHealthDetails, setShowHealthDetails] = useState(false);
  const dockerConnected = Boolean(health?.docker.connected && !health.docker.error);

  return (
    <header className="sticky top-0 z-30 border-b border-white/[0.08] bg-[#0d0e0d]/90 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-[1480px] items-center justify-between px-4 sm:px-6 lg:px-10">
        <div className="flex items-center gap-2.5">
          <div className="grid h-8 w-8 place-items-center bg-amber-300 text-stone-950">
            <Boxes className="h-4 w-4" strokeWidth={2.4} />
          </div>
          <span className="text-sm font-semibold tracking-[-0.02em] text-white">deployctl</span>
        </div>

        <div className="flex items-center gap-2">
          <div className="relative hidden sm:block">
            <button
              onClick={() => setShowHealthDetails((open) => !open)}
              className="flex h-9 items-center gap-2 border border-white/[0.08] bg-white/[0.025] px-3 text-xs text-stone-300 transition-colors hover:border-white/[0.16] hover:bg-white/[0.05]"
              aria-expanded={showHealthDetails}
            >
              <span className={`status-dot ${dockerConnected ? 'status-dot-live' : 'status-dot-down'}`} />
              <span>Docker {dockerConnected ? 'online' : 'offline'}</span>
              <ChevronDown className={`h-3 w-3 text-stone-600 transition-transform ${showHealthDetails ? 'rotate-180' : ''}`} />
            </button>

            {showHealthDetails && health && (
              <div className="absolute right-0 top-11 w-72 border border-white/[0.1] bg-[#151614] p-4 shadow-2xl shadow-black/50">
                <div className="mb-3 flex items-center gap-2 border-b border-white/[0.08] pb-3">
                  <Server className="h-4 w-4 text-amber-300" />
                  <p className="text-xs font-medium text-white">Docker environment</p>
                </div>
                <dl className="space-y-2.5 text-xs">
                  <HealthRow label="Host" value={health.docker.host || 'local'} mono />
                  <HealthRow label="Engine" value={health.docker.server_version || 'Unavailable'} />
                  <HealthRow label="API" value={health.docker.api_version || 'Unavailable'} mono />
                  <HealthRow label="Platform" value={health.docker.os_type || 'Unavailable'} />
                </dl>
                {health.docker.error && <p className="mt-3 border-l-2 border-rose-500 pl-2 text-[11px] leading-5 text-rose-300">{health.docker.error}</p>}
              </div>
            )}
          </div>

          <button
            onClick={onRefresh}
            disabled={refreshing}
            title={lastUpdated ? `Updated ${lastUpdated.toLocaleTimeString()}` : 'Refresh status'}
            className="grid h-9 w-9 place-items-center border border-white/[0.08] text-stone-500 transition-colors hover:border-white/[0.16] hover:text-stone-100 disabled:opacity-50"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`} />
          </button>

          <button
            onClick={onNewDeployment}
            className="flex h-9 items-center gap-2 bg-amber-300 px-3.5 text-xs font-semibold text-stone-950 transition-colors hover:bg-amber-200 active:translate-y-px"
          >
            <Plus className="h-3.5 w-3.5" strokeWidth={2.5} />
            <span className="hidden min-[430px]:inline">New deployment</span>
            <span className="min-[430px]:hidden">New</span>
          </button>
        </div>
      </div>
    </header>
  );
}

function HealthRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="text-stone-500">{label}</dt>
      <dd className={`max-w-44 truncate text-stone-200 ${mono ? 'font-mono text-[11px]' : ''}`} title={value}>
        {value}
      </dd>
    </div>
  );
}
