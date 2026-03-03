import { create } from "zustand";
import api from "../services/api.js";

export const useProjectStore = create((set, get) => ({
  projects: [],
  currentProject: null,
  loading: false,
  error: null,

  fetchProjects: async () => {
    set({ loading: true, error: null });
    try {
      const data = await api.listProjects();
      set({ projects: data.projects || [], loading: false });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  fetchProject: async (id) => {
    set({ loading: true, error: null });
    try {
      const project = await api.getProject(id);
      set({ currentProject: project, loading: false });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  createProject: async (data) => {
    set({ loading: true, error: null });
    try {
      const project = await api.createProject(data);
      set((state) => ({
        projects: [project, ...state.projects],
        loading: false,
      }));
      return project;
    } catch (error) {
      set({ error: error.message, loading: false });
      throw error;
    }
  },

  updateProject: async (id, data) => {
    try {
      const updated = await api.updateProject(id, data);
      set((state) => ({
        projects: state.projects.map((p) => (p.id === id ? updated : p)),
        currentProject:
          state.currentProject?.id === id ? updated : state.currentProject,
      }));
      return updated;
    } catch (error) {
      set({ error: error.message });
      throw error;
    }
  },

  deleteProject: async (id) => {
    try {
      await api.deleteProject(id);
      set((state) => ({
        projects: state.projects.filter((p) => p.id !== id),
        currentProject:
          state.currentProject?.id === id ? null : state.currentProject,
      }));
    } catch (error) {
      set({ error: error.message });
      throw error;
    }
  },

  updateBlueprint: async (projectId, blueprint) => {
    try {
      await api.updateBlueprint(projectId, blueprint);
      set((state) => ({
        currentProject:
          state.currentProject?.id === projectId
            ? { ...state.currentProject, blueprint }
            : state.currentProject,
      }));
    } catch (error) {
      set({ error: error.message });
      throw error;
    }
  },

  clearError: () => set({ error: null }),
}));

export const useExploreStore = create((set) => ({
  projects: [],
  loading: false,
  error: null,
  filters: { language: "", search: "", sort: "newest" },

  fetchExplore: async (filters = {}) => {
    set({ loading: true, error: null });
    try {
      const data = await api.explore(filters);
      set({ projects: data.projects || [], loading: false, filters });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  setFilters: (filters) =>
    set((state) => ({ filters: { ...state.filters, ...filters } })),
}));

export const useAnalysisStore = create((set) => ({
  result: null,
  loading: false,
  error: null,
  step: 0, // 0: idle, 1: cloning, 2: analyzing, 3: generating, 4: done

  analyzeGitHub: async (repoUrl, branch, userToken) => {
    set({ loading: true, error: null, step: 1 });
    try {
      // Simulate step progression for UX
      setTimeout(() => set({ step: 2 }), 1000);
      setTimeout(() => set({ step: 3 }), 2500);

      const result = await api.analyzeGitHub(repoUrl, branch, userToken);
      set({ result, loading: false, step: 4 });
      return result;
    } catch (error) {
      const isBackendDown =
        error.message.includes("Failed to fetch") ||
        error.message.includes("NetworkError") ||
        error.message.includes("502") ||
        error.message.includes("503");

      const msg = isBackendDown
        ? 'Octo backend is not running. Start it with "octo serve" in your terminal, then try again.'
        : error.message;

      set({ error: msg, loading: false, step: 0 });
      throw error;
    }
  },

  reset: () => set({ result: null, loading: false, error: null, step: 0 }),
}));

export const useEnvVarStore = create((set) => ({
  envVars: [],
  loading: false,

  fetchEnvVars: async (projectId) => {
    set({ loading: true });
    try {
      const data = await api.getProjectEnvVars(projectId);
      set({ envVars: data || [], loading: false });
    } catch {
      set({ loading: false });
    }
  },

  saveEnvVars: async (projectId, envVars) => {
    try {
      await api.saveProjectEnvVars(projectId, envVars);
      set({ envVars });
    } catch (error) {
      throw error;
    }
  },
}));
