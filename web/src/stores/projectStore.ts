import { create } from 'zustand';
import { projectApi } from '../api/project';
import { getApiErrorMessage } from '../api/client';
import { getSessionGeneration, onSessionReset } from '../utils/session';
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
    const generation = getSessionGeneration();
    set({ isLoading: true, error: null });
    try {
      const projects = await projectApi.getProjects();
      if (generation !== getSessionGeneration()) return;
      set({ projects, isLoading: false });
    } catch (error) {
      if (generation !== getSessionGeneration()) return;
      set({
        error: getApiErrorMessage(error, '获取项目列表失败'),
        isLoading: false,
      });
    }
  },

  createProject: async (data) => {
    const generation = getSessionGeneration();
    set({ isLoading: true, error: null });
    try {
      const project = await projectApi.createProject(data);
      if (generation !== getSessionGeneration()) return;
      set((state) => ({
        projects: [...state.projects, project],
        isLoading: false,
      }));
    } catch (error) {
      if (generation !== getSessionGeneration()) throw error;
      set({
        error: getApiErrorMessage(error, '创建项目失败'),
        isLoading: false,
      });
      throw error;
    }
  },

  deleteProject: async (id) => {
    const generation = getSessionGeneration();
    set({ isLoading: true, error: null });
    try {
      await projectApi.deleteProject(id);
      if (generation !== getSessionGeneration()) return;
      set((state) => ({
        projects: state.projects.filter((p) => p.id !== id),
        isLoading: false,
      }));
    } catch (error) {
      if (generation !== getSessionGeneration()) throw error;
      set({
        error: getApiErrorMessage(error, '删除项目失败'),
        isLoading: false,
      });
      throw error;
    }
  },

  setCurrentProject: (project) => {
    set({ currentProject: project });
  },
}));

onSessionReset(() => useProjectStore.setState({ projects: [], currentProject: null, isLoading: false, error: null }));
