import { apiClient } from './client';
import type { Project, CreateProjectRequest } from '../types';

export const projectApi = {
  // 获取项目列表
  getProjects: async (): Promise<Project[]> => {
    const response = await apiClient.get('/api/v1/projects/');
    return response.data;
  },

  // 获取单个项目
  getProject: async (id: number): Promise<Project> => {
    const response = await apiClient.get(`/api/v1/projects/${id}`);
    return response.data;
  },

  // 创建项目
  createProject: async (data: CreateProjectRequest): Promise<Project> => {
    const response = await apiClient.post('/api/v1/projects/', data);
    return response.data;
  },

  // 删除项目
  deleteProject: async (id: number): Promise<void> => {
    await apiClient.delete(`/api/v1/projects/${id}`);
  },
};
