import { Card, List, Tag, Progress, Table, Typography, Alert } from 'antd';
import type { ComplexityResult, SecurityResult, BugResult } from '../../types';

const { Title, Text } = Typography;

// 复杂度分析结果
const Complexity = ({ data }: { data: ComplexityResult }) => {
  const columns = [
    {
      title: '函数名',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '圈复杂度',
      dataIndex: 'complexity',
      key: 'complexity',
      render: (value: number) => (
        <Progress
          percent={Math.min(value * 5, 100)}
          size="small"
          status={value > 10 ? 'exception' : value > 5 ? 'normal' : 'success'}
          format={() => value}
        />
      ),
    },
    {
      title: '行数',
      dataIndex: 'lines',
      key: 'lines',
    },
    {
      title: '问题',
      dataIndex: 'issues',
      key: 'issues',
      render: (issues: string[]) =>
        issues?.map((issue, idx) => (
          <Tag key={idx} color="warning">
            {issue}
          </Tag>
        )) || '-',
    },
  ];

  return (
    <div>
      <Alert
        message={data.summary}
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Card size="small" title="统计信息" style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-around' }}>
          <div style={{ textAlign: 'center' }}>
            <Title level={4}>{data.statistics.total_functions}</Title>
            <Text type="secondary">总函数数</Text>
          </div>
          <div style={{ textAlign: 'center' }}>
            <Title level={4} style={{ color: '#52c41a' }}>
              {data.statistics.simple_functions}
            </Title>
            <Text type="secondary">简单函数</Text>
          </div>
          <div style={{ textAlign: 'center' }}>
            <Title level={4} style={{ color: '#faad14' }}>
              {data.statistics.medium_functions}
            </Title>
            <Text type="secondary">中等复杂度</Text>
          </div>
          <div style={{ textAlign: 'center' }}>
            <Title level={4} style={{ color: '#f5222d' }}>
              {data.statistics.complex_functions + data.statistics.very_complex_functions}
            </Title>
            <Text type="secondary">复杂函数</Text>
          </div>
        </div>
      </Card>
      <Table
        columns={columns}
        dataSource={data.functions}
        rowKey="name"
        size="small"
        pagination={false}
      />
    </div>
  );
};

// 安全扫描结果
const Security = ({ data }: { data: SecurityResult }) => {
  const severityColors: Record<string, string> = {
    Critical: 'red',
    High: 'orange',
    Medium: 'yellow',
    Low: 'blue',
  };

  return (
    <div>
      <Alert
        message={data.summary}
        type={data.total > 0 ? 'warning' : 'success'}
        showIcon
        style={{ marginBottom: 16 }}
      />
      {data.total > 0 && (
        <Card size="small" title="统计" style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', gap: 16 }}>
            {data.statistics.critical > 0 && (
              <Tag color="red">严重: {data.statistics.critical}</Tag>
            )}
            {data.statistics.high > 0 && (
              <Tag color="orange">高危: {data.statistics.high}</Tag>
            )}
            {data.statistics.medium > 0 && (
              <Tag color="yellow">中危: {data.statistics.medium}</Tag>
            )}
            {data.statistics.low > 0 && (
              <Tag color="blue">低危: {data.statistics.low}</Tag>
            )}
          </div>
        </Card>
      )}
      <List
        dataSource={data.issues || []}
        renderItem={(item) => (
          <List.Item>
            <Card
              size="small"
              style={{ width: '100%' }}
              title={
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>{item.description}</span>
                  <Tag color={severityColors[item.severity]}>{item.severity}</Tag>
                </div>
              }
            >
              <p>
                <Text type="secondary">规则: {item.rule_id}</Text>
              </p>
              <p>
                <Text type="secondary">位置: 第 {item.line} 行</Text>
              </p>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                }}
              >
                {item.code_snippet}
              </pre>
              <p>
                <Text type="secondary">建议: {item.suggestion}</Text>
              </p>
            </Card>
          </List.Item>
        )}
        locale={{ emptyText: '未发现安全问题' }}
      />
    </div>
  );
};

// Bug 检测结果
const Bugs = ({ data }: { data: BugResult }) => {
  const severityColors: Record<string, string> = {
    High: 'red',
    Medium: 'orange',
    Low: 'blue',
  };

  return (
    <div>
      <Alert
        message={data.summary}
        type={data.total > 0 ? 'warning' : 'success'}
        showIcon
        style={{ marginBottom: 16 }}
      />
      {data.total > 0 && (
        <Card size="small" title="统计" style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', gap: 16 }}>
            {data.statistics.high > 0 && (
              <Tag color="red">高危: {data.statistics.high}</Tag>
            )}
            {data.statistics.medium > 0 && (
              <Tag color="orange">中危: {data.statistics.medium}</Tag>
            )}
            {data.statistics.low > 0 && (
              <Tag color="blue">低危: {data.statistics.low}</Tag>
            )}
          </div>
        </Card>
      )}
      <List
        dataSource={data.bugs || []}
        renderItem={(item) => (
          <List.Item>
            <Card
              size="small"
              style={{ width: '100%' }}
              title={
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>{item.description}</span>
                  <Tag color={severityColors[item.severity]}>{item.severity}</Tag>
                </div>
              }
            >
              <p>
                <Text type="secondary">类别: {item.category}</Text>
              </p>
              <p>
                <Text type="secondary">位置: 第 {item.line} 行</Text>
              </p>
              <pre
                style={{
                  background: '#f5f5f5',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                }}
              >
                {item.code_snippet}
              </pre>
              <p>
                <Text type="secondary">修复建议:</Text>
              </p>
              <pre
                style={{
                  background: '#f6ffed',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                  border: '1px solid #b7eb8f',
                }}
              >
                {item.fix_suggestion}
              </pre>
            </Card>
          </List.Item>
        )}
        locale={{ emptyText: '未发现 Bug' }}
      />
    </div>
  );
};

export default { Complexity, Security, Bugs };
