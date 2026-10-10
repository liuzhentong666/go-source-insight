import { useMemo, useSyncExternalStore } from 'react';
import { CanceledError } from 'axios';
import { workspaceApi } from '../api/workspace';
import { getApiErrorMessage } from '../api/client';
import { useAuthStore } from '../stores/authStore';
import { getSessionGeneration } from '../utils/session';
import { SourceBindingController } from '../utils/sourceBinding';

export function useProjectSource(projectId: number) {
  const ownerId = useAuthStore(state => state.user?.id);
  const generation = getSessionGeneration();
  const controller = useMemo(() => new SourceBindingController(projectId, (id, signal) => {
    if (!ownerId || generation !== getSessionGeneration()) return Promise.reject(new CanceledError('Session changed'));
    return workspaceApi.getSource(id, signal);
  }, error => getApiErrorMessage(error, '无法读取本地源码目录，请刷新重试')), [projectId, ownerId, generation]);
  const snapshot = useSyncExternalStore(controller.subscribe, controller.getSnapshot);
  return { ...snapshot, refreshSource: controller.refresh };
}
