import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, Spin } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import Layout from './components/Layout';
import Login from './pages/Login';
import Register from './pages/Register';
import Dashboard from './pages/Dashboard';
import Projects from './pages/Projects';
import Analysis from './pages/Analysis';
import History from './pages/History';
import Workspace from './pages/Workspace';
import { useAuthStore } from './stores/authStore';

// 受保护的路由组件
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated, initialized, initialize, user } = useAuthStore();

  useEffect(() => {
    void initialize();
  }, [initialize]);

  if (!initialized) return <div style={{ padding: 64, textAlign: 'center' }}><Spin tip="正在验证会话" /></div>;
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <div key={user?.id}>{children}</div>;
};

// 公开路由组件（已登录用户重定向到首页）
const PublicRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated, initialized, initialize } = useAuthStore();

  useEffect(() => {
    void initialize();
  }, [initialize]);

  if (!initialized) return <div style={{ padding: 64, textAlign: 'center' }}><Spin tip="正在验证会话" /></div>;
  if (isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
};

function App() {
  return (
    <ConfigProvider locale={zhCN}>
      <BrowserRouter>
        <Routes>
          {/* 公开路由 */}
          <Route
            path="/login"
            element={
              <PublicRoute>
                <Login />
              </PublicRoute>
            }
          />
          <Route
            path="/register"
            element={
              <PublicRoute>
                <Register />
              </PublicRoute>
            }
          />

          {/* 受保护的路由 */}
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout />
              </ProtectedRoute>
            }
          >
            <Route index element={<Dashboard />} />
            <Route path="projects" element={<Projects />} />
            <Route path="workspace" element={<Workspace />} />
            <Route path="workspace/:projectId" element={<Workspace />} />
            <Route path="analysis" element={<Analysis />} />
            <Route path="history" element={<History />} />
          </Route>

          {/* 默认重定向 */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}

export default App;
