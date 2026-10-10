import { Layout as AntLayout, Menu, Avatar, Dropdown, Button, Space } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  ProjectOutlined,
  CodeOutlined,
  HistoryOutlined,
  UserOutlined,
  LogoutOutlined,
  MenuOutlined,
} from '@ant-design/icons';
import { useAuthStore } from '../../stores/authStore';
import './layout.css';

const { Header, Sider, Content } = AntLayout;

const Layout = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuthStore();

  const menuItems = [
    { key: '/workspace', icon: <CodeOutlined />, label: '项目工作台' },
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: '仪表盘',
    },
    {
      key: '/projects',
      icon: <ProjectOutlined />,
      label: '项目管理',
    },
    {
      key: '/analysis',
      icon: <CodeOutlined />,
      label: '代码分析',
    },
    {
      key: '/history',
      icon: <HistoryOutlined />,
      label: '分析历史',
    },
  ];

  const userMenuItems = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      danger: true,
    },
  ];

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key);
  };

  const handleUserMenuClick = ({ key }: { key: string }) => {
    if (key === 'logout') {
      logout();
      navigate('/login');
    }
  };

  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider theme="light" width={200} breakpoint="md" collapsedWidth={0} trigger={<Button type="text" icon={<MenuOutlined />} aria-label="展开或收起导航" />}>
        <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', borderBottom: '1px solid #f0f0f0' }}>
          <h2 style={{ margin: 0, color: '#1890ff' }}>GoSource</h2>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[location.pathname.startsWith('/workspace') ? '/workspace' : location.pathname]}
          items={menuItems}
          onClick={handleMenuClick}
          style={{ borderRight: 0 }}
        />
      </Sider>
      <AntLayout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', justifyContent: 'flex-end', alignItems: 'center', boxShadow: '0 1px 4px rgba(0,0,0,0.1)' }}>
          <Dropdown menu={{ items: userMenuItems, onClick: handleUserMenuClick }} placement="bottomRight">
            <Button type="text" aria-label="账号菜单" className="app-account-menu">
            <Space>
              <Avatar icon={<UserOutlined />} />
              <span>{user?.username || '用户'}</span>
            </Space>
            </Button>
          </Dropdown>
        </Header>
        <Content className="app-content" style={{ background: '#fff', borderRadius: 8, minHeight: 280 }}>
          <Outlet />
        </Content>
      </AntLayout>
    </AntLayout>
  );
};

export default Layout;
