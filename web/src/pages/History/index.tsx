import { useEffect, useState } from 'react';
import { Table, Card, Typography, Tag, Select, Button, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useProjectStore } from '../../stores/projectStore';
import { analysisApi } from '../../api/analysis';
import type { Analysis } from '../../types';

const { Title } = Typography;
const { Option } = Select;

const History = () => {
  const { projects, fetchProjects } = useProjectStore();
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [analyses, setAnalyses] = useState<Analysis[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  useEffect(() => {
    if (selectedProject) {
      fetchAnalyses();
    }
  }, [selectedProject]);

  const fetchAnalyses = async () => {
    if (!selectedProject) return;
    
    setLoading(true);
    try {
      const data = await analysisApi.getAnalysisResults(selectedProject);
      setAnalyses(data);
    } catch (error) {
      message.error('获取分析历史失败');
    } finally {
      setLoading(false);
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      pending: { color: 'default', text: '等待中' },
      running: { color: 'processing', text: '分析中' },
      completed: { color: 'success', text: '已完成' },
      failed: { color: 'error', text: '失败' },
    };
    const { color, text } = statusMap[status] || { color: 'default', text: status };
    return <Tag color={color}>{text}</Tag>;
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '项目',
      key: 'project',
      render: (_: any, record: Analysis) => {
        const project = projects.find((p) => p.id === record.project_id);
        return project?.name || '未知项目';
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => getStatusTag(status),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      key: 'updated_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={2} style={{ margin: 0 }}>分析历史</Title>
        <div>
          <Select
            style={{ width: 200, marginRight: 16 }}
            placeholder="选择项目"
            value={selectedProject}
            onChange={setSelectedProject}
            allowClear
          >
            {projects.map((project) => (
              <Option key={project.id} value={project.id}>
                {project.name}
              </Option>
            ))}
          </Select>
          <Button
            icon={<ReloadOutlined />}
            onClick={fetchAnalyses}
            loading={loading}
            disabled={!selectedProject}
          >
            刷新
          </Button>
        </div>
      </div>

      <Card>
        <Table
          columns={columns}
          dataSource={analyses}
          rowKey="id"
          loading={loading}
          locale={{ emptyText: selectedProject ? '暂无分析记录' : '请先选择一个项目' }}
        />
      </Card>
    </div>
  );
};

export default History;
