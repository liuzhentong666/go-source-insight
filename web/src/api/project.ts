import { apiClient } from './client';
import type { Project, CreateProjectRequest } from '../types';

export const projectApi = {
  // 获取项目列表
  getProjects: async (signal?: AbortSignal): Promise<Project[]> => {
    const response = await apiClient.get('/projects/', { signal });
    return response.data || [];
  },

  // 获取单个项目
  getProject: async (id: number, signal?: AbortSignal): Promise<Project> => {
    const response = await apiClient.get(`/projects/${id}`, { signal });
    return response.data;
  },

  // 创建项目
  createProject: async (data: CreateProjectRequest, signal?: AbortSignal): Promise<Project> => {
    const response = await apiClient.post('/projects/', { name: data.name, description: data.description }, { signal });
    return response.data;
  },

  // 删除项目
  deleteProject: async (id: number, signal?: AbortSignal): Promise<void> => {
    await apiClient.delete(`/projects/${id}`, { signal });
  },
};
