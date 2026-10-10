import test from 'node:test';
import assert from 'node:assert/strict';
import { absolutePathProblem, canonicalAbsolutePath, jobMatchesScope, relativeScope, scanMatchesInput, sourceBindingProblem, sourceMode, sourceSetupCommands } from '../src/utils/sourceScope.ts';
import { SourceBindingController } from '../src/utils/sourceBinding.ts';
import { ProjectJobsController } from '../src/utils/projectJobs.ts';
import { workspaceApi } from '../src/api/workspace.ts';
import { apiClient, getApiErrorMessage } from '../src/api/client.ts';
import { AxiosError } from 'axios';
import { resetSession, setSessionToken } from '../src/utils/session.ts';

const binding = (extra = {}) => ({ project_id: 5, binding_configured: true, absolute_input_allowed: true,
  host_root: 'C:\\code\\project', binding_id: 'opaque-binding-one', available: true, excluded_paths: [], ...extra });
const scanned = { target_path: 'pkg', production_file_count: 2, total_files: 3, test_file_count: 1, total_lines: 25,
  files: ['pkg/main.go'], production_files: ['pkg/main.go'], test_files: [], skipped_dirs: [] };
const tick = () => new Promise(resolve => setImmediate(resolve));
const response = (config, data, status = 200) => ({ config, data, status, statusText: 'OK', headers: {} });
function memoryStorage() {
  const entries = new Map();
  return { get length() { return entries.size; }, key: index => [...entries.keys()][index] || null,
    getItem: key => entries.get(key) || null, setItem: (key, value) => entries.set(key, value), removeItem: key => entries.delete(key) };
}

test('full operation paths accept Windows and POSIX directories or Go files', () => {
  for (const path of ['C:\\code\\project', 'c:/code/project/', 'C:\\code\\project\\pkg\\main.go', '/workspace/project', '/workspace/project/main.go', '/', 'C:\\']) {
    assert.equal(absolutePathProblem(path), null, path);
  }
  for (const path of ['', '.', 'pkg/main.go', 'C:code', '\\code\\project', '\\\\server\\share', '\\\\?\\C:\\code',
    'C:/code/../private', 'C:/code/./main.go', 'C:/code//main.go', 'C:/code/main.go:stream', 'C:/code/NUL.go',
    '//workspace/project', '/workspace//project', '/workspace/../private', '/workspace/./project']) assert.ok(absolutePathProblem(path), path);
});

test('canonical paths tolerate Windows case and separators while POSIX remains case-sensitive', () => {
  assert.equal(canonicalAbsolutePath(' C:\\CODE\\Project\\pkg\\ '), canonicalAbsolutePath('c:/code/project/PKG'));
  assert.notEqual(canonicalAbsolutePath('/workspace/Project'), canonicalAbsolutePath('/workspace/project'));
  assert.equal(relativeScope('c:/CODE/project/pkg/main.go', 'C:\\code\\project'), 'pkg/main.go');
  assert.equal(relativeScope('c:/CODE/project/', 'C:\\code\\project'), '.');
  assert.equal(relativeScope('C:/code/project-other', 'C:/code/project'), null);
  assert.equal(relativeScope('D:/code/project', 'C:/code/project'), null);
  assert.equal(relativeScope('/Workspace/project/pkg', '/workspace/project'), null);
});

test('scan remains usable after absolute input becomes a relative server target', () => {
  const info = binding();
  const scan = { project_id: 5, request_target: 'C:\\code\\project\\pkg', binding_id: info.binding_id, result: scanned };
  assert.equal(scanMatchesInput(scan, 'c:/CODE/project/PKG/', info), scanned);
  assert.equal(scanMatchesInput(scan, 'C:/code/project/other', info), null);
  assert.equal(scanMatchesInput(scan, 'C:/code/project/pkg', binding({ binding_id: 'new-binding' })), null);
  assert.equal(scanMatchesInput(scan, 'C:/code/project/pkg', binding({ available: false })), null);
  assert.equal(scanMatchesInput({ ...scan, result: { ...scanned, target_path: 'other' } }, scan.request_target, info), null);
});

test('current reports match both binding and normalized scope; old and different scopes remain separate', () => {
  const job = { project_id: 5, target: 'pkg', scope_binding: 'opaque-binding-one' };
  assert.equal(jobMatchesScope(job, 'c:/CODE/project/PKG', binding()), true);
  assert.equal(jobMatchesScope(job, 'C:/code/project/other', binding()), false);
  assert.equal(jobMatchesScope(job, 'C:/code/project/pkg', binding({ binding_id: 'new-binding' })), false);
  assert.equal(jobMatchesScope({ project_id: 5, target: 'pkg' }, 'C:/code/project/pkg', binding()), false);
  assert.equal(jobMatchesScope({ project_id: 5, target: 'Pkg', scope_binding: 'opaque-binding-one' }, '/workspace/project/pkg', binding({ host_root: '/workspace/project' })), false);
});

test('missing and unreadable local directories give startup guidance without discarding displayed root', () => {
  assert.match(sourceBindingProblem(null, true, null), /正在读取/);
  assert.match(sourceBindingProblem(null, false, '读取失败'), /读取失败/);
  const missing = binding({ binding_configured: false, absolute_input_allowed: false, host_root: undefined, binding_id: '' });
  const guidance = sourceBindingProblem(missing, false, null);
  assert.match(guidance, /尚未选择本地源码目录/);
  assert.match(guidance, /-SourceDirectory/);
  assert.match(guidance, /--source-directory/);
  assert.doesNotMatch(guidance, /管理员|owner_id|project_id|JSON/);
  assert.match(sourceSetupCommands.powershell, /^\.\/scripts\/deploy\.ps1 -Mode web -SourceDirectory 'C:\\code\\project'$/);
  assert.match(sourceSetupCommands.bash, /^bash scripts\/deploy\.sh --mode web --source-directory '\/workspace\/project'$/);
  const unmounted = binding({ available: false, unavailable_code: 'source_unavailable' });
  assert.match(sourceBindingProblem(unmounted, false, null), /不可读取或未挂载/);
  assert.doesNotMatch(sourceBindingProblem(unmounted, false, null), /管理员/);
  assert.equal(unmounted.host_root, 'C:\\code\\project');
  assert.equal(sourceBindingProblem(binding(), false, null), null);
});

test('local source is immediately usable by existing projects, while old API modes stay compatible', async () => {
  const requested = [];
  apiClient.defaults.adapter = async config => {
    requested.push(config.url);
    const projectId = Number(config.url.split('/')[2]);
    return response(config, binding({ project_id: projectId, source_mode: 'local' }));
  };
  for (const projectId of [5, 6]) {
    const controller = new SourceBindingController(projectId, workspaceApi.getSource, () => '读取失败');
    const stop = controller.subscribe(() => {}); await tick();
    const current = controller.getSnapshot();
    assert.equal(current.info.project_id, projectId);
    assert.equal(current.info.host_root, 'C:\\code\\project');
    assert.equal(sourceMode(current.info), 'local');
    assert.equal(sourceBindingProblem(current.info, current.loading, current.error), null);
    assert.equal(relativeScope(current.info.host_root, current.info.host_root), '.');
    stop();
  }
  assert.deepEqual(requested, ['/projects/5/source', '/projects/6/source']);
  assert.equal(sourceMode(binding()), 'bound');
  assert.equal(sourceMode(binding({ source_mode: 'bound' })), 'bound');
  const legacy = binding({ source_mode: 'legacy', binding_configured: false, absolute_input_allowed: false, host_root: undefined });
  assert.equal(sourceMode(legacy), 'legacy');
  assert.ok(sourceBindingProblem(legacy, false, null));
  assert.equal(sourceMode({ ...legacy, source_mode: undefined }), 'legacy');
});

test('a shared local root and binding identity cannot associate another project scan or task', () => {
  const first = binding({ source_mode: 'local' });
  const second = binding({ source_mode: 'local', project_id: 6 });
  const scan = { project_id: 5, request_target: first.host_root, binding_id: first.binding_id, result: { ...scanned, target_path: '.' } };
  const job = { project_id: 5, target: '.', scope_binding: first.binding_id };
  assert.equal(scanMatchesInput(scan, first.host_root, first), scan.result);
  assert.equal(scanMatchesInput(scan, second.host_root, second), null);
  assert.equal(jobMatchesScope(job, first.host_root, first), true);
  assert.equal(jobMatchesScope(job, second.host_root, second), false);
  const storage = memoryStorage();
  const jobs = new ProjectJobsController(1, 5, {}, storage);
  jobs.track({ job_id: 'local-task', project_id: 5, status: 'queued' }, 'analyze', '.', first.binding_id);
  assert.equal(new ProjectJobsController(1, 5, {}, storage).getSnapshot().jobs.length, 1);
  assert.equal(new ProjectJobsController(1, 6, {}, storage).getSnapshot().jobs.length, 0);
  assert.equal(new ProjectJobsController(2, 5, {}, storage).getSnapshot().jobs.length, 0);
});

test('refreshing a changed local root invalidates earlier scans and reports until explicitly run again', async () => {
  let currentInfo = binding({ source_mode: 'local' });
  const controller = new SourceBindingController(5, async () => currentInfo, () => '读取失败');
  const stop = controller.subscribe(() => {}); await tick();
  const oldScan = { project_id: 5, request_target: currentInfo.host_root, binding_id: currentInfo.binding_id, result: { ...scanned, target_path: '.' } };
  const oldJob = { project_id: 5, target: '.', scope_binding: currentInfo.binding_id };
  currentInfo = binding({ source_mode: 'local', host_root: 'C:\\code\\next-project', binding_id: 'changed-local-directory' });
  const refreshing = controller.refresh();
  assert.equal(controller.getSnapshot().info, null);
  assert.equal(scanMatchesInput(oldScan, oldScan.request_target, controller.getSnapshot().info), null);
  await refreshing;
  const refreshed = controller.getSnapshot().info;
  assert.equal(refreshed.host_root, currentInfo.host_root);
  assert.equal(sourceBindingProblem(refreshed, false, null), null);
  assert.equal(scanMatchesInput(oldScan, refreshed.host_root, refreshed), null);
  assert.equal(jobMatchesScope(oldJob, refreshed.host_root, refreshed), false);
  const sameRootChangedExclusions = binding({ source_mode: 'local', binding_id: 'changed-exclusions', excluded_paths: ['generated'] });
  assert.equal(scanMatchesInput(oldScan, oldScan.request_target, sameRootChangedExclusions), null);
  assert.equal(jobMatchesScope(oldJob, oldScan.request_target, sameRootChangedExclusions), false);
  stop();
});

test('late source information is ignored after project switch or refresh', async () => {
  const pending = [];
  const controller = new SourceBindingController(5, (_, signal) => new Promise(resolve => pending.push({ signal, resolve })), () => '读取失败');
  const unsubscribe = controller.subscribe(() => {});
  const refreshed = controller.refresh();
  assert.equal(pending[0].signal.aborted, true);
  pending[1].resolve(binding({ binding_id: 'latest-binding' })); await refreshed;
  pending[0].resolve(binding()); await tick();
  assert.equal(controller.getSnapshot().info.binding_id, 'latest-binding');
  const delayed = controller.refresh(); unsubscribe();
  assert.equal(pending[2].signal.aborted, true);
  pending[2].resolve(binding()); await delayed;
  assert.equal(controller.getSnapshot().info, null);
  const replacement = new SourceBindingController(6, async () => binding({ project_id: 6, host_root: '/workspace/new-project', binding_id: 'other-project' }), () => '读取失败');
  const stop = replacement.subscribe(() => {}); await tick();
  assert.equal(replacement.getSnapshot().info.project_id, 6); stop();
});

test('source information cannot be applied to a different project and failures stay visible', async () => {
  const controller = new SourceBindingController(5, async () => binding({ project_id: 6 }), () => '绑定信息与当前项目不符');
  const stop = controller.subscribe(() => {}); await tick();
  assert.equal(controller.getSnapshot().info, null);
  assert.match(controller.getSnapshot().error, /当前项目不符/); stop();
});

test('failed binding refresh clears prior executable permissions rather than trusting stale source information', async () => {
  let offline = false;
  const controller = new SourceBindingController(5, async () => {
    if (offline) throw new Error('offline');
    return binding();
  }, () => '无法读取绑定');
  const stop = controller.subscribe(() => {}); await tick();
  assert.equal(sourceBindingProblem(controller.getSnapshot().info, false, null), null);
  offline = true; await controller.refresh();
  const current = controller.getSnapshot();
  assert.equal(current.info, null);
  assert.match(sourceBindingProblem(current.info, current.loading, current.error), /无法读取绑定/);
  stop();
});

test('late source response from another account cannot expose an executable binding', async () => {
  resetSession(); setSessionToken('old-test-session');
  let complete;
  apiClient.defaults.adapter = config => new Promise(resolve => { complete = () => resolve(response(config, binding())); });
  const controller = new SourceBindingController(5, workspaceApi.getSource, () => '会话已变更，请重新读取绑定');
  const stop = controller.subscribe(() => {}); await tick();
  resetSession(); setSessionToken('new-test-session');
  complete(); await tick();
  assert.equal(controller.getSnapshot().info, null);
  assert.match(controller.getSnapshot().error, /会话已变更/); stop(); resetSession();
});

test('absolute scope reaches scan/analyze/index unchanged; only server relative target is remembered', async () => {
  const requests = [];
  apiClient.defaults.adapter = async config => {
    requests.push({ url: config.url, data: config.data && JSON.parse(config.data) });
    if (config.url.endsWith('/source')) return response(config, binding());
    if (config.url.endsWith('/scan')) return response(config, scanned);
    return response(config, { job_id: config.url.endsWith('/index') ? 'index-job' : 'analyze-job', project_id: 5, status: 'queued' }, 202);
  };
  const target = 'C:\\code\\project\\pkg';
  const info = await workspaceApi.getSource(5);
  const scan = await workspaceApi.scan(5, target);
  assert.equal(scanMatchesInput({ project_id: 5, request_target: target, binding_id: info.binding_id, result: scan }, target, info), scan);
  const analyze = await workspaceApi.analyze(5, target); const index = await workspaceApi.index(5, target);
  assert.deepEqual(requests.slice(1).map(item => item.data.target), [target, target, target]);
  const storage = memoryStorage();
  const jobs = new ProjectJobsController(1, 5, { getJob: async id => ({ job_id: id, project_id: 5, kind: 'analyze', status: 'completed', target: 'pkg', updated_at: '', created_at: '', result: null }), cancelJob: async () => ({}) }, storage);
  jobs.track(analyze, 'analyze', scan.target_path, info.binding_id); jobs.track(index, 'index', scan.target_path, info.binding_id);
  const stop = jobs.subscribe(() => {}); await tick();
  assert.equal(jobs.getSnapshot().jobs[0].scope_binding, info.binding_id);
  assert.equal(jobMatchesScope(jobs.getSnapshot().jobs[0], 'c:/code/PROJECT/PKG', info), true);
  assert.equal(storage.getItem('gosource.jobs.1.5').includes('C:'), false); stop();
  assert.equal(new ProjectJobsController(1, 5, {}, storage).getSnapshot().jobs[0].scope_binding, info.binding_id);
});

test('outside, unsafe and excluded scope errors are understandable without exposing backend roots', () => {
  for (const [code, phrase] of [['invalid_target', /当前本地源码目录/], ['unsafe_path', /源码边界/], ['excluded_target', /排除范围/]]) {
    const error = new AxiosError('request failed', 'ERR_BAD_REQUEST', {}, null, { data: { code, error: 'safe message' }, status: 403 });
    assert.match(getApiErrorMessage(error), phrase);
  }
});
