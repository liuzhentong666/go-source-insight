import { apiClient } from './client';
import type { LoginRequest, RegisterRequest, LoginResponse, User } from '../types';

export const authApi = {
  // 登录
  login: async (data: LoginRequest): Promise<LoginResponse> => {
    const response = await apiClient.post('/api/v1/users/login', data);
    return response.data;
  },

  // 注册
  register: async (data: RegisterRequest): Promise<LoginResponse> => {
    const response = await apiClient.post('/api/v1/users/register', data);
    return response.data;
  },

  // 获取用户信息
  getProfile: async (): Promise<User> => {
    const response = await apiClient.get('/api/v1/users/profile');
    return response.data;
  },
};
