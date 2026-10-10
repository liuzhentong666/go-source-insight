import { test } from 'node:test';
import assert from 'node:assert/strict';
import { failureHint, jobLabel, latestJob, readStaticReport, relativePathProblem, statusLabels } from '../src/pages/Workspace/model.ts';

test('UI does not present canceling as canceled', () => {
  assert.notEqual(statusLabels.canceling.text, statusLabels.canceled.text);
  assert.match(statusLabels.canceling.text, /等待服务端确认/);
  assert.equal(jobLabel({ recovering: true, status: 'queued' }), '恢复状态中');
});

test('relative targets reject absolute paths, traversal and Windows forms', () => {
  for (const path of ['/tmp/source', 'C:/source', '\\server\\share', 'pkg/../secret', 'pkg\\file.go', 'pkg:stream', 'pkg//a']) assert.ok(relativePathProblem(path), path);
  for (const path of ['', '.', 'pkg', 'pkg/main.go']) assert.equal(relativePathProblem(path), null, path);
  assert.ok(relativePathProblem('.', false));
});

test('analysis result must come from completed static job; failed partial results stay hidden', () => {
  const report = { target_path: '.', files: [], files_analyzed: 0 };
  assert.equal(readStaticReport({ status: 'completed', result: report }), report);
  assert.equal(readStaticReport({ recovering: true, status: 'completed', result: report }), null);
  for (const status of ['queued', 'running', 'canceling', 'canceled', 'failed']) assert.equal(readStaticReport({ status, result: report }), null);
  assert.equal(readStaticReport({ status: 'completed', result: { chunks_indexed: 3 } }), null);
  assert.equal(readStaticReport(undefined), null);
});

test('latest job selects the kind and latest submission even if it failed', () => {
  const earlier = { job_id: 'a', kind: 'analyze', status: 'completed', created_at: '2026-01-01T00:00:00Z' };
  const newer = { job_id: 'b', kind: 'analyze', status: 'failed', created_at: '2026-01-02T00:00:00Z' };
  const index = { job_id: 'c', kind: 'index', created_at: '2026-01-03T00:00:00Z' };
  const jobs = [earlier, index, newer];
  assert.equal(latestJob(jobs, 'analyze'), newer);
  assert.deepEqual(jobs, [earlier, index, newer]);
  assert.equal(latestJob(jobs, 'legacy_analyze'), undefined);
  const accepted = { job_id: 'd', kind: 'analyze', status: 'queued', created_at: '' };
  assert.equal(latestJob([...jobs, accepted], 'analyze'), accepted);
});

test('publication uncertainty and restart failures instruct inspection before manual retry', () => {
  assert.match(failureHint('index_publication_unknown'), /可能已经生效.*先通过问答检查/);
  assert.match(failureHint('interrupted'), /没有自动重放/);
  assert.match(failureHint('queue_full'), /没有创建任务/);
  assert.equal(failureHint('unknown'), null);
});
