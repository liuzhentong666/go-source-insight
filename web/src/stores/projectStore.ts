import { create } from 'zustand';
import { projectApi } from '../api/project';
import type { Project, CreateProjectRequest } from '../types';

interface ProjectState {
  projects: Project[];
  currentProject: Project | null;
  isLoading: boolean;
  error: string | null;
  fetchProjects: () => Promise<void>;
  createProject: (data: CreateProjectRequest) => Promise<void>;
  deleteProject: (id: number) => Promise<void>;
  setCurrentProject: (project: Project | null) => void;
}

export const useProjectStore = create<ProjectState>((set) => ({
  projects: [],
  currentProject: null,
  isLoading: false,
  error: null,

  fetchProjects: async () => {
    set({ isLoading: true, error: null });
    try {
      const projects = await projectApi.getProjects();
      set({ projects, isLoading: false });
    } catch (error: any) {
      set({
        error: error.response?.data?.error || '获取项目列表失败',
        isLoading: false,
      });
    }
  },

  createProject: async (data) => {
    set({ isLoading: true, error: null });
    try {
      const project = await projectApi.createProject(data);
      set((state) => ({
        projects: [...state.projects, project],
        isLoading: false,
      }));
    } catch (error: any) {
      set({
        error: error.response?.data?.error || '创建项目失败',
        isLoading: false,
      });
      throw error;
    }
  },

  deleteProject: async (id) => {
    set({ isLoading: true, error: null });
    try {
      await projectApi.deleteProject(id);
      set((state) => ({
        projects: state.projects.filter((p) => p.id !== id),
        isLoading: false,
      }));
    } catch (error: any) {
      set({
        error: error.response?.data?.error || '删除项目失败',
        isLoading: false,
      });
      throw error;
    }
  },

  setCurrentProject: (project) => {
    set({ currentProject: project });
  },
}));
