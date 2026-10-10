import { useMemo, useSyncExternalStore } from 'react';
import { useAuthStore } from '../stores/authStore';
import { jobsApi } from '../api/jobs';
import { getApiErrorMessage } from '../api/client';
import { getSessionGeneration, onSessionReset, storage } from '../utils/session';
import { ProjectJobsController } from '../utils/projectJobs';
import type { JobAccepted, JobKind } from '../types';
export { isJobActive } from '../utils/projectJobs';

const controllers = new Map<string, ProjectJobsController>();
onSessionReset(() => { controllers.forEach(controller => controller.dispose()); controllers.clear(); });
const empty = { jobs: [], loading: false, error: null };
const subscribeEmpty = () => () => {};
const snapshotEmpty = () => empty;

export function useProjectJobs(projectId: number | null) {
  const ownerId = useAuthStore(state => state.user?.id);
  const generation = getSessionGeneration();
  const controller = useMemo(() => {
    if (!ownerId || !projectId) return null;
    const key = `${generation}:${ownerId}:${projectId}`;
    let value = controllers.get(key);
    if (!value) {
      value = new ProjectJobsController(ownerId, projectId, jobsApi, storage(), getApiErrorMessage);
      controllers.set(key, value);
    }
    return value;
  }, [ownerId, projectId, generation]);
  const snapshot = useSyncExternalStore(controller?.subscribe || subscribeEmpty, controller?.getSnapshot || snapshotEmpty);
  return {
    ...snapshot,
    trackJob: (accepted: JobAccepted, kind: JobKind, target?: string, scopeBinding?: string) => {
      if (generation === getSessionGeneration()) controller?.track(accepted, kind, target, scopeBinding);
    },
    refresh: () => controller?.refresh(),
    cancel: (jobId: string) => controller?.cancel(jobId) || Promise.resolve(),
  };
}
