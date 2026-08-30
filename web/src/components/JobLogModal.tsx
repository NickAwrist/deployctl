import { useEffect, useRef, useState } from 'react';
import { Check, Copy, LoaderCircle, Square, Terminal, X } from 'lucide-react';
import { api } from '../lib/api';
import { Job, JobLog } from '../lib/types';
import { formatDuration } from '../lib/format';
import { copyText } from '../lib/clipboard';
import { useModalDismiss } from '../lib/useModalDismiss';
import { StatusBadge } from './StatusBadge';

interface JobLogModalProps {
  jobId: string | null;
  onClose: () => void;
  onJobFinished?: () => void;
}

function isJobStatus(value: string): value is Job['status'] {
  return ['queued', 'running', 'succeeded', 'failed', 'cancelled'].includes(value);
}

export function JobLogModal({ jobId, onClose, onJobFinished }: JobLogModalProps) {
  const [logs, setLogs] = useState<JobLog[]>([]);
  const [job, setJob] = useState<Job | null>(null);
  const [copied, setCopied] = useState(false);
  const [streaming, setStreaming] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);

  useModalDismiss(Boolean(jobId), onClose);

  useEffect(() => {
    if (!jobId) {
      setLogs([]);
      setJob(null);
      return;
    }

    setLogs([]);
    setStreaming(true);
    void api.getJob(jobId).then(setJob).catch(() => setStreaming(false));

    const unsubscribe = api.streamJobLogs(jobId, {
      onLog: (log) => {
        setLogs((current) => current.some((item) => item.sequence === log.sequence)
          ? current
          : [...current, log].sort((left, right) => left.sequence - right.sequence));
      },
      onStatus: setJob,
      onDone: (status) => {
        setStreaming(false);
        setJob((current) => current && isJobStatus(status) ? { ...current, status } : current);
        onJobFinished?.();
      },
      onError: () => setStreaming(false),
    });

    return unsubscribe;
  }, [jobId, onJobFinished]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  if (!jobId) return null;

  const copyLogs = async () => {
    const copied = await copyText(logs.map((log) => log.message).join('\n'));
    if (!copied) return;
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1_500);
  };

  return (
    <div
      className="modal-enter fixed inset-0 z-50 grid place-items-center bg-black/80 p-3 backdrop-blur-sm sm:p-6"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="flex h-[82vh] w-full max-w-5xl flex-col border border-white/[0.1] bg-[#111210] shadow-2xl shadow-black/70" role="dialog" aria-modal="true" aria-label="Job output">
        <header className="flex items-center justify-between gap-4 border-b border-white/[0.08] px-4 py-3.5 sm:px-5">
          <div className="flex min-w-0 items-center gap-3">
            <Terminal className="h-4 w-4 shrink-0 text-amber-300" />
            <div className="min-w-0">
              <div className="flex items-center gap-2.5">
                <span className="truncate text-xs font-medium capitalize text-white">{job?.deployment_name || 'Job output'}</span>
                {job && <StatusBadge status={job.status} />}
              </div>
              <p className="mt-0.5 truncate font-mono text-[9px] text-stone-700">{job?.type ?? 'loading'} / {jobId}</p>
            </div>
          </div>

          <div className="flex items-center gap-1">
            {streaming && (
              <button onClick={() => void api.cancelJob(jobId)} className="flex h-8 items-center gap-1.5 px-2.5 text-[10px] text-rose-400 hover:bg-rose-500/10">
                <Square className="h-3 w-3" /> Cancel
              </button>
            )}
            <button disabled={!logs.length} onClick={() => void copyLogs()} className="grid h-8 w-8 place-items-center text-stone-600 hover:bg-white/[0.05] hover:text-white disabled:opacity-30" title="Copy output">
              {copied ? <Check className="h-3.5 w-3.5 text-stone-300" /> : <Copy className="h-3.5 w-3.5" />}
            </button>
            <button onClick={onClose} className="grid h-8 w-8 place-items-center text-stone-600 hover:bg-white/[0.05] hover:text-white" aria-label="Close logs">
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="flex-1 overflow-auto bg-[#090a09] p-4 font-mono text-[11px] leading-5 sm:p-5">
          {!logs.length ? (
            <div className="grid h-full place-items-center text-stone-700">
              <div className="text-center">
                {streaming && <LoaderCircle className="mx-auto mb-3 h-5 w-5 animate-spin text-amber-300" />}
                <p>{streaming ? 'Waiting for output' : 'This job did not write any output.'}</p>
              </div>
            </div>
          ) : (
            <div>
              {logs.map((log) => (
                <div key={log.sequence} className="whitespace-pre-wrap break-words py-0.5 text-stone-400">{log.message}</div>
              ))}
              <div ref={endRef} />
            </div>
          )}
        </div>

        <footer className="flex items-center justify-between border-t border-white/[0.08] px-4 py-2.5 font-mono text-[9px] text-stone-700 sm:px-5">
          <span>{logs.length} lines</span>
          <span>{streaming ? 'streaming' : job?.duration_ms ? formatDuration(job.duration_ms) : 'complete'}</span>
        </footer>
      </div>
    </div>
  );
}
