import { Alert, Button, Empty, Space, Table, Tag, Typography } from 'antd';
import type { Job } from '../../types';
import { isJobActive } from '../../hooks/useProjectJobs';
import { failureHint, formatTime, kindLabels, statusLabels } from '../../pages/Workspace/model';

export default function TaskList({ jobs, loading, error, refresh, cancel }: {
  jobs: Job[]; loading: boolean; error: string | null; refresh: () => void; cancel: (id: string) => Promise<void>;
}) {
  return <div className="workspace-stack">
    <div className="workspace-header"><Typography.Text type="secondary">仅显示本浏览器记住的当前账号、当前项目任务；刷新后恢复已知任务，不代表服务端完整历史。</Typography.Text><Button onClick={refresh} loading={loading}>刷新状态</Button></div>
    {error && <Alert type="error" showIcon title="任务状态暂未更新" description={error} />}
    <Table<Job> size="small" dataSource={jobs} rowKey="job_id" loading={loading} scroll={{ x: 680 }} pagination={{ pageSize: 10 }} locale={{ emptyText: <Empty description="尚未在本浏览器提交或记住此项目的任务" /> }} columns={[
      { title: '任务', key: 'kind', render: (_, job) => <Space orientation="vertical" size={2}><Typography.Text strong>{kindLabels[job.kind] || job.kind}</Typography.Text><Typography.Text type="secondary" className="workspace-path">{job.target}</Typography.Text></Space> },
      { title: '状态', dataIndex: 'status', render: (status: string, job) => <Tag color={job.recovering ? 'processing' : statusLabels[status]?.color}>{job.recovering ? '恢复状态中' : statusLabels[status]?.text || status}</Tag> },
      { title: '提交时间', dataIndex: 'created_at', render: (value: string) => formatTime(value) },
      { title: '操作', key: 'action', render: (_, job) => job.recovering ? <Typography.Text type="secondary">正在查询</Typography.Text> : isJobActive(job.status) ? <Button size="small" danger disabled={job.status === 'canceling'} onClick={() => { void cancel(job.job_id).catch(() => undefined); }}>{job.status === 'canceling' ? '等待确认' : '取消任务'}</Button> : <Typography.Text type="secondary">已结束</Typography.Text> },
    ]} expandable={{ expandedRowRender: (job) => <div className="workspace-task-detail workspace-stack">
      <Typography.Text copyable className="workspace-path">任务 ID：{job.job_id}</Typography.Text>
      <Typography.Text type="secondary">开始：{formatTime(job.started_at)} · 结束：{formatTime(job.finished_at)}</Typography.Text>
      {job.failure_code && <Alert type="error" showIcon title={job.failure_reason || '任务失败'} description={<>{job.failure_code}{failureHint(job.failure_code) && <p>{failureHint(job.failure_code)}</p>}</>} />}
      {job.status === 'canceling' && <Alert type="warning" title="取消请求已提交，任务仍可能正在释放资源；最终状态以服务端为准。" />}
      {job.kind === 'index' && job.status === 'completed' && job.result && <Typography.Text>索引快照已发布。问答将使用已发布的项目快照。</Typography.Text>}
    </div> }} />
  </div>;
}
