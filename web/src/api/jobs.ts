import { apiClient } from './client';
import type { Job } from '../types';

export const jobsApi = {
  getJob: async (jobId: string, signal?: AbortSignal): Promise<Job> =>
    (await apiClient.get(`/jobs/${encodeURIComponent(jobId)}`, { signal })).data,
  cancelJob: async (jobId: string, signal?: AbortSignal): Promise<Job> =>
    (await apiClient.post(`/jobs/${encodeURIComponent(jobId)}/cancel`, {}, { signal })).data,
};
