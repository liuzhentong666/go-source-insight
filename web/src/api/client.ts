import axios, { CanceledError } from 'axios';
import type { InternalAxiosRequestConfig } from 'axios';
import { getSessionGeneration, getSessionToken, resetSession } from '../utils/session';

type SessionRequest = InternalAxiosRequestConfig & { sessionGeneration?: number; sessionToken?: string | null };
export const apiClient = axios.create({
  baseURL: '/api/v1',
  timeout: 60_000,
  headers: { 'Content-Type': 'application/json' },
});
apiClient.interceptors.request.use((config: SessionRequest) => {
  config.sessionGeneration = getSessionGeneration();
  config.sessionToken = getSessionToken();
  if (config.sessionToken) config.headers.Authorization = `Bearer ${config.sessionToken}`;
  return config;
});
apiClient.interceptors.response.use(response => {
  if ((response.config as SessionRequest).sessionGeneration !== getSessionGeneration()) {
    throw new CanceledError('Session changed');
  }
  return response;
}, error => {
  const config = error.config as SessionRequest | undefined;
  if (config && config.sessionGeneration !== getSessionGeneration()) {
    return Promise.reject(new CanceledError('Session changed'));
  }
  if (error.response?.status === 401 && config?.sessionToken) resetSession();
  return Promise.reject(error);
});

export function getApiErrorMessage(error: unknown, fallback = '请求失败，请稍后重试'): string {
  if (axios.isAxiosError(error)) {
    const code = error.response?.data?.code;
    const messages: Record<string, string> = {
      queue_full: '任务队列已满，本次未创建任务，请稍后重试',
      project_busy: '项目正在索引或问答，请等待资源释放后重试',
      ai_unavailable: 'AI 服务暂不可用，请检查 Ollama 和向量库',
      timeout: '操作超时，请稍后查看任务状态或重试问答',
      interrupted: '服务重启中断了任务，请先检查状态后手动处理',
      index_publication_unknown: '索引发布状态未知，请先验证现有索引，勿直接重试',
      source_unavailable: '本地源码目录尚未就绪或无法读取，请检查当前目录并刷新目录信息',
      invalid_target: '路径无效或不在当前本地源码目录内，请检查完整路径',
      unsafe_path: '路径包含链接或越出源码边界，请选择当前本地目录内的真实路径',
      excluded_target: '此路径在项目的排除范围内，不能用于扫描、分析或索引',
    };
    if (typeof code === 'string' && messages[code]) return messages[code];
    if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') return '等待服务响应超时，请检查服务状态';
    const message = error.response?.data?.error;
    if (typeof message === 'string') return message;
    if (!error.response) return '无法连接服务，请检查服务是否启动';
  }
  return fallback;
}
export function isRequestCanceled(error: unknown) { return axios.isCancel(error); }
