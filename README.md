# GoSource-Insight 🚀

这是一个基于 Go 语言开发的 AI 源码分析助手，支持命令行和 Web UI 两种使用方式。

## ✨ 功能特性

- 📂 **源码扫描**：全自动扫描 Go 项目并进行智能切片
- 🧠 **向量检索**：集成 Milvus 向量数据库，支持语义级代码搜索
- 💬 **交互对话**：支持在终端与你的代码进行连续对话
- 🌐 **Web UI界面**：基于 React + Ant Design 的可视化代码分析平台
- 🔒 **隐私保护**：完全基于本地 Ollama 模型，代码不外传
- 📊 **结构化日志**：支持 Text/JSON 格式，灵活的日志级别控制
- 📈 **项目管理**：支持多项目管理，分析历史记录
- 👥 **用户系统**：JWT 认证，用户注册/登录/资料管理
- 🔍 **代码分析**：复杂度分析、安全扫描、Bug 检测

## 🛠️ 技术栈

### 后端
- Go 1.25.5
- Gin (Web 框架)
- GORM (ORM)
- PostgreSQL (数据库)
- Milvus (向量存储)
- Ollama (LLM 引擎)
- LangChainGo (AI 框架)

### 前端
- React 18 + TypeScript
- Vite (构建工具)
- Ant Design 5.x (UI 组件库)
- Zustand (状态管理)
- Monaco Editor (代码编辑器)
- Axios (HTTP 客户端)

## 🚀 快速开始

### 环境要求

- Go 1.25.5+
- Node.js 18+
- PostgreSQL 14+
- WSL2 (Windows 用户)

### 1. 克隆项目

```bash
git clone https://github.com/yourusername/go-ai-study.git
cd go-ai-study
```

### 2. 配置数据库

在 WSL2 中安装并配置 PostgreSQL：

```bash
# 安装 PostgreSQL
sudo apt update
sudo apt install postgresql postgresql-contrib -y

# 启动服务
sudo service postgresql start

# 创建数据库
sudo -u postgres psql -c "CREATE DATABASE goaiinsight;"
sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'your_password';"
```

### 3. 配置后端

编辑 `api/.env` 文件：

```env
PORT=8087
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=goaiinsight
JWT_SECRET=your_jwt_secret_key
```

### 4. 启动后端服务

```bash
cd api
go mod tidy
go run main.go
```

服务将在 http://localhost:8087 启动

### 5. 启动前端

```bash
cd web
npm install
npm run dev
```

访问 http://localhost:3000 使用 Web UI

---

## 🌐 Web UI 功能

### 仪表盘
- 项目统计概览
- 最近分析记录
- 快速开始入口

### 项目管理
- 创建和管理多个分析项目
- 项目详情查看和删除
- 分析历史记录

### 代码分析
- Monaco Editor 代码编辑
- 复杂度分析（圈复杂度、函数统计）
- 安全扫描（漏洞检测）
- Bug 检测
- 可视化分析结果

### 用户系统
- 用户注册和登录
- JWT 认证和授权
- 个人资料管理

## 🔧 后端 API 架构

### API 端点

| 方法 | 端点 | 描述 | 认证 |
|------|------|------|------|
| GET | `/health` | 健康检查 | 否 |
| POST | `/api/v1/users/register` | 用户注册 | 否 |
| POST | `/api/v1/users/login` | 用户登录 | 否 |
| GET | `/api/v1/users/profile` | 获取用户资料 | 是 |
| POST | `/api/v1/projects` | 创建项目 | 是 |
| GET | `/api/v1/projects` | 列出项目 | 是 |
| GET | `/api/v1/projects/:id` | 获取项目详情 | 是 |
| DELETE | `/api/v1/projects/:id` | 删除项目 | 是 |
| POST | `/api/v1/analysis/analyze` | 代码分析 | 是 |
| GET | `/api/v1/analysis/:projectId` | 获取分析结果 | 是 |

### 项目结构

```
go-ai-study/
├── api/                    # API 服务
│   ├── config/            # 配置管理
│   ├── database/          # 数据库连接
│   ├── handlers/          # HTTP 处理器
│   ├── middleware/        # 中间件（JWT、CORS）
│   ├── models/            # 数据模型
│   ├── routes/            # 路由定义
│   └── main.go            # 入口文件
├── web/                    # Web UI
│   ├── src/
│   │   ├── api/          # API 客户端
│   │   ├── components/   # 公共组件
│   │   ├── pages/        # 页面组件
│   │   ├── stores/       # 状态管理
│   │   └── types/        # TypeScript 类型
│   └── package.json
├── cmd/                    # CLI 工具
├── internal/               # 内部包
│   ├── ai/                # AI 引擎
│   ├── tools/             # 分析工具
│   └── cli/               # CLI 实现
└── config/                 # 配置文件
```

## 📋 最近更新

### 2024-04-12
- ✅ 完成 Web UI 前端开发（React + TypeScript + Ant Design）
- ✅ 实现用户注册/登录/资料管理
- ✅ 实现项目 CRUD 功能
- ✅ 集成 Monaco Editor 代码编辑器
- ✅ 实现代码分析结果可视化（复杂度/安全/Bug）
- ✅ 修复 PostgreSQL 数据库连接配置
- ✅ 添加 CORS 跨域支持
- ✅ 修复 List 组件空数据源白屏问题

### 已知问题
- ⚠️ Ant Design 组件 API 弃用警告（不影响功能）
- ⚠️ Monaco Editor 跟踪阻止警告（浏览器隐私设置）

---

## 📝 日志配置

### 命令行参数

```bash
# 日志级别
--log-level <debug|info|warn|error>    # 默认：info

# 日志格式
--log-format <text|json>              # 默认：text

# 日志输出
--log-output <stdout|stderr|file>      # 默认：stdout

# 日志文件路径（当 log-output=file 时使用）
--log-file <path>
```

### 使用示例

```bash
# 使用 JSON 格式日志
go-ai-insight --log-format json analyze ./myproject

# 输出日志到文件
go-ai-insight --log-output file --log-file /var/log/insight.log test ./myproject

# 使用 debug 级别
go-ai-insight --log-level debug security ./myproject

# 组合使用
go-ai-insight --log-format json --log-level debug --log-file debug.json list
```

### 配置文件

在 `~/.go-ai-insight/config.json` 中配置：

```json
{
  "default_output": "stdout",
  "default_format": "text",
  "verbose": false,
  "ollama_endpoint": "http://localhost:11434",
  "milvus_endpoint": "http://localhost:19530",
  "log_config": {
    "level": "info",
    "format": "text",
    "output": "stdout",
    "file_path": ""
  }
}
```

**注意：** 命令行参数优先级 > 配置文件

### 日志级别说明

| 级别 | 说明 | 使用场景 |
|------|------|----------|
| debug | 最详细的日志 | 开发调试 |
| info | 常规信息 | 正常运行 |
| warn | 警告信息 | 潜在问题 |
| error | 错误信息 | 发生错误 |

### 日志输出格式

**Text 格式（默认）：**
```
time=2026-02-06T23:09:24.277+08:00 level=INFO msg=工具注册成功 tool=test_generator enabled=true
```

**JSON 格式：**
```json
{"time":"2026-02-06T23:09:26.94069409+08:00","level":"INFO","msg":"工具注册成功","tool":"test_generator","enabled":true}
```

### 生产环境推荐配置

```json
{
  "log_config": {
    "level": "info",
    "format": "json",
    "output": "file",
    "file_path": "/var/log/go-ai-insight/app.log"
  }
}
```

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发流程

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

### 代码规范

- Go 代码遵循 `gofmt` 格式
- TypeScript 代码使用 ESLint 检查
- 提交前运行测试确保通过

---

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

## 🙏 致谢

- [Gin](https://github.com/gin-gonic/gin) - Web 框架
- [GORM](https://gorm.io/) - ORM 库
- [Ant Design](https://ant.design/) - UI 组件库
- [Monaco Editor](https://microsoft.github.io/monaco-editor/) - 代码编辑器
- [Ollama](https://ollama.ai/) - 本地 LLM 引擎