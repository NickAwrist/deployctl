import { LoaderCircle } from 'lucide-react';

interface StatusBadgeProps {
  status: string;
  size?: 'sm' | 'md';
}

interface StatusStyle {
  dot: string;
  text: string;
  label: string;
  spinning?: boolean;
}

function statusStyle(status: string): StatusStyle {
  switch (status.toLowerCase()) {
    case 'running':
    case 'succeeded':
    case 'up':
    case 'ok':
      return { dot: 'bg-emerald-400', text: 'text-emerald-300', label: status };
    case 'queued':
    case 'building':
    case 'running_job':
      return { dot: 'bg-amber-300', text: 'text-amber-200', label: status.replace('_job', ''), spinning: true };
    case 'failed':
    case 'dead':
    case 'error':
    case 'unavailable':
      return { dot: 'bg-rose-400', text: 'text-rose-300', label: status };
    case 'degraded':
    case 'partial':
    case 'paused':
    case 'cancelled':
      return { dot: 'bg-amber-300', text: 'text-amber-200', label: status };
    default:
      return { dot: 'bg-stone-600', text: 'text-stone-500', label: status || 'stopped' };
  }
}

export function StatusBadge({ status, size = 'sm' }: StatusBadgeProps) {
  const style = statusStyle(status);
  return (
    <span className={`inline-flex items-center gap-2 capitalize ${style.text} ${size === 'md' ? 'text-sm' : 'text-xs'}`}>
      {style.spinning ? (
        <LoaderCircle className="h-3 w-3 animate-spin" />
      ) : (
        <span className={`h-1.5 w-1.5 rounded-full ${style.dot}`} />
      )}
      {style.label.replace('_', ' ')}
    </span>
  );
}
