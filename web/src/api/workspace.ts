import { apiClient } from './client';
import type { ScanResult, JobAccepted, AskRequest, AnswerResult, ProjectSourceInfo } from '../types';

async function submit(projectId: number, kind: 'analyze' | 'index', target: string, signal?: AbortSignal): Promise<JobAccepted> {
  const response = await apiClient.post(`/projects/${projectId}/${kind}`, { target }, { signal });
  if (response.status !== 202) throw new Error('服务未返回任务接收状态');
  return response.data;
}
export const workspaceApi = {
  getSource: async (projectId: number, signal?: AbortSignal): Promise<ProjectSourceInfo> =>
    (await apiClient.get(`/projects/${projectId}/source`, { signal })).data,
  scan: async (projectId: number, target = '.', signal?: AbortSignal): Promise<ScanResult> =>
    (await apiClient.post(`/projects/${projectId}/scan`, { target }, { signal })).data,
  analyze: (projectId: number, target = '.', signal?: AbortSignal) => submit(projectId, 'analyze', target, signal),
  index: (projectId: number, target = '.', signal?: AbortSignal) => submit(projectId, 'index', target, signal),
  ask: async (projectId: number, data: AskRequest, signal?: AbortSignal): Promise<AnswerResult> =>
    (await apiClient.post(`/projects/${projectId}/ask`, { question: data.question, file: data.file }, { signal, timeout: 350_000 })).data,
};
