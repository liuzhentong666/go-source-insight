import { useEffect } from 'react';
import { Card, Row, Col, Statistic, List, Typography, Tag } from 'antd';
import { ProjectOutlined, CodeOutlined, BugOutlined, SafetyOutlined } from '@ant-design/icons';
import { useProjectStore } from '../../stores/projectStore';
import { useAuthStore } from '../../stores/authStore';

const { Title, Text } = Typography;

const Dashboard = () => {
  const { user } = useAuthStore();
  const { projects, fetchProjects } = useProjectStore();

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  const stats = [
    {
      title: '项目数量',
      value: projects.length,
      icon: <ProjectOutlined style={{ color: '#1890ff', fontSize: 24 }} />,
      color: '#e6f7ff',
    },
    {
      title: '分析次数',
      value: 0,
      icon: <CodeOutlined style={{ color: '#52c41a', fontSize: 24 }} />,
      color: '#f6ffed',
    },
    {
      title: '发现问题',
      value: 0,
      icon: <BugOutlined style={{ color: '#faad14', fontSize: 24 }} />,
      color: '#fffbe6',
    },
    {
      title: '安全漏洞',
      value: 0,
      icon: <SafetyOutlined style={{ color: '#f5222d', fontSize: 24 }} />,
      color: '#fff1f0',
    },
  ];

  const recentActivities = [
    { title: '欢迎使用 GoSource Insight', time: '刚刚', type: 'info' },
  ];

  return (
    <div>
      <Title level={2}>欢迎回来，{user?.username || '用户'}！</Title>
      <Text type="secondary">这里是您的代码分析仪表盘</Text>

      <Row gutter={16} style={{ marginTop: 24 }}>
        {stats.map((stat, index) => (
          <Col span={6} key={index}>
            <Card style={{ background: stat.color }}>
              <Statistic
                title={stat.title}
                value={stat.value}
                prefix={stat.icon}
                valueStyle={{ fontSize: 32, fontWeight: 'bold' }}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={16} style={{ marginTop: 24 }}>
        <Col span={12}>
          <Card title="最近项目">
            <List
              dataSource={projects.slice(0, 5)}
              renderItem={(project) => (
                <List.Item>
                  <List.Item.Meta
                    title={project.name}
                    description={project.description || '暂无描述'}
                  />
                  <Tag>{new Date(project.created_at).toLocaleDateString()}</Tag>
                </List.Item>
              )}
              locale={{ emptyText: '暂无项目' }}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="最近活动">
            <List
              dataSource={recentActivities}
              renderItem={(item) => (
                <List.Item>
                  <List.Item.Meta
                    title={item.title}
                    description={item.time}
                  />
                  <Tag color="blue">{item.type}</Tag>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
