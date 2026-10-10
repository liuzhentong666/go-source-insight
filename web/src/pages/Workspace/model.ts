import type { Job, StaticReport } from '../../types/index.ts';

export const statusLabels: Record<string, { text: string; color: string }> = {
  pending: { text: '等待中', color: 'default' },
  queued: { text: '排队中', color: 'default' },
  running: { text: '执行中', color: 'processing' },
  canceling: { text: '正在取消，等待服务端确认', color: 'warning' },
  canceled: { text: '已取消', color: 'default' },
  completed: { text: '已完成', color: 'success' },
  failed: { text: '失败', color: 'error' },
};

export const kindLabels: Record<string, string> = { analyze: '静态分析', index: '建立索引', legacy_analyze: '代码片段分析' };

export function jobLabel(job: Job): string {
  return job.recovering ? '恢复状态中' : statusLabels[job.status]?.text || job.status;
}

export function latestJob(jobs: Job[], kind: string): Job | undefined {
  return jobs.filter((job) => job.kind === kind).reduce<Job | undefined>((latest, job) => {
    // Accepted and restored IDs have no server timestamp until the first query.
    // In that interval preserve browser submission order instead of hiding the new task.
    if (!latest || !latest.created_at || !job.created_at) return job;
    return job.created_at > latest.created_at ? job : latest;
  }, undefined);
}

export function readStaticReport(job: Job | undefined): StaticReport | null {
  if (job?.recovering || job?.status !== 'completed' || !job.result || typeof job.result !== 'object' || !('files' in job.result)) return null;
  return job.result as StaticReport;
}

export function relativePathProblem(value: string, allowRoot = true): string | null {
  const path = value.trim();
  if (!path || (allowRoot && path === '.')) return null;
  if (path.startsWith('/') || /[\\:]/.test(path) || path.split('/').some((part) => !part || part === '..' || part === '.')) {
    return '使用项目内相对路径和 / 分隔符，例如 pkg 或 pkg/main.go；不能使用绝对路径或 ..。';
  }
  return null;
}

export function failureHint(code?: string): string | null {
  const hints: Record<string, string> = {
    interrupted: '服务中断使任务终止。任务没有自动重放；索引可能已发布，请先通过问答检查再决定是否重新提交。',
    index_publication_unknown: '索引发布结果无法确认，快照可能已经生效。请先通过问答检查当前索引，确认后再手动重建。',
    timeout: '任务超过服务端时限。请检查范围和服务状态，再决定是否手动提交。',
    ai_unavailable: 'Ollama 或向量存储暂不可用。扫描和静态分析仍可使用。',
    queue_full: '任务队列已满，本次没有创建任务。稍后可手动提交。',
    project_busy: '此项目正在索引或问答，请等待当前操作释放资源。',
    source_unavailable: '源码尚未就绪或未能完整读取，请检查当前本地源码目录并刷新目录信息。',
    invalid_source: '源码分析失败，请检查 Go 语法。',
  };
  return code ? hints[code] || null : null;
}

export function formatTime(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN');
}
