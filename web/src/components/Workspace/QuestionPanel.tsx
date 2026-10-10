import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Collapse, Empty, Input, Space, Typography } from 'antd';
import { workspaceApi } from '../../api/workspace';
import type { AnswerResult } from '../../types';
import { relativePathProblem } from '../../pages/Workspace/model';
import { getApiErrorMessage } from '../../api/client';

interface Exchange { id: number; question: string; response?: AnswerResult; error?: string }

export default function QuestionPanel({ projectId, indexing, onWaitingChange }: { projectId: number; indexing: boolean; onWaitingChange: (waiting: boolean) => void }) {
  const [question, setQuestion] = useState('');
  const [file, setFile] = useState('');
  const [exchanges, setExchanges] = useState<Exchange[]>([]);
  const [waiting, setWaiting] = useState(false);
  const [stopped, setStopped] = useState(false);
  const request = useRef<AbortController | null>(null);
  const sequence = useRef(0);
  useEffect(() => () => { sequence.current += 1; request.current?.abort(); }, []);

  const send = async () => {
    if (!question.trim() || waiting || request.current || indexing || relativePathProblem(file, false)) return;
    const id = ++sequence.current;
    const controller = new AbortController();
    request.current = controller;
    const text = question.trim();
    setWaiting(true);
    onWaitingChange(true);
    setStopped(false);
    setExchanges((items) => [...items, { id, question: text }]);
    setQuestion('');
    try {
      const response = await workspaceApi.ask(projectId, { question: text, ...(file.trim() ? { file: file.trim() } : {}) }, controller.signal);
      if (sequence.current !== id || controller.signal.aborted) return;
      setExchanges((items) => items.map((item) => item.id === id ? { ...item, response } : item));
    } catch (error) {
      if (sequence.current !== id || controller.signal.aborted) return;
      setExchanges((items) => items.map((item) => item.id === id ? { ...item, error: getApiErrorMessage(error) } : item));
    } finally {
      if (sequence.current === id) { setWaiting(false); onWaitingChange(false); request.current = null; }
    }
  };

  const stopWaiting = () => {
    const id = sequence.current;
    sequence.current += 1;
    request.current?.abort();
    request.current = null;
    setWaiting(false);
    onWaitingChange(false);
    setStopped(true);
    setExchanges((items) => items.map((item) => item.id === id ? { ...item, error: '已取消本次等待。服务端是否结束未获确认，稍后操作仍可能返回项目忙。' } : item));
  };

  return <div className="workspace-stack">
    <Alert type="info" showIcon title="按当前问题检索项目索引" description="每次请求独立检索，没有服务端多轮记忆。下方记录仅是当前页面会话，不会作为模型上下文；离开项目后清空。扫描和分析不会自动建立索引。" />
    {indexing && <Alert type="warning" title="此项目正在建立索引，请等待任务终止后问答。" />}
    {stopped && <Alert type="warning" showIcon title="已停止等待响应" description="取消已发出，服务端的最终终止状态未确认。" />}
    <label className="workspace-scope"><Typography.Text>限定来源文件（可选）</Typography.Text><Input placeholder="例如 internal/service.go" value={file} onChange={(event) => setFile(event.target.value)} disabled={waiting} status={relativePathProblem(file, false) ? 'error' : undefined} />{relativePathProblem(file, false) && <Typography.Text type="danger">{relativePathProblem(file, false)}</Typography.Text>}</label>
    <label><Typography.Text>源码问题</Typography.Text><Input.TextArea rows={4} maxLength={4096} showCount placeholder="例如：请求取消后，资源在哪里释放？" value={question} onChange={(event) => setQuestion(event.target.value)} disabled={waiting} /></label>
    <Space wrap><Button type="primary" loading={waiting} disabled={indexing || !question.trim() || Boolean(relativePathProblem(file, false))} onClick={() => { void send(); }}>发送问题</Button>{waiting && <Button onClick={stopWaiting}>停止等待</Button>}{exchanges.length > 0 && <Button disabled={waiting} onClick={() => setExchanges([])}>清空会话</Button>}</Space>
    {exchanges.length === 0 ? <Empty description="已有项目索引可直接提问；本浏览器没有任务记录，不代表未建立索引" /> : [...exchanges].reverse().map((exchange) => <Card key={exchange.id} size="small">
      <div className="workspace-question">{exchange.question}</div>
      {exchange.error && <Alert type="error" showIcon title="本次未获得回答" description={exchange.error} />}
      {exchange.response ? <div className="workspace-stack">
        {exchange.response.answer ? <div className="workspace-answer">{exchange.response.answer}</div> : <Alert type="info" title="没有匹配的索引内容，未生成回答" description="请检查已发布索引和来源文件范围；空结果不代表源码不存在。" />}
        {(exchange.response.sources || []).length > 0 ? <Collapse items={exchange.response.sources.map((source, index) => ({ key: `${source.id}:${index}`, label: <Typography.Text className="workspace-path">{source.source} · 第 {source.start_line}–{source.end_line} 行</Typography.Text>, children: <><Typography.Text type="secondary">检索来源片段 · 物理行号</Typography.Text><pre className="workspace-code">{source.content}</pre></> }))} /> : <Typography.Text type="secondary">本次来源：0 条</Typography.Text>}
      </div> : !exchange.error && <Typography.Text type="secondary">等待真实响应…</Typography.Text>}
    </Card>)}
  </div>;
}
