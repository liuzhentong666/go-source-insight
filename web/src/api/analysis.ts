import { apiClient } from './client';
import type { Analysis, AnalysisRequest, LegacyJobAccepted } from '../types';

export const analysisApi = {
  // 提交代码分析
  analyzeCode: async (data: AnalysisRequest, signal?: AbortSignal): Promise<LegacyJobAccepted> => {
    const response = await apiClient.post('/analysis/analyze', data, { signal });
    return response.data;
  },

  // 获取分析结果
  getAnalysisResults: async (projectId: number, signal?: AbortSignal): Promise<Analysis[]> => {
    const response = await apiClient.get(`/analysis/${projectId}`, { signal });
    return response.data || [];
  },
};
