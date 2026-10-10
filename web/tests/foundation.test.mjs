import test from 'node:test';
import assert from 'node:assert/strict';
import { AxiosError } from 'axios';
import { ProjectJobsController } from '../src/utils/projectJobs.ts';
import { apiClient } from '../src/api/client.ts';
import { useAuthStore } from '../src/stores/authStore.ts';
import { useProjectStore } from '../src/stores/projectStore.ts';
import { resetSession, setSessionToken } from '../src/utils/session.ts';

function memoryStorage() {
  const entries = new Map();
  return { get length() { return entries.size; }, key: index => [...entries.keys()][index] || null,
    getItem: key => entries.get(key) || null, setItem: (key, value) => entries.set(key, value), removeItem: key => entries.delete(key) };
}
globalThis.sessionStorage = memoryStorage();
globalThis.localStorage = memoryStorage();
const user = id => ({ id, username: `test${id}`, email: 'test@example.invalid', created_at: '', updated_at: '' });
const response = (config, data, status = 200) => ({ config, data, status, statusText: 'OK', headers: {} });
const tick = () => new Promise(resolve => setImmediate(resolve));
const job = (status, time = 1) => ({ job_id: 'job-a', project_id: 5, kind: 'index', target: '.', status,
  result: status === 'completed' ? { chunks_indexed: 4 } : null, created_at: '', updated_at: `2026-10-09T00:00:0${time}Z`, started_at: null, finished_at: null });

test('restore validates profile before authentication and removes legacy long-lived credentials', async () => {
  resetSession();
  useAuthStore.setState({ initialized: false });
  sessionStorage.setItem('gosource.session.token', 'restored-test-token');
  localStorage.setItem('token', 'legacy-token'); localStorage.setItem('user', '{}');
  let complete;
  apiClient.defaults.adapter = config => new Promise(resolve => { complete = () => resolve(response(config, user(7))); });
  const pending = useAuthStore.getState().initialize();
  assert.equal(useAuthStore.getState().isAuthenticated, false);
  assert.equal(useAuthStore.getState().initialized, false);
  await tick(); complete(); await pending;
  assert.equal(useAuthStore.getState().user.id, 7);
  assert.equal(useAuthStore.getState().initialized, true);
  assert.equal(localStorage.getItem('token'), null);
  assert.equal(localStorage.getItem('user'), null);
});

test('401 clears current user and known job state; stale 401 cannot clear replacement session', async () => {
  setSessionToken('current-token');
  sessionStorage.setItem('gosource.jobs.7.5', '[]');
  apiClient.defaults.adapter = async config => { throw new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, response(config, {}, 401)); };
  await assert.rejects(apiClient.get('/jobs/test'));
  assert.equal(useAuthStore.getState().isAuthenticated, false);
  assert.equal(sessionStorage.getItem('gosource.jobs.7.5'), null);
  setSessionToken('old-token');
  let fail;
  apiClient.defaults.adapter = config => new Promise((_, reject) => { fail = () => reject(new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, response(config, {}, 401))); });
  const pending = apiClient.get('/jobs/old'); await tick();
  resetSession(); setSessionToken('new-token');
  useAuthStore.setState({ user: user(9), isAuthenticated: true });
  fail(); await assert.rejects(pending, error => error.code === 'ERR_CANCELED');
  assert.equal(useAuthStore.getState().user.id, 9);
  assert.equal(useAuthStore.getState().isAuthenticated, true);
});

test('logout while profile is delayed never revives the old session', async () => {
  resetSession(); setSessionToken('old-profile');
  let complete;
  apiClient.defaults.adapter = config => new Promise(resolve => { complete = () => resolve(response(config, user(1))); });
  const pending = useAuthStore.getState().fetchProfile(); await tick();
  useAuthStore.getState().logout(); complete(); await pending;
  assert.equal(useAuthStore.getState().user, null);
  assert.equal(useAuthStore.getState().isAuthenticated, false);
});

test('register sends only accepted API fields', async () => {
  let body;
  apiClient.defaults.adapter = async config => { body = JSON.parse(config.data); return response(config, { token: 'test-token', user: user(3) }, 201); };
  await useAuthStore.getState().register({ username: 'test3', email: 'test@example.invalid', password: 'test-password', confirmPassword: 'test-password' });
  assert.deepEqual(Object.keys(body).sort(), ['email', 'password', 'username']);
});

test('late login cannot replace a newer account and project store clears on session switch', async () => {
  const responses = [];
  apiClient.defaults.adapter = config => new Promise(resolve => responses.push(() => resolve(response(config, {
    token: `token-${responses.length}`, user: user(responses.length),
  }))));
  const oldLogin = useAuthStore.getState().login({ username: 'old-test', password: 'test-password' });
  await tick();
  useProjectStore.setState({ projects: [{ id: 99 }], currentProject: { id: 99 } });
  const newLogin = useAuthStore.getState().login({ username: 'new-test', password: 'test-password' });
  await tick();
  assert.equal(useProjectStore.getState().projects.length, 0);
  assert.equal(useProjectStore.getState().currentProject, null);
  responses[1](); await newLogin;
  responses[0](); await assert.rejects(oldLogin, error => error.code === 'ERR_CANCELED');
  assert.equal(useAuthStore.getState().user.id, 2);
});

test('known jobs restore within owner/project and polling requests are deduplicated', async () => {
  const storage = memoryStorage();
  let complete; let calls = 0;
  const transport = { getJob: () => { calls++; return new Promise(resolve => { complete = resolve; }); }, cancelJob: async () => job('canceling', 2) };
  const first = new ProjectJobsController(1, 5, transport, storage);
  first.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  const second = new ProjectJobsController(1, 5, transport, storage);
  assert.equal(second.getSnapshot().jobs[0].recovering, true);
  assert.equal(new ProjectJobsController(2, 5, transport, storage).getSnapshot().jobs.length, 0);
  assert.equal(new ProjectJobsController(1, 6, transport, storage).getSnapshot().jobs.length, 0);
  const unsubscribe = second.subscribe(() => {});
  second.refresh(); second.refresh(); assert.equal(calls, 1);
  complete(job('completed', 3)); await tick();
  assert.equal(second.getSnapshot().jobs[0].result.chunks_indexed, 4);
  assert.equal(second.getSnapshot().jobs[0].recovering, false);
  unsubscribe();
});

test('canceling remains active and delayed poll cannot undo completed publication', async () => {
  let complete; let cancelCalls = 0;
  const controller = new ProjectJobsController(1, 5, { getJob: () => new Promise(resolve => { complete = resolve; }),
    cancelJob: async () => { cancelCalls++; return job('completed', 3); } });
  controller.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  const unsubscribe = controller.subscribe(() => {});
  const a = controller.cancel('job-a'); const b = controller.cancel('job-a');
  await Promise.all([a, b]); assert.equal(cancelCalls, 1);
  complete(job('running', 1)); await tick();
  assert.equal(controller.getSnapshot().jobs[0].status, 'completed');
  unsubscribe();
  const canceling = new ProjectJobsController(1, 5, { getJob: async () => job('queued'), cancelJob: async () => job('canceling', 2) });
  canceling.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  await canceling.cancel('job-a');
  assert.equal(canceling.getSnapshot().jobs[0].status, 'canceling');
});

test('unsubscribing aborts request and ignores a delayed response', async () => {
  let complete; let signal;
  const controller = new ProjectJobsController(1, 5, { getJob: (_, current) => { signal = current; return new Promise(resolve => { complete = resolve; }); }, cancelJob: async () => job('canceled') });
  controller.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  const unsubscribe = controller.subscribe(() => {}); unsubscribe();
  assert.equal(signal.aborted, true); complete(job('completed', 3)); await tick();
  assert.equal(controller.getSnapshot().jobs[0].status, 'queued');
});

test('late running poll cannot undo canceling when timestamps round to the same millisecond', async () => {
  let complete;
  const controller = new ProjectJobsController(1, 5, {
    getJob: () => new Promise(resolve => { complete = resolve; }),
    cancelJob: async () => ({ ...job('canceling'), updated_at: '2026-10-09T00:00:00.100900Z' }),
  });
  controller.track({ job_id: 'job-a', project_id: 5, status: 'running' }, 'index');
  const unsubscribe = controller.subscribe(() => {});
  await controller.cancel('job-a');
  complete({ ...job('running'), updated_at: '2026-10-09T00:00:00.100100Z' });
  await tick();
  assert.equal(controller.getSnapshot().jobs[0].status, 'canceling');
  unsubscribe();
});

test('an accepted submission arriving after leaving a project still records the correct project', () => {
  const storage = memoryStorage();
  const transport = { getJob: async () => job('completed'), cancelJob: async () => job('canceled') };
  const controller = new ProjectJobsController(1, 5, transport, storage);
  const unsubscribe = controller.subscribe(() => {}); unsubscribe();
  controller.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  assert.equal(new ProjectJobsController(1, 5, transport, storage).getSnapshot().jobs[0].job_id, 'job-a');
  assert.equal(new ProjectJobsController(1, 6, transport, storage).getSnapshot().jobs.length, 0);
});

test('cancel failure remains visible without inventing a terminal state', async () => {
  const controller = new ProjectJobsController(1, 5, { getJob: async () => job('running'), cancelJob: async () => { throw new Error('offline'); } }, undefined, () => '取消请求失败');
  controller.track({ job_id: 'job-a', project_id: 5, status: 'running' }, 'index');
  await assert.rejects(controller.cancel('job-a'));
  assert.equal(controller.getSnapshot().jobs[0].status, 'running');
  assert.equal(controller.getSnapshot().error, '取消请求失败');
});

test('poll retries are finite, do not replay interrupted or unknown publications', async () => {
  let calls = 0;
  const controller = new ProjectJobsController(1, 5, { getJob: async () => { calls++; throw new Error('offline'); }, cancelJob: async () => job('canceled') }, undefined, () => '离线', 1);
  controller.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  let observePause;
  const paused = new Promise((resolve, reject) => {
    const deadline = setTimeout(() => reject(new Error('Polling did not pause within two seconds')), 2000);
    observePause = () => {
      const snapshot = controller.getSnapshot();
      if (!snapshot.loading && /自动重试已暂停/.test(snapshot.error || '')) { clearTimeout(deadline); resolve(); }
    };
  });
  const unsubscribe = controller.subscribe(observePause);
  try {
    await paused;
    assert.equal(calls, 5); assert.match(controller.getSnapshot().error, /自动重试已暂停/);
    await new Promise(resolve => setTimeout(resolve, 60));
    assert.equal(calls, 5, 'paused polling must not keep requesting');
  } finally { unsubscribe(); }
  const terminal = new ProjectJobsController(1, 5, { getJob: async () => ({ ...job('failed'), failure_code: 'index_publication_unknown' }), cancelJob: async () => job('canceled') });
  terminal.track({ job_id: 'job-a', project_id: 5, status: 'queued' }, 'index');
  const stop = terminal.subscribe(() => {}); await tick();
  assert.equal(terminal.getSnapshot().jobs[0].failure_code, 'index_publication_unknown'); stop();
});
