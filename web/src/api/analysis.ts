import { apiClient } from './client';
import type { Analysis, AnalysisRequest } from '../types';

export const analysisApi = {
  // 提交代码分析
  analyzeCode: async (data: AnalysisRequest): Promise<{ message: string; analysis_id: number; status: string }> => {
    const response = await apiClient.post('/api/v1/analysis/analyze', data);
    return response.data;
  },

  // 获取分析结果
  getAnalysisResults: async (projectId: number): Promise<Analysis[]> => {
    const response = await apiClient.get(`/api/v1/analysis/${projectId}`);
    return response.data;
  },
};
