# GoSource-Insight Web 工作台

React 19、TypeScript、Vite 和 Ant Design 工作台复用现有 API：注册登录、项目管理、源码扫描、静态分析、异步索引、Ollama 源码问答。浏览器始终访问相对 `/api/v1`，正式部署由 Nginx 同源反代，不需要打开后端 CORS。

## 本机一键部署

需要 Docker Compose V2、运行中的 Linux 容器引擎及与 `go.mod` 兼容的宿主 Go 工具链（Bash 还需 `curl`、`openssl` 及常见系统工具 `ps`、`find`、`cksum`），在仓库根目录执行：

```powershell
./scripts/deploy.ps1 -Mode web
# 有缓存模型时启动并检查 AI，不会下载模型：
./scripts/deploy.ps1 -Mode web -WithAI
```

```bash
bash scripts/deploy.sh --mode web
bash scripts/deploy.sh --mode web --with-ai
```

默认工作台为 `http://127.0.0.1:3000`。`WEB_PORT` 可修改发布端口；如果本地环境文件已有覆盖，以 Compose 实际显示的地址为准。仅绑定本机回环地址，数据库和 AI 服务不公开。静态操作无需 AI；Web 就绪和 API `/ready` 仅说明网页及核心服务可用，不说明模型在线。

部署沿用 `.runtime/api.env` 中的服务凭据和已有 PostgreSQL 命名卷。首次才生成随机凭据，重复部署不会覆盖文件或修改数据库密码。也可同时从环境注入 `API_DB_PASSWORD` 和 `API_JWT_SECRET`。不得把服务密钥放入 `VITE_` 变量、静态文件或浏览器。CLI/API 模式仍独立，不需要 Web 配置或 Node 环境；Web 构建使用独立 Docker 上下文 `web/`。

首次准备 AI 模型仍可运行原 CLI 部署入口；该入口会下载缺失模型。Web 的 `WithAI` 只验证已有缓存，缺失时失败。详细凭据、运行及隔离验证方法见 [部署说明](../docs/WEB_DEPLOYMENT.md) 与 [API 说明](../api/README.md)。

## 工作台流程

1. 在网页选择自己的账号注册并登录，没有预置账户。
2. 打开 `/workspace`，使用「新建项目」填写名称和说明，进入 `/workspace/:projectId`。
3. 本机部署执行 `./scripts/deploy.ps1 -Mode web` 或 `bash scripts/deploy.sh --mode web`，入口自动准备宿主源码读取服务。默认 host 模式直接输入任意本机可读目录或 Go 文件完整路径，无需预选固定根或填写所有者/项目 ID；显式 mapped 模式保留旧挂载入口。详见 [本地源码读取](../docs/SOURCE_BINDINGS.md)。
4. host 模式在操作范围填写任意本机可读目录或 Go 文件完整路径；mapped 模式填写显示根或其子路径。点击「扫描源码」后，扫描包含生产及测试文件，静态分析只检查生产 Go 文件。源码页显示相对扫描信息，问答来源可核对原文和物理行号。
5. 点击「静态分析」，在「任务记录」查看排队、运行与最终状态，在「分析结果」阅读复杂度、安全和 Bug 规则结果。规则输出是需要人工复核的线索。
6. AI 服务和缓存模型可用后，点击「建立索引」，确认替换整个项目索引，等待任务完成，再在「源码问答」点击「发送问题」。可填项目内相对文件过滤，空结果或低置信度应核对已索引范围与来源。

「任务记录」只记住本浏览器中的当前账号和项目提交过的任务 ID，并查询服务端状态；它不是服务端完整任务历史。旧分析历史入口保留兼容报告。问答是普通 HTTP 请求，当前没有流式输出或持久会话历史。

「停止等待」只取消浏览器等待，不代表已确认服务端操作终止；重新提交前留意项目忙状态。本浏览器没有索引记录时仍可尝试问答，因为此前可能通过 API 或其他客户端发布过索引。

完整路径属于运行读取服务的本机，不是远程浏览器的磁盘。host source-info 可无显示根，可用性表示读取服务状态；具体目标的存在性、权限及安全性由服务端检查。mapped 模式仍限制配置根。路径或排除配置变化不自动更新旧索引；问答文件过滤和来源保持索引内相对路径。

取消排队任务会释放容量，运行任务进入 `canceling` 后等待资源关闭。已确认发布的索引不会回滚；`interrupted` 与 `index_publication_unknown` 可能已经发布快照，需要先核对索引再重试。同项目索引和问答互斥，忙时提示稍后再试。索引子目录也会替换整个项目快照，索引 `pkg` 后不会同时保留其他目录；重新索引 `.` 恢复整项目范围。只支持一个 API 实例，详情见 [接口契约](../api/CONTRACT.md)。

## 前端开发

在 `web/` 目录执行：

```bash
npm ci
npm run dev
npm run build
npm run lint
```

开发服务仅监听 `127.0.0.1:3000`，`/api` 原样代理到默认 `http://127.0.0.1:8087`；若 API 用 `API_PORT` 改了端口，`WEB_API_PROXY_TARGET` 需同步。需要自定义时复制 `.env.example` 为已忽略的 `.env.local`，设置 `WEB_API_PROXY_TARGET` 为没有用户名、密码或路径的 HTTP(S) origin。此设置只由开发服务读取，不进入前端包。开发时先独立启动 API 并准备源码。

`WEB_PROXY_TIMEOUT_SECONDS` 默认 330 秒，范围 30–3600，适用于 Vite 开发代理和部署代理。依据为默认 AI 操作期限 300 秒加 30 秒余量；Nginx 上游连接期限为 10 秒。普通客户端请求期限为 60 秒，问答为 350 秒，页面提供中止操作。后端 HTTP 写入期限当前固定为 300 秒，即使将 AI 配置提高到更长也受此限制；调高代理并不扩展后端或浏览器期限。索引/分析提交立即返回任务 ID，任务期限不等于单次 HTTP 等待期限。当前部署不承诺任意长问答。

依赖说明：`axios` 1.20.0、`react-router-dom` 7.18.4 为依赖复审后的版本。`echarts`、`@monaco-editor/react` 仍在 `package.json` 中但 `src` 未导入，相关审计条目见 [最终验收记录](../docs/progress/2026-10-10-final-acceptance.md)。
