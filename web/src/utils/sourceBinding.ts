import type { ProjectSourceInfo } from '../types';

interface BindingSnapshot { info: ProjectSourceInfo | null; loading: boolean; error: string | null }
export class SourceBindingController {
  private snapshot: BindingSnapshot = { info: null, loading: true, error: null };
  private listeners = new Set<() => void>();
  private request: AbortController | null = null;
  private cycle = 0;
  private readonly projectId: number;
  private readonly fetch: (projectId: number, signal: AbortSignal) => Promise<ProjectSourceInfo>;
  private readonly errorMessage: (error: unknown) => string;
  constructor(projectId: number, fetch: (projectId: number, signal: AbortSignal) => Promise<ProjectSourceInfo>, errorMessage: (error: unknown) => string) {
    this.projectId = projectId; this.fetch = fetch; this.errorMessage = errorMessage;
  }
  getSnapshot = () => this.snapshot;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    if (this.listeners.size === 1) void this.refresh();
    return () => { this.listeners.delete(listener); if (!this.listeners.size) { this.cycle++; this.request?.abort(); this.request = null; } };
  };
  private publish(snapshot: BindingSnapshot) { this.snapshot = snapshot; this.listeners.forEach(listener => listener()); }
  refresh = async () => {
    this.request?.abort();
    const controller = new AbortController();
    this.request = controller;
    const cycle = ++this.cycle;
    this.publish({ info: null, loading: true, error: null });
    try {
      const info = await this.fetch(this.projectId, controller.signal);
      if (controller.signal.aborted || cycle !== this.cycle) return;
      if (info.project_id !== this.projectId) throw new Error('Source binding does not belong to this project');
      this.publish({ info, loading: false, error: null });
    } catch (error) {
      if (!controller.signal.aborted && cycle === this.cycle) this.publish({ info: null, loading: false, error: this.errorMessage(error) });
    } finally { if (this.request === controller) this.request = null; }
  };
}
