import { apiClient } from './client';
import type { LoginRequest, RegisterRequest, LoginResponse, User } from '../types';

export const authApi = {
  // 登录
  login: async (data: LoginRequest): Promise<LoginResponse> => {
    const response = await apiClient.post('/users/login', { username: data.username, password: data.password });
    return response.data;
  },

  // 注册
  register: async (data: RegisterRequest): Promise<LoginResponse> => {
    const response = await apiClient.post('/users/register', { username: data.username, email: data.email, password: data.password });
    return response.data;
  },

  // 获取用户信息
  getProfile: async (signal?: AbortSignal): Promise<User> => {
    const response = await apiClient.get('/users/profile', { signal });
    return response.data;
  },
};
