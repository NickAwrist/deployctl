import { FormEvent, useEffect, useState } from 'react';
import { GitBranch, LoaderCircle, Plus, X } from 'lucide-react';
import { api } from '../lib/api';
import { CreateDeploymentInput } from '../lib/types';
import { useModalDismiss } from '../lib/useModalDismiss';

interface CreateDeploymentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (jobId: string) => void;
}

const emptyForm: CreateDeploymentInput = {
  repo_url: '',
  name: '',
  compose_file: '',
  env_file: '',
};

export function CreateDeploymentModal({ isOpen, onClose, onSuccess }: CreateDeploymentModalProps) {
  const [form, setForm] = useState<CreateDeploymentInput>(emptyForm);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useModalDismiss(isOpen, onClose);

  useEffect(() => {
    if (!isOpen) {
      setForm(emptyForm);
      setError(null);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!form.repo_url.trim()) {
      setError('Repository URL is required.');
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const response = await api.createDeployment(form);
      onClose();
      onSuccess(response.job_id);
    } catch (requestError: unknown) {
      setError(requestError instanceof Error ? requestError.message : 'Could not create the deployment.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="modal-enter fixed inset-0 z-50 grid place-items-center bg-black/75 p-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-lg border border-white/[0.1] bg-[#151614] shadow-2xl shadow-black/70" role="dialog" aria-modal="true" aria-labelledby="create-deployment-title">
        <div className="flex items-start justify-between border-b border-white/[0.08] px-6 py-5">
          <div>
            <p className="font-mono text-[9px] uppercase tracking-[0.18em] text-amber-300">New deployment</p>
            <h2 id="create-deployment-title" className="mt-1.5 text-xl font-semibold tracking-[-0.03em] text-white">Connect a repository</h2>
            <p className="mt-1 text-xs text-stone-600">deployctl will clone it and resolve the Compose configuration.</p>
          </div>
          <button onClick={onClose} className="text-stone-600 transition-colors hover:text-white" aria-label="Close">
            <X className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={submit} className="space-y-5 p-6">
          {error && <p className="border-l-2 border-rose-500 pl-3 text-xs leading-5 text-rose-300">{error}</p>}

          <Field label="Git repository" required>
            <div className="relative">
              <GitBranch className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-stone-700" />
              <input
                autoFocus
                value={form.repo_url}
                onChange={(event) => setForm((current) => ({ ...current, repo_url: event.target.value }))}
                placeholder="git@github.com:owner/project.git"
                className="field-input pl-9"
              />
            </div>
          </Field>

          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Name" hint="optional">
              <input
                value={form.name}
                onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                placeholder="project-name"
                className="field-input"
              />
            </Field>
            <Field label="Compose file" hint="auto-detected">
              <input
                value={form.compose_file}
                onChange={(event) => setForm((current) => ({ ...current, compose_file: event.target.value }))}
                placeholder="compose.yaml"
                className="field-input"
              />
            </Field>
          </div>

          <Field label="Environment file" hint="optional local path">
            <input
              value={form.env_file}
              onChange={(event) => setForm((current) => ({ ...current, env_file: event.target.value }))}
              placeholder="/path/to/.env"
              className="field-input"
            />
          </Field>

          <div className="flex justify-end gap-2 border-t border-white/[0.08] pt-5">
            <button type="button" onClick={onClose} className="h-9 border border-white/[0.1] px-4 text-xs text-stone-400 hover:text-white">Cancel</button>
            <button disabled={loading} className="flex h-9 items-center gap-2 bg-amber-300 px-4 text-xs font-semibold text-stone-950 hover:bg-amber-200 disabled:opacity-50">
              {loading ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Plus className="h-3.5 w-3.5" />}
              Create deployment
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function Field({ label, hint, required = false, children }: { label: string; hint?: string; required?: boolean; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-2 flex items-center justify-between text-xs text-stone-400">
        <span>{label}{required && <span className="ml-1 text-amber-300">*</span>}</span>
        {hint && <span className="font-mono text-[9px] text-stone-700">{hint}</span>}
      </span>
      {children}
    </label>
  );
}
