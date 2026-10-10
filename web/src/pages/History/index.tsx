import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Empty, Select, Space, Table, Tabs, Tag, Typography } from 'antd';
import { Link } from 'react-router-dom';
import { projectApi } from '../../api/project';
import { analysisApi } from '../../api/analysis';
import { getApiErrorMessage, isRequestCanceled } from '../../api/client';
import { useAuthStore } from '../../stores/authStore';
import type { Analysis, AnalysisResult as LegacyResult, Project } from '../../types';
import { Complexity, Security, Bugs } from '../../components/AnalysisResult';
import { formatTime, statusLabels } from '../Workspace/model';
import '../Workspace/workspace.css';

function parseResult(row: Analysis): LegacyResult | null {
  if (row.status !== 'completed' || !row.result) return null;
  try { const value = JSON.parse(row.result); return value && typeof value === 'object' && ('complexity' in value || 'security' in value || 'bugs' in value) ? value : null; } catch { return null; }
}

function LegacyReport({ row }: { row: Analysis }) {
  const result = parseResult(row);
  if (!result) return <Typography.Text type="secondary">{row.status === 'completed' ? '已完成记录没有可读取的旧版报告。' : '任务尚无完成报告。请查看项目工作台的已知任务状态。'}</Typography.Text>;
  return <Tabs items={[
    { key: 'complexity', label: '复杂度', children: result.complexity ? <Complexity data={result.complexity} /> : <Empty description="没有复杂度结果" /> },
    { key: 'security', label: '安全规则', children: result.security ? <Security data={result.security} /> : <Empty description="没有安全结果" /> },
    { key: 'bugs', label: '缺陷候选', children: result.bugs ? <Bugs data={result.bugs} /> : <Empty description="没有缺陷结果" /> },
  ]} />;
}

function HistoryForOwner() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selected, setSelected] = useState<number | undefined>();
  const [analyses, setAnalyses] = useState<Analysis[]>([]);
  const [loading, setLoading] = useState(false);
  const [projectsLoading, setProjectsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);
  const currentProject = useRef<number | undefined>(undefined);
  useEffect(() => {
    const controller = new AbortController();
    projectApi.getProjects(controller.signal).then((data) => { if (!controller.signal.aborted) setProjects(data || []); }).catch((caught) => { if (!controller.signal.aborted && !isRequestCanceled(caught)) setError(getApiErrorMessage(caught)); }).finally(() => { if (!controller.signal.aborted) setProjectsLoading(false); });
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!selected) return;
    const controller = new AbortController();
    analysisApi.getAnalysisResults(selected, controller.signal).then((data) => { if (!controller.signal.aborted && currentProject.current === selected) setAnalyses(data || []); }).catch((caught) => { if (!controller.signal.aborted && currentProject.current === selected && !isRequestCanceled(caught)) setError(getApiErrorMessage(caught)); }).finally(() => { if (!controller.signal.aborted && currentProject.current === selected) setLoading(false); });
    return () => controller.abort();
  }, [selected, revision]);
  const select = (value: number | undefined) => { currentProject.current = value; setSelected(value); setAnalyses([]); setError(null); setLoading(Boolean(value)); };
  return <div className="workspace workspace-stack">
    <div className="workspace-header"><div><Typography.Title level={2}>旧版分析历史</Typography.Title><Typography.Text type="secondary">服务端保存的代码片段分析报告。源码工作台的任务记录仅保留本浏览器已知任务。</Typography.Text></div><Link to="/workspace"><Button>打开项目工作台</Button></Link></div>
    <Space wrap><Select aria-label="选择历史项目" placeholder="选择项目" style={{ width: 240 }} value={selected} onChange={select} allowClear loading={projectsLoading} options={projects.map((project) => ({ value: project.id, label: project.name }))} /><Button disabled={!selected || loading} loading={loading} onClick={() => { setLoading(true); setError(null); setRevision((value) => value + 1); }}>刷新历史</Button>{selected && <Link to={`/workspace/${selected}?tab=tasks`}>查看项目任务</Link>}</Space>
    {error && <Alert type="error" showIcon title="历史读取失败" description={error} />}
    <Card><Table<Analysis> dataSource={analyses} rowKey="id" loading={loading} scroll={{ x: 620 }} pagination={{ pageSize: 10 }} columns={[
      { title: '记录 ID', dataIndex: 'id', width: 100 },
      { title: '状态', dataIndex: 'status', render: (status: string) => <Tag color={statusLabels[status]?.color}>{statusLabels[status]?.text || status}</Tag> },
      { title: '提交时间', dataIndex: 'created_at', render: (value: string) => formatTime(value) },
      { title: '更新时间', dataIndex: 'updated_at', render: (value: string) => formatTime(value) },
    ]} expandable={{ expandedRowRender: (row) => <LegacyReport row={row} /> }} locale={{ emptyText: selected ? '此项目没有旧版分析记录' : '请先选择一个项目' }} /></Card>
  </div>;
}

export default function History() {
  const owner = useAuthStore((state) => state.user?.id);
  return owner ? <HistoryForOwner key={owner} /> : null;
}
