import type { Job, JobAccepted, JobKind, JobStatus } from '../types';

export function isJobActive(status: JobStatus) {
  return status === 'queued' || status === 'running' || status === 'canceling';
}
export interface JobsSnapshot { jobs: Job[]; loading: boolean; error: string | null }
interface JobsTransport {
  getJob: (id: string, signal: AbortSignal) => Promise<Job>;
  cancelJob: (id: string, signal: AbortSignal) => Promise<Job>;
}
interface KnownJob { job_id: string; project_id: number; kind: JobKind; target: string; scope_binding?: string }

// Restore IDs learned by this browser session, not a fabricated server history.
export class ProjectJobsController {
  private snapshot: JobsSnapshot = { jobs: [], loading: false, error: null };
  private listeners = new Set<() => void>();
  private failures = new Map<string, number>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private request: AbortController | undefined;
  private cancellations = new Map<string, Promise<void>>();
  private cancelControllers = new Set<AbortController>();
  private cycle = 0;
  private running = false;
  private readonly key: string;
  private readonly projectId: number;
  private readonly transport: JobsTransport;
  private readonly sessionStorage?: Storage;
  private readonly errorMessage: (error: unknown) => string;
  private readonly interval: number;
  constructor(
    ownerId: number,
    projectId: number,
    transport: JobsTransport,
    sessionStorage?: Storage,
    errorMessage: (error: unknown) => string = () => '任务状态读取失败，请手动刷新',
    interval = 1500,
  ) {
    this.projectId = projectId;
    this.transport = transport;
    this.sessionStorage = sessionStorage;
    this.errorMessage = errorMessage;
    this.interval = interval;
    this.key = `gosource.jobs.${ownerId}.${projectId}`;
    try {
      const known: unknown = JSON.parse(sessionStorage?.getItem(this.key) || '[]');
      if (Array.isArray(known)) {
        this.snapshot.jobs = known.filter((entry): entry is KnownJob =>
          !!entry && typeof entry.job_id === 'string' && entry.project_id === projectId &&
          ['analyze', 'index', 'legacy_analyze'].includes(entry.kind) && typeof entry.target === 'string')
          .slice(-100).map(entry => this.placeholder(entry, 'queued', true));
      }
    } catch { /* Ignore corrupt browser state. */ }
  }
  private placeholder(entry: KnownJob, status: JobStatus, recovering = false): Job {
    return { ...entry, status, recovering, result: null, created_at: '', updated_at: '', started_at: null, finished_at: null };
  }
  getSnapshot = () => this.snapshot;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    if (this.listeners.size === 1) { this.running = true; void this.poll(); }
    return () => {
      this.listeners.delete(listener);
      if (!this.listeners.size) this.stop();
    };
  };
  private publish(patch: Partial<JobsSnapshot>) {
    this.snapshot = { ...this.snapshot, ...patch };
    this.listeners.forEach(listener => listener());
  }
  private persist() {
    try {
      const ids = this.snapshot.jobs.map(({ job_id, project_id, kind, target, scope_binding }) => ({ job_id, project_id, kind, target, scope_binding }));
      this.sessionStorage?.setItem(this.key, JSON.stringify(ids));
    } catch { /* Tasks remain available in memory. */ }
  }
  track = (accepted: JobAccepted, kind: JobKind, target = '.', scope_binding?: string) => {
    if (accepted.project_id !== this.projectId || !accepted.job_id) return;
    if (!this.snapshot.jobs.some(job => job.job_id === accepted.job_id)) {
      this.publish({ jobs: [...this.snapshot.jobs, this.placeholder({ ...accepted, kind, target, scope_binding }, accepted.status)] });
      this.persist();
    }
    this.failures.delete(accepted.job_id);
    this.refresh();
  };
  private merge(job: Job) {
    if (job.project_id !== this.projectId) return;
    const existing = this.snapshot.jobs.find(item => item.job_id === job.job_id);
    if (!existing) return;
    // A late poll must not undo a newer cancellation or confirmed publication.
    if (existing.status === 'completed' && job.status !== 'completed') return;
    if (existing.status === 'canceling' && (job.status === 'queued' || job.status === 'running')) return;
    if (existing.updated_at && job.updated_at && Date.parse(existing.updated_at) > Date.parse(job.updated_at)) return;
    if (!existing.recovering && !isJobActive(existing.status) && isJobActive(job.status)) return;
    this.publish({ jobs: this.snapshot.jobs.map(item => item.job_id === job.job_id ? { ...job, scope_binding: existing.scope_binding, recovering: false } : item) });
    this.persist();
  }
  refresh = () => {
    this.failures.clear();
    this.publish({ error: null });
    if (this.timer) clearTimeout(this.timer);
    this.timer = undefined;
    if (this.running && !this.request) void this.poll();
  };
  private async poll() {
    if (!this.running || this.request) return;
    const jobs = this.snapshot.jobs.filter(job => (job.recovering || isJobActive(job.status)) && (this.failures.get(job.job_id) || 0) < 5);
    if (!jobs.length) { this.publish({ loading: false }); return; }
    const controller = new AbortController();
    const cycle = this.cycle;
    this.request = controller;
    this.publish({ loading: true });
    await Promise.all(jobs.map(async job => {
      try {
        const result = await this.transport.getJob(job.job_id, controller.signal);
        if (cycle !== this.cycle || controller.signal.aborted) return;
        this.failures.delete(job.job_id);
        this.merge(result);
      } catch (error) {
        if (cycle !== this.cycle || controller.signal.aborted) return;
        const attempts = (this.failures.get(job.job_id) || 0) + 1;
        this.failures.set(job.job_id, attempts);
        this.publish({ error: `${this.errorMessage(error)}${attempts >= 5 ? '；自动重试已暂停，请手动刷新' : ''}` });
      }
    }));
    if (cycle !== this.cycle || controller.signal.aborted) return;
    this.request = undefined;
    this.publish({ loading: false, ...(this.failures.size === 0 ? { error: null } : {}) });
    if (this.running && this.snapshot.jobs.some(job => (job.recovering || isJobActive(job.status)) && (this.failures.get(job.job_id) || 0) < 5)) {
      const attempts = Math.max(0, ...this.failures.values());
      this.timer = setTimeout(() => { this.timer = undefined; void this.poll(); }, Math.min(30_000, this.interval * 2 ** attempts));
    }
  }
  cancel = (id: string): Promise<void> => {
    const existing = this.cancellations.get(id);
    if (existing) return existing;
    if (!this.snapshot.jobs.some(job => job.job_id === id)) return Promise.resolve();
    const controller = new AbortController();
    const cycle = this.cycle;
    this.cancelControllers.add(controller);
    const pending = this.transport.cancelJob(id, controller.signal).then(job => {
      if (cycle === this.cycle && !controller.signal.aborted) { this.merge(job); this.refresh(); }
    }).catch(error => {
      if (cycle === this.cycle && !controller.signal.aborted) this.publish({ error: this.errorMessage(error) });
      throw error;
    }).finally(() => {
      this.cancelControllers.delete(controller);
      if (this.cancellations.get(id) === pending) this.cancellations.delete(id);
    });
    this.cancellations.set(id, pending);
    return pending;
  };
  stop() {
    this.running = false;
    this.cycle++;
    if (this.timer) clearTimeout(this.timer);
    this.timer = undefined;
    this.request?.abort();
    this.request = undefined;
    this.cancelControllers.forEach(controller => controller.abort());
    this.cancelControllers.clear();
    this.cancellations.clear();
    this.snapshot = { ...this.snapshot, loading: false };
  }
  dispose() { this.stop(); this.listeners.clear(); }
}
