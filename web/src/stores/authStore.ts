import { create } from 'zustand';
import { CanceledError } from 'axios';
import { authApi } from '../api/auth';
import { getApiErrorMessage } from '../api/client';
import { getSessionGeneration, onSessionReset, readStoredToken, resetSession, setSessionToken } from '../utils/session';
import type { User, LoginRequest, RegisterRequest, LoginResponse } from '../types';

interface AuthState {
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  initialized: boolean;
  isLoading: boolean;
  error: string | null;
  login: (credentials: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => void;
  fetchProfile: () => Promise<void>;
  initialize: () => Promise<void>;
}
let initialization: Promise<void> | null = null;
let profileController: AbortController | null = null;

export const useAuthStore = create<AuthState>((set, get) => {
  async function authenticate(call: () => Promise<LoginResponse>) {
    resetSession();
    const generation = getSessionGeneration();
    set({ isLoading: true, error: null });
    try {
      const response = await call();
      if (generation !== getSessionGeneration()) throw new CanceledError('Session changed');
      setSessionToken(response.token);
      set({ token: response.token, user: response.user, isAuthenticated: true, initialized: true, isLoading: false });
    } catch (error) {
      if (generation === getSessionGeneration()) set({ error: getApiErrorMessage(error, '认证失败'), isLoading: false });
      throw error;
    }
  }
  async function verifyProfile() {
    const generation = getSessionGeneration();
    profileController?.abort();
    const controller = new AbortController();
    profileController = controller;
    try {
      const user = await authApi.getProfile(controller.signal);
      if (generation === getSessionGeneration()) set({ user, isAuthenticated: true, error: null });
    } catch (error) {
      if (generation === getSessionGeneration()) {
        resetSession();
        set({ error: getApiErrorMessage(error, '会话验证失败，请重新登录') });
      }
    } finally {
      if (profileController === controller) profileController = null;
      if (generation === getSessionGeneration()) set({ initialized: true, isLoading: false });
    }
  }
  return {
    token: null, user: null, isAuthenticated: false, initialized: false, isLoading: false, error: null,
    initialize: () => {
      if (get().initialized) return Promise.resolve();
      if (initialization) return initialization;
      const token = readStoredToken();
      if (!token) { set({ initialized: true }); return Promise.resolve(); }
      setSessionToken(token);
      set({ token, isLoading: true });
      initialization = verifyProfile().finally(() => { initialization = null; });
      return initialization;
    },
    login: credentials => authenticate(() => authApi.login(credentials)),
    register: data => authenticate(() => authApi.register(data)),
    logout: () => { resetSession(); },
    fetchProfile: verifyProfile,
  };
});

onSessionReset(() => {
  profileController?.abort();
  initialization = null;
  useAuthStore.setState({ token: null, user: null, isAuthenticated: false, initialized: true, isLoading: false, error: null });
});
