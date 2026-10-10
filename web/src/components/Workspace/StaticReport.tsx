import { Alert, Collapse, Empty, Space, Statistic, Tabs, Tag } from 'antd';
import type { StaticReport } from '../../types';
import { Complexity, Security, Bugs } from '../AnalysisResult';

export default function StaticReportView({ report }: { report: StaticReport }) {
  return <div className="workspace-stack">
    <Alert type="info" showIcon title={`分析范围：${report.target_path} · 测试文件不参与静态分析`} description="这是静态规则结果。候选问题需要结合调用路径与业务语义复核，不能直接视为已确认缺陷。" />
    <div className="workspace-metrics">
      <Statistic title="已分析文件" value={report.files_analyzed} />
      <Statistic title="函数" value={report.functions_analyzed} />
      <Statistic title="复杂度提示" value={report.complexity_issues} />
      <Statistic title="安全提示" value={report.security_issues} />
      <Statistic title="缺陷候选" value={report.bug_issues} />
    </div>
    {(report.files || []).length === 0 ? <Empty description="本次范围没有可分析的生产 Go 文件" /> : <Collapse items={report.files.map((file) => ({
      key: file.path,
      label: <Space wrap><span className="workspace-path">{file.path}</span><Tag>{file.functions} 个函数</Tag><Tag color={file.security_issues ? 'warning' : 'default'}>安全 {file.security_issues}</Tag><Tag>缺陷候选 {file.bug_issues}</Tag></Space>,
      children: <Tabs items={[
        { key: 'complexity', label: '复杂度', children: file.complexity ? <Complexity data={file.complexity} /> : <Empty description="该文件没有复杂度结果" /> },
        { key: 'security', label: '安全规则', children: file.security ? <Security data={file.security} /> : <Empty description="该文件没有安全规则结果" /> },
        { key: 'bugs', label: '缺陷候选', children: file.bugs ? <Bugs data={file.bugs} /> : <Empty description="该文件没有缺陷规则结果" /> },
      ]} />,
    }))} />}
  </div>;
}
