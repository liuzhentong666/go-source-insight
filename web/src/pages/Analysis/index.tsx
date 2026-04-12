import { useState, useEffect } from 'react';
import { Row, Col, Card, Button, Select, message, Typography, Spin, Tabs } from 'antd';
import Editor from '@monaco-editor/react';
import { useProjectStore } from '../../stores/projectStore';
import { analysisApi } from '../../api/analysis';
import AnalysisResult from '../../components/AnalysisResult';
import type { AnalysisResult as AnalysisResultType } from '../../types';

const { Title } = Typography;
const { Option } = Select;

const defaultCode = `package main

import (
	"fmt"
	"os"
)

func main() {
	// 示例代码
	file, err := os.Open("test.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	
	fmt.Println("Hello, World!")
}

func complexFunction(x int) int {
	if x > 0 {
		if x > 10 {
			if x > 100 {
				return x * 100
			}
			return x * 10
		}
		return x
	}
	return 0
}`;

const Analysis = () => {
  const { projects, fetchProjects } = useProjectStore();
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [code, setCode] = useState(defaultCode);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<AnalysisResultType | null>(null);
  const [activeTab, setActiveTab] = useState('complexity');

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  const handleAnalyze = async () => {
    if (!selectedProject) {
      message.warning('请先选择一个项目');
      return;
    }

    setLoading(true);
    setResult(null);

    try {
      await analysisApi.analyzeCode({
        project_id: selectedProject,
        code: code,
      });
      
      message.success('分析任务已提交，请稍后查看结果');
      
      // 模拟获取结果（实际应该轮询查询）
      setTimeout(async () => {
        try {
          const analyses = await analysisApi.getAnalysisResults(selectedProject);
          if (analyses.length > 0 && analyses[0].status === 'completed') {
            const analysisData = JSON.parse(analyses[0].result);
            setResult(analysisData);
          }
        } catch (error) {
          console.error('获取分析结果失败', error);
        }
        setLoading(false);
      }, 3000);
    } catch (error: any) {
      message.error(error.response?.data?.error || '分析失败');
      setLoading(false);
    }
  };

  return (
    <div>
      <Title level={2}>代码分析</Title>

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={12}>
          <Select
            style={{ width: '100%' }}
            placeholder="选择项目"
            value={selectedProject}
            onChange={setSelectedProject}
          >
            {projects.map((project) => (
              <Option key={project.id} value={project.id}>
                {project.name}
              </Option>
            ))}
          </Select>
        </Col>
        <Col span={12} style={{ textAlign: 'right' }}>
          <Button
            type="primary"
            onClick={handleAnalyze}
            loading={loading}
            disabled={!selectedProject}
          >
            开始分析
          </Button>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="代码编辑器" bodyStyle={{ padding: 0 }}>
            <Editor
              height="500px"
              defaultLanguage="go"
              value={code}
              onChange={(value) => setCode(value || '')}
              theme="vs-dark"
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                lineNumbers: 'on',
                roundedSelection: false,
                scrollBeyondLastLine: false,
                automaticLayout: true,
              }}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card 
            title="分析结果" 
            style={{ height: '100%' }}
            bodyStyle={{ height: 'calc(100% - 57px)', overflow: 'auto' }}
          >
            {loading ? (
              <div style={{ textAlign: 'center', padding: '100px 0' }}>
                <Spin size="large" />
                <p style={{ marginTop: 16 }}>正在分析中...</p>
              </div>
            ) : result ? (
              <Tabs activeKey={activeTab} onChange={setActiveTab}>
                <Tabs.TabPane tab="复杂度" key="complexity">
                  <AnalysisResult.Complexity data={result.complexity} />
                </Tabs.TabPane>
                <Tabs.TabPane tab="安全" key="security">
                  <AnalysisResult.Security data={result.security} />
                </Tabs.TabPane>
                <Tabs.TabPane tab="Bug" key="bugs">
                  <AnalysisResult.Bugs data={result.bugs} />
                </Tabs.TabPane>
              </Tabs>
            ) : (
              <div style={{ textAlign: 'center', padding: '100px 0', color: '#999' }}>
                点击"开始分析"查看结果
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Analysis;
