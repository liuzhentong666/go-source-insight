import type { Job, ProjectSourceInfo, ScanResult } from '../types';

interface AbsolutePath { windows: boolean; path: string; key: string }
function parseAbsolute(value: string): AbsolutePath | null {
  const input = value.trim();
  const windows = /^[a-z]:[\\/]/i.test(input);
  if (!input || (!windows && (!input.startsWith('/') || input.startsWith('//')))) return null;
  if ([...input].some(character => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127)) return null;
  if (!windows && /[\\:]/.test(input)) return null;
  const normalized = windows ? input.replaceAll('\\', '/') : input;
  if (windows && normalized.slice(2).includes(':')) return null;
  const body = normalized.slice(windows ? 3 : 1);
  const components = body.endsWith('/') ? body.slice(0, -1).split('/') : body.split('/');
  if (body && components.some(part => !part || part === '.' || part === '..')) return null;
  if (windows && components.some(part => /[<>"|?*]/.test(part) || /[. ]$/.test(part))) return null;
  if (windows && components.some(part => /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(part))) return null;
  const path = normalized.length > (windows ? 3 : 1) && normalized.endsWith('/') ? normalized.slice(0, -1) : normalized;
  return { windows, path, key: windows ? path.toLowerCase() : path };
}

export function absolutePathProblem(value: string): string | null {
  return parseAbsolute(value) ? null : '请输入完整目录或 Go 文件路径，例如 C:\\code\\project 或 /workspace/project；不能使用 ..、链接设备路径或相对路径。';
}
export function canonicalAbsolutePath(value: string): string | null { return parseAbsolute(value)?.key || null; }

// This is a display/association key. Server confinement remains authoritative.
export function relativeScope(input: string, hostRoot: string): string | null {
  const target = parseAbsolute(input);
  const root = parseAbsolute(hostRoot);
  if (!target || !root || target.windows !== root.windows) return null;
  if (target.key === root.key) return '.';
  const prefix = root.key.endsWith('/') ? root.key : `${root.key}/`;
  return target.key.startsWith(prefix) ? target.path.slice(prefix.length) : null;
}
export function relativeScopesEqual(left: string, right: string, hostRoot: string): boolean {
  const windows = parseAbsolute(hostRoot)?.windows;
  return windows ? left.toLowerCase() === right.toLowerCase() : left === right;
}
export interface ScopedScan { project_id: number; request_target: string; binding_id: string; result: ScanResult }
export function scanMatchesInput(scan: ScopedScan | null, input: string, info: ProjectSourceInfo | null): ScanResult | null {
  if (!scan || !info?.host_root || scan.project_id !== info.project_id || !info.available || !info.binding_id || scan.binding_id !== info.binding_id) return null;
  const expected = relativeScope(input, info.host_root);
  if (expected === null || canonicalAbsolutePath(input) !== canonicalAbsolutePath(scan.request_target)) return null;
  return relativeScopesEqual(expected, scan.result.target_path, info.host_root) ? scan.result : null;
}
export function jobMatchesScope(job: Job, input: string, info: ProjectSourceInfo | null): boolean {
  if (!info?.host_root || job.project_id !== info.project_id || !info.binding_id || job.scope_binding !== info.binding_id) return false;
  const relative = relativeScope(input, info.host_root);
  return relative !== null && relativeScopesEqual(relative, job.target, info.host_root);
}
export function sourceMode(info: ProjectSourceInfo | null): 'local' | 'bound' | 'legacy' {
  return info?.source_mode || (info?.binding_configured ? 'bound' : 'legacy');
}
export const sourceSetupCommands = {
  powershell: "./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\\code\\project'",
  bash: "bash scripts/deploy.sh --mode web --source-directory '/workspace/project'",
};
export function sourceBindingProblem(info: ProjectSourceInfo | null, loading: boolean, error: string | null): string | null {
  if (loading) return '正在读取本地源码目录，请稍候。';
  if (error) return error;
  if (!info) return '尚未读取本地源码目录，请刷新目录信息。';
  if (!info.binding_configured || !info.absolute_input_allowed || !info.host_root) return '尚未选择本地源码目录。启动时添加 -SourceDirectory（PowerShell）或 --source-directory（Bash），再刷新目录信息。';
  if (!info.available) return '当前本地源码目录不可读取或未挂载。请检查目录是否存在、是否可读取，再刷新目录信息。';
  if (!info.binding_id) return '本地源码目录信息不完整，请刷新目录信息或更新本地程序。';
  return null;
}
