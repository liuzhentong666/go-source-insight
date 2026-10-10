import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { Alert, Button, Card, Empty, Form, Input, Modal, Popconfirm, Space, Spin, Statistic, Steps, Table, Tabs, Tag, Typography } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { projectApi } from '../../api/project';
import { workspaceApi } from '../../api/workspace';
import { getApiErrorMessage, isRequestCanceled } from '../../api/client';
import { useAuthStore } from '../../stores/authStore';
import { isJobActive, useProjectJobs } from '../../hooks/useProjectJobs';
import { useProjectSource } from '../../hooks/useProjectSource';
import type { CreateProjectRequest, IndexSummary, Project } from '../../types';
import type { ScopedScan } from '../../utils/sourceScope';
import { absolutePathProblem, jobMatchesScope, relativeScope, scanMatchesInput, sourceBindingProblem, sourceMode, sourceSetupCommands } from '../../utils/sourceScope';
import StaticReportView from '../../components/Workspace/StaticReport';
import TaskList from '../../components/Workspace/TaskList';
import QuestionPanel from '../../components/Workspace/QuestionPanel';
import { failureHint, formatTime, jobLabel, latestJob, readStaticReport, statusLabels } from './model';
import './workspace.css';

function ProjectWorkspace({ project, onDeleted }: { project: Project; onDeleted: () => void }) {
  const { jobs, loading: jobsLoading, error: jobsError, trackJob, refresh, cancel } = useProjectJobs(project.id);
  const { info: sourceInfo, loading: sourceLoading, error: sourceError, refreshSource } = useProjectSource(project.id);
  const [searchParams, setSearchParams] = useSearchParams();
  const [target, setTarget] = useState<string | null>(null);
  const [scan, setScan] = useState<ScopedScan | null>(null);
  const [busy, setBusy] = useState<'scan' | 'analyze' | 'index' | 'delete' | null>(null);
  const [operationError, setOperationError] = useState<string | null>(null);
  const [confirmIndex, setConfirmIndex] = useState(false);
  const [asking, setAsking] = useState(false);
  const mounted = useRef(true);
  const scanRequest = useRef<AbortController | null>(null);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; scanRequest.current?.abort(); }; }, []);
  const actualTarget = (target ?? sourceInfo?.host_root ?? '').trim();
  const bindingProblem = sourceBindingProblem(sourceInfo, sourceLoading, sourceError);
  const pathProblem = absolutePathProblem(actualTarget) || (sourceInfo?.host_root && relativeScope(actualTarget, sourceInfo.host_root) === null ? '完整路径必须位于当前本地源码目录内。' : null);
  const matchingScan = scanMatchesInput(scan, actualTarget, sourceInfo);
  const scopeJobs = jobs.filter(job => jobMatchesScope(job, actualTarget, sourceInfo));
  const latestAnalysis = latestJob(scopeJobs, 'analyze');
  const latestIndex = latestJob(scopeJobs, 'index');
  const projectLatestIndex = latestJob(jobs, 'index');
  const report = readStaticReport(latestAnalysis);
  const active = jobs.some((job) => isJobActive(job.status));
  const indexing = jobs.some((job) => job.kind === 'index' && isJobActive(job.status));
  const indexResult = latestIndex?.status === 'completed' && latestIndex.result && 'chunks_indexed' in latestIndex.result ? latestIndex.result as IndexSummary : null;
  const publishedIndex = projectLatestIndex?.status === 'completed' && projectLatestIndex.result && 'chunks_indexed' in projectLatestIndex.result ? projectLatestIndex.result as IndexSummary : null;
  const tab = searchParams.get('tab') || 'source';
  const chooseTab = (key: string) => setSearchParams({ tab: key }, { replace: true });

  const scanSource = async () => {
    if (busy || bindingProblem || pathProblem || !sourceInfo?.binding_id) return;
    setBusy('scan'); setOperationError(null); setScan(null);
    const controller = new AbortController();
    scanRequest.current = controller;
    const bindingId = sourceInfo.binding_id;
    try { const result = await workspaceApi.scan(project.id, actualTarget, controller.signal); if (mounted.current && !controller.signal.aborted) setScan({ project_id: project.id, request_target: actualTarget, binding_id: bindingId, result }); }
    catch (error) { if (mounted.current && !isRequestCanceled(error)) setOperationError(getApiErrorMessage(error)); }
    finally { if (mounted.current) setBusy(null); }
  };

  const submit = async (kind: 'analyze' | 'index') => {
    if (busy || asking || bindingProblem || pathProblem || !matchingScan || active || !sourceInfo?.binding_id) return;
    setConfirmIndex(false); setBusy(kind); setOperationError(null);
    try {
      const accepted = await workspaceApi[kind](project.id, actualTarget);
      trackJob(accepted, kind, matchingScan.target_path, sourceInfo.binding_id);
      if (mounted.current) chooseTab(kind === 'analyze' ? 'analysis' : 'tasks');
    } catch (error) { if (mounted.current && !isRequestCanceled(error)) setOperationError(getApiErrorMessage(error)); }
    finally { if (mounted.current) setBusy(null); }
  };

  const remove = async () => {
    setBusy('delete'); setOperationError(null);
    try { await projectApi.deleteProject(project.id); if (mounted.current) onDeleted(); }
    catch (error) { if (mounted.current && !isRequestCanceled(error)) setOperationError(getApiErrorMessage(error)); }
    finally { if (mounted.current) setBusy(null); }
  };

  const actionReason = bindingProblem || pathProblem || (asking ? '正在等待此项目的问答响应，请结束等待后再提交任务。' : active ? '当前项目有未结束任务，请等待或到任务记录中取消。' : !matchingScan ? '先扫描当前完整路径，确认源码已就绪。' : matchingScan.total_files === 0 ? '当前范围没有 Go 文件，不能建立索引或分析。' : null);
  return <div className="workspace-stack">
    <Card>
      <div className="workspace-header"><div><div className="workspace-eyebrow">PROJECT / {project.id}</div><Typography.Title level={3}>{project.name}</Typography.Title><Typography.Text type="secondary">{project.description || '选择本地源码范围，开始一次分析。'}</Typography.Text></div><Popconfirm title={`删除项目「${project.name}」？`} description="项目元数据将被删除。此操作无法撤销。" onConfirm={remove} okText="删除项目" okButtonProps={{ danger: true }}><Button danger disabled={active || asking || Boolean(busy)}>删除项目</Button></Popconfirm></div>
      <Space wrap><Tag color={matchingScan ? 'success' : 'default'}>{matchingScan ? '当前范围已扫描' : '当前范围未扫描'}</Tag><Tag color={latestAnalysis ? latestAnalysis.recovering ? 'processing' : statusLabels[latestAnalysis.status]?.color : 'default'}>当前范围分析：{latestAnalysis ? jobLabel(latestAnalysis) : '无已知记录'}</Tag><Tag color={latestIndex ? latestIndex.recovering ? 'processing' : statusLabels[latestIndex.status]?.color : 'default'}>当前范围索引：{latestIndex ? jobLabel(latestIndex) : '无已知记录'}</Tag></Space>
    </Card>
    <Card>
      <div className="workspace-stack">
        <Steps size="small" current={matchingScan ? report ? indexResult ? 4 : 3 : 2 : 0} items={[{ title: '准备源码' }, { title: '扫描' }, { title: '静态分析' }, { title: '索引' }, { title: '问答' }]} />
        <div className="workspace-help">建议先扫描，再查看静态规则，按需索引后问答。扫描统计生产与测试 Go 文件；静态分析排除测试文件，索引包含测试源码。静态分析与索引可独立执行；扫描不会自动触发后续操作。</div>
        <label className="workspace-scope"><Typography.Text strong>操作范围（完整路径）</Typography.Text><Input value={target ?? sourceInfo?.host_root ?? ''} onChange={(event) => setTarget(event.target.value)} disabled={Boolean(busy) || sourceLoading} placeholder={'C:\\code\\project 或 /workspace/project/main.go'} status={!bindingProblem && pathProblem ? 'error' : undefined} /><Typography.Text type={!bindingProblem && pathProblem ? 'danger' : 'secondary'}>{bindingProblem || pathProblem || '填写启动时选择的本地源码目录，或其中目录、Go 文件的完整路径。'}</Typography.Text></label>
        <Space wrap><Button onClick={() => { setTarget(null); setConfirmIndex(false); }} disabled={!sourceInfo?.host_root || Boolean(busy) || sourceLoading}>使用当前源码目录</Button><Button onClick={() => { setConfirmIndex(false); void refreshSource(); }} loading={sourceLoading} disabled={Boolean(busy)}>刷新目录信息</Button>{!sourceInfo?.binding_configured && !sourceLoading && <Button type="link" onClick={() => chooseTab('source')}>查看启动说明</Button>}</Space>
        <div className="workspace-actions"><Button onClick={() => { void scanSource(); }} loading={busy === 'scan'} disabled={Boolean(busy) || Boolean(bindingProblem) || Boolean(pathProblem)}>扫描源码</Button><Button type="primary" onClick={() => { void submit('analyze'); }} loading={busy === 'analyze'} disabled={Boolean(busy) || Boolean(actionReason) || !matchingScan?.production_file_count}>静态分析</Button><Button onClick={() => setConfirmIndex(true)} loading={busy === 'index'} disabled={Boolean(busy) || Boolean(actionReason)}>建立索引</Button></div>
        {actionReason && <Typography.Text type="secondary">{actionReason}</Typography.Text>}
        {matchingScan && matchingScan.total_files > 0 && matchingScan.production_file_count === 0 && <Typography.Text type="secondary">扫描只发现测试 Go 文件，静态分析会排除测试文件；可按需建立索引。</Typography.Text>}
        {operationError && <Alert type="error" showIcon title="操作未完成" description={operationError} />}
        {projectLatestIndex?.status === 'failed' && ['index_publication_unknown', 'interrupted'].includes(projectLatestIndex.failure_code || '') && <Alert type="warning" showIcon title="请先检查当前索引，再决定是否重新建立" description={failureHint(projectLatestIndex.failure_code)} />}
      </div>
    </Card>
    <Card><Tabs activeKey={['source', 'analysis', 'ask', 'tasks'].includes(tab) ? tab : 'source'} onChange={chooseTab} items={[
      { key: 'source', label: '源码', children: <div className="workspace-stack">
        <Alert type={bindingProblem ? 'warning' : 'info'} showIcon title={sourceLoading ? '正在读取本地源码目录' : sourceInfo?.binding_configured ? '当前本地源码目录' : '启动时选择本地源码目录'} description={<><p>{bindingProblem || '使用下面的目录，或输入其中目录、Go 文件的完整路径。'}</p>{sourceInfo?.host_root && <Typography.Text code copyable>{sourceInfo.host_root}</Typography.Text>}{sourceMode(sourceInfo) === 'local' && <p>启动时选择一次目录，所有项目（包括已有项目）都可直接使用。各项目的分析、任务和索引分别保存。</p>}{!sourceLoading && sourceInfo && !sourceInfo.binding_configured && <><p>在仓库目录运行对应启动命令，将示例路径换成自己的源码目录：</p><p>PowerShell：<Typography.Text code copyable>{sourceSetupCommands.powershell}</Typography.Text></p><p>Bash：<Typography.Text code copyable>{sourceSetupCommands.bash}</Typography.Text></p></>}<p>Docker 中的程序只读取已设置的目录；浏览器不能直接读取任意本机硬盘。目录设置完成后点击“刷新目录信息”。</p>{sourceInfo?.excluded_paths?.length ? <p>额外排除：{sourceInfo.excluded_paths.join('、')}</p> : null}</>} />
        {matchingScan ? <><div className="workspace-metrics"><Statistic title="Go 文件" value={matchingScan.total_files} /><Statistic title="生产文件" value={matchingScan.production_file_count} /><Statistic title="测试文件" value={matchingScan.test_file_count} /><Statistic title="物理行数" value={matchingScan.total_lines} /></div><Table size="small" rowKey="path" dataSource={(matchingScan.files || []).map((path) => ({ path, test: (matchingScan.test_files || []).includes(path) }))} pagination={{ pageSize: 12 }} columns={[{ title: '相对文件路径', dataIndex: 'path', render: (path: string) => <span className="workspace-path">{path}</span> }, { title: '用途', dataIndex: 'test', width: 100, render: (test: boolean) => <Tag>{test ? '测试' : '生产'}</Tag> }]} locale={{ emptyText: '扫描已完成，当前范围有 0 个 Go 文件' }} />{matchingScan.skipped_dirs?.length > 0 && <Typography.Paragraph type="secondary">跳过目录：{matchingScan.skipped_dirs.join('、')}</Typography.Paragraph>}</> : <Empty description="准备源码后，扫描当前范围以确认文件和行数" />}
      </div> },
      { key: 'analysis', label: '分析结果', children: report ? <><Typography.Paragraph type="secondary">当前范围最近一次分析 · {formatTime(latestAnalysis?.finished_at)} · 项目相对范围 {latestAnalysis?.target}</Typography.Paragraph><StaticReportView report={report} /></> : latestAnalysis ? <Alert showIcon type={latestAnalysis.status === 'failed' ? 'error' : latestAnalysis.status === 'canceled' ? 'warning' : 'info'} title={`当前范围最近分析：${jobLabel(latestAnalysis)}`} description={latestAnalysis.failure_code ? failureHint(latestAnalysis.failure_code) || latestAnalysis.failure_reason : latestAnalysis.status === 'completed' ? '任务已完成，但没有可展示的文件报告。' : '查看任务记录了解服务端状态；只有完成的任务才会显示结果。'} /> : <Empty description="当前路径范围没有可关联的分析记录；先扫描当前路径，再提交分析。其他范围或之前目录设置的记录仍可在任务记录中查看。" /> },
      { key: 'ask', label: '源码问答', children: <div className="workspace-stack"><Alert type="info" title="更换目录后需重新建立索引才能更新问答来源" description="问答仍使用项目已发布的快照，修改路径或扫描不会自动更新来源。" />{publishedIndex && <Alert type="success" title={`项目已知索引：${publishedIndex.files_scanned} 个文件 · ${publishedIndex.chunks_indexed} 个片段`} description={`项目相对范围 ${publishedIndex.target_path}；问答使用已发布的整个项目快照，修改操作路径不会自动替换索引。`} />}<QuestionPanel projectId={project.id} indexing={indexing || busy === 'index'} onWaitingChange={setAsking} /></div> },
      { key: 'tasks', label: <>任务记录 {active && <Tag color="processing">进行中</Tag>}</>, children: <TaskList jobs={jobs} loading={jobsLoading} error={jobsError} refresh={refresh} cancel={cancel} /> },
    ]} /></Card>
    <Modal title="确认替换整个项目索引" open={confirmIndex} onCancel={() => setConfirmIndex(false)} onOk={() => { void submit('index'); }} okText="确认建立索引" confirmLoading={busy === 'index'}><div className="workspace-stack"><Typography.Paragraph>本次范围：<Typography.Text code>{actualTarget}</Typography.Text></Typography.Paragraph><Alert type="warning" showIcon title="即使选择子目录或单个文件，也会用本次范围替换整个项目的已发布索引快照。" description="其他文件将不再出现在此项目的问答检索中。取消任务不会撤销已经发布的快照；发布结果无法确认时请先检查当前索引。" /></div></Modal>
  </div>;
}

function WorkspaceForOwner() {
  const navigate = useNavigate();
  const { projectId } = useParams();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm<CreateProjectRequest>();
  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(true);
  const load = useCallback(async () => {
    controller.current?.abort();
    const request = new AbortController(); controller.current = request;
    setLoading(true); setError(null);
    try { const data = await projectApi.getProjects(request.signal); if (!request.signal.aborted) setProjects(data || []); }
    catch (caught) { if (!request.signal.aborted && !isRequestCanceled(caught)) setError(getApiErrorMessage(caught, '无法读取项目列表')); }
    finally { if (!request.signal.aborted) setLoading(false); }
  }, []);
  useEffect(() => { mounted.current = true; void load(); return () => { mounted.current = false; controller.current?.abort(); }; }, [load]);
  const project = projects.find((item) => String(item.id) === projectId);
  const create = async (values: CreateProjectRequest) => {
    setCreating(true); setError(null);
    try {
      const created = await projectApi.createProject({ name: values.name.trim(), description: values.description?.trim() });
      if (!mounted.current) return;
      setProjects((items) => [...items, created]); setCreateOpen(false); form.resetFields(); navigate(`/workspace/${created.id}`);
    } catch (caught) { if (mounted.current && !isRequestCanceled(caught)) setError(getApiErrorMessage(caught, '项目创建失败')); }
    finally { if (mounted.current) setCreating(false); }
  };
  return <div className="workspace">
    <div className="workspace-header"><div><div className="workspace-eyebrow">GO SOURCE INSIGHT</div><Typography.Title level={2}>项目工作台</Typography.Title><Typography.Text type="secondary">从源码准备到可追溯的分析与问答。</Typography.Text></div><Space wrap><Button icon={<ReloadOutlined />} onClick={() => { void load(); }} loading={loading}>刷新项目</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreateOpen(true); }}>新建项目</Button></Space></div>
    {error && <Alert type="error" showIcon title="项目操作未完成" description={error} style={{ marginBottom: 18 }} />}
    <div className="workspace-grid"><Card title="我的项目" size="small"><div className="workspace-projects">{loading && projects.length === 0 ? <Spin /> : projects.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="创建第一个项目" /> : projects.map((item) => <button type="button" className="workspace-project" aria-current={project?.id === item.id ? 'page' : undefined} key={item.id} onClick={() => navigate(`/workspace/${item.id}`)}><strong>{item.name}</strong><small>项目 {item.id} · {formatTime(item.updated_at).split(' ')[0]}</small></button>)}</div></Card><div>{project ? <ProjectWorkspace key={project.id} project={project} onDeleted={() => { setProjects((items) => items.filter((item) => item.id !== project.id)); navigate('/workspace'); }} /> : <Card><div className="workspace-welcome">{loading ? <Spin size="large" /> : projectId ? <Empty description="项目不存在或不属于当前账号，请从左侧重新选择" /> : <><Typography.Title level={3}>选择一个项目，开始了解源码</Typography.Title><Typography.Paragraph type="secondary">创建或选择项目后，直接使用启动时选择的本地源码目录，扫描、分析和建立问答索引。</Typography.Paragraph><Button type="primary" onClick={() => setCreateOpen(true)}>新建项目</Button></>}</div></Card>}</div></div>
    <Modal title="新建源码项目" open={createOpen} onCancel={() => { if (!creating) setCreateOpen(false); }} footer={null} destroyOnHidden><Form form={form} layout="vertical" onFinish={(values) => { void create(values); }}><Form.Item name="name" label="项目名称" rules={[{ required: true, whitespace: true, message: '请输入项目名称' }]}><Input autoFocus maxLength={160} placeholder="例如：订单服务" disabled={creating} /></Form.Item><Form.Item name="description" label="描述（可选）"><Input.TextArea rows={3} maxLength={2000} disabled={creating} /></Form.Item><Typography.Paragraph type="secondary">项目用于分别保存分析、任务与索引。创建后可直接使用当前本地源码目录。</Typography.Paragraph>{error && <Alert type="error" title={error} style={{ marginBottom: 16 }} />}<Button type="primary" htmlType="submit" loading={creating} block>创建项目</Button></Form></Modal>
  </div>;
}

export default function Workspace() {
  const ownerId = useAuthStore((state) => state.user?.id);
  return ownerId ? <WorkspaceForOwner key={ownerId} /> : null;
}
