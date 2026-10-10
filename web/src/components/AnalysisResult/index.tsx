import { Alert, Empty, Space, Table, Tag, Typography } from 'antd';
import type { BugResult, ComplexityResult, SecurityResult } from '../../types';

export const Complexity = ({ data }: { data: ComplexityResult }) => <div className="workspace-stack">
  <Alert title={data.summary || '复杂度规则已执行'} type="info" showIcon />
  <Space wrap><Tag>总函数 {data.statistics?.total_functions ?? data.functions?.length ?? 0}</Tag><Tag>简单（1–10）{data.statistics?.simple_functions ?? '—'}</Tag><Tag>中等（11–20）{data.statistics?.medium_functions ?? '—'}</Tag><Tag>复杂（21–50）{data.statistics?.complex_functions ?? '—'}</Tag><Tag>非常复杂（&gt;50）{data.statistics?.very_complex_functions ?? '—'}</Tag></Space>
  <Table dataSource={data.functions || []} rowKey={(row) => `${row.name}:${row.line}`} size="small" pagination={{ pageSize: 12 }} scroll={{ x: 650 }} columns={[
    { title: '函数', dataIndex: 'name' },
    { title: '起始行', dataIndex: 'line', width: 85 },
    { title: '圈复杂度', dataIndex: 'complexity', width: 100, render: (value: number) => <Tag color={value > 20 ? 'error' : value > 10 ? 'warning' : 'default'}>{value}</Tag> },
    { title: '行数', dataIndex: 'lines', width: 75 },
    { title: '规则提示', dataIndex: 'issues', render: (issues: string[] | null) => issues?.length ? issues.map((issue, index) => <Tag key={index} color="warning">{issue}</Tag>) : '无' },
  ]} locale={{ emptyText: '规则已执行，没有函数结果' }} />
</div>;

const severityColor: Record<string, string> = { Critical: 'red', High: 'orange', Medium: 'gold', Low: 'blue' };

export const Security = ({ data }: { data: SecurityResult }) => <div className="workspace-stack">
  <Alert title={data.summary || `安全规则已执行，返回 ${data.total} 条提示`} type={data.total ? 'warning' : 'success'} showIcon />
  {(data.issues || []).length === 0 ? <Empty description="本次安全规则没有返回提示" /> : data.issues.map((issue, index) => <div key={`${issue.id}:${index}`}>
    <Space wrap><Tag color={severityColor[issue.severity]}>{issue.severity}</Tag><Typography.Text strong>{issue.description}</Typography.Text></Space>
    <p className="workspace-help">规则 {issue.rule_id} · 第 {issue.line} 行 · {issue.category}</p>
    {issue.code_snippet && <pre className="workspace-code">{issue.code_snippet}</pre>}
    {issue.suggestion && <Typography.Paragraph>建议：{issue.suggestion}</Typography.Paragraph>}
  </div>)}
</div>;

export const Bugs = ({ data }: { data: BugResult }) => <div className="workspace-stack">
  <Alert title={data.status === 'error' ? '缺陷规则执行失败' : `${data.status === 'partial' ? '部分文件的缺陷规则已执行' : '缺陷规则已执行'}，返回 ${data.total} 条待复核候选`} type={data.status === 'error' ? 'error' : data.status === 'partial' || data.total ? 'warning' : 'success'} showIcon description="静态候选需要人工复核。低置信度提示仅代表可能风险，并非已确认 Bug。" />
  {(data.bugs || []).length === 0 ? <Empty description={data.status === 'error' ? '规则执行失败，没有可用结果' : '本次缺陷规则没有返回候选'} /> : data.bugs.map((issue, index) => <div key={`${issue.id}:${index}`}>
    <Space wrap><Tag color={severityColor[issue.severity]}>{issue.severity}</Tag><Tag color={issue.confidence === 'low' ? 'warning' : 'default'}>{issue.confidence === 'low' ? '低置信度 · 待复核' : issue.confidence === 'medium' ? '中置信度 · 待复核' : issue.confidence === 'high' ? '高置信度 · 待复核' : '待复核候选'}</Tag><Typography.Text strong>{issue.description}</Typography.Text></Space>
    <p className="workspace-help">规则 {issue.rule_id} · 第 {issue.line} 行 · {issue.category}</p>
    {issue.code_snippet && <pre className="workspace-code">{issue.code_snippet}</pre>}
    {issue.fix_suggestion && <><Typography.Text>修复建议</Typography.Text><pre className="workspace-code">{issue.fix_suggestion}</pre></>}
  </div>)}
</div>;
