import { useCallback, useEffect, useState } from 'react';
import { AlertCircle } from 'lucide-react';
import { api } from './lib/api';
import { DeploymentListItem, Job, SystemHealth } from './lib/types';
import { Header } from './components/Header';
import { Dashboard } from './components/Dashboard';
import { DeploymentDetailView } from './components/DeploymentDetail';
import { CreateDeploymentModal } from './components/CreateDeploymentModal';
import { JobLogModal } from './components/JobLogModal';

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'The daemon did not return a response.';
}

export function App() {
  const [deployments, setDeployments] = useState<DeploymentListItem[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [health, setHealth] = useState<SystemHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [selectedDeployment, setSelectedDeployment] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const fetchData = useCallback(async (background = false) => {
    if (!background) setRefreshing(true);

    try {
      const [healthData, deploymentsData, jobsData] = await Promise.all([
        api.getHealth(),
        api.listDeployments(),
        api.listJobs(),
      ]);
      setHealth(healthData);
      setDeployments(deploymentsData);
      setJobs(jobsData);
      setLastUpdated(new Date());
      setLoadError(null);
    } catch (error: unknown) {
      setLoadError(errorMessage(error));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    void fetchData();
    const interval = window.setInterval(() => void fetchData(true), 10_000);
    return () => window.clearInterval(interval);
  }, [fetchData]);

  const handleDeploy = async (name: string, build = false) => {
    const response = await api.deployDeployment(name, build);
    setActiveJobId(response.job_id);
  };

  const handleRestart = async (name: string, build = false) => {
    const response = await api.restartDeployment(name, build);
    setActiveJobId(response.job_id);
  };

  const handleStop = async (name: string) => {
    const response = await api.stopDeployment(name);
    setActiveJobId(response.job_id);
  };

  const handleUpdate = async (name: string, build = false) => {
    const response = await api.updateDeployment(name, build);
    setActiveJobId(response.job_id);
  };

  const handleDelete = async (name: string) => {
    const response = await api.deleteDeployment(name);
    setSelectedDeployment(null);
    setActiveJobId(response.job_id);
  };

  const handleJobFinished = useCallback(() => {
    void fetchData(true);
  }, [fetchData]);

  return (
    <div className="min-h-screen bg-[#0d0e0d] text-stone-100 selection:bg-amber-300 selection:text-stone-950">
      <Header
        health={health}
        refreshing={refreshing}
        lastUpdated={lastUpdated}
        onRefresh={() => void fetchData()}
        onNewDeployment={() => setIsCreateOpen(true)}
      />

      <main className="app-enter mx-auto w-full max-w-[1480px] px-4 pb-12 pt-8 sm:px-6 lg:px-10">
        {loadError && (
          <div className="mb-6 flex items-start justify-between gap-4 border border-rose-900/60 bg-rose-950/20 px-4 py-3 text-sm text-rose-200">
            <div className="flex items-start gap-2.5">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-400" />
              <div>
                <p className="font-medium">Could not reach deployctld</p>
                <p className="mt-0.5 text-xs text-rose-300/70">{loadError}</p>
              </div>
            </div>
            <button className="text-xs font-medium text-rose-200 underline underline-offset-4" onClick={() => void fetchData()}>
              Retry
            </button>
          </div>
        )}

        {selectedDeployment ? (
          <DeploymentDetailView
            deploymentName={selectedDeployment}
            onBack={() => setSelectedDeployment(null)}
            onDeploy={handleDeploy}
            onRestart={handleRestart}
            onStop={handleStop}
            onUpdate={handleUpdate}
            onDelete={handleDelete}
            onViewJobLogs={setActiveJobId}
          />
        ) : (
          <Dashboard
            deployments={deployments}
            jobs={jobs}
            loading={loading}
            onSelectDeployment={setSelectedDeployment}
            onDeploy={handleDeploy}
            onRestart={handleRestart}
            onStop={handleStop}
            onUpdate={handleUpdate}
            onViewJobLogs={setActiveJobId}
            onNewDeployment={() => setIsCreateOpen(true)}
          />
        )}
      </main>

      <CreateDeploymentModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        onSuccess={(jobId) => {
          void fetchData();
          setActiveJobId(jobId);
        }}
      />

      <JobLogModal
        jobId={activeJobId}
        onClose={() => setActiveJobId(null)}
        onJobFinished={handleJobFinished}
      />
    </div>
  );
}
