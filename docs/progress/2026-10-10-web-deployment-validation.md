# Web 部署层验证记录

本记录覆盖 Web 构建、部署配置、脚本及隔离代理运行层，不代替真实浏览器和模型问答验收。

## 构建

- 2026-10-09 完整多阶段 `web/Dockerfile` 构建最终退出 0，生成 Web `:local` 镜像。2026-10-10 已只读确认该镜像存在，运行用户为 `101:101`；本机镜像身份未写入维护记录。
- 构建上下文排除本机 `node_modules`、`dist`、环境文件和 npm 凭据文件。Linux 构建实际执行标准 `npm ci`，安装 277 个包，再执行 TypeScript 编译及 Vite 构建，转换 3089 个模块。没有复制宿主机已构建的静态包替代完整构建。
- 首次干净安装暴露原锁缺失 Monaco editor 的 peer/传递记录。补锁仅增加四个已有版本的记录，保留其余所有锁条目；修复后的 Linux 干净安装和完整镜像构建均成功。
- 默认 Docker Hub 鉴权入口连接失败；本次通过构建参数覆盖基础镜像分发位置，使用可达公开分发源的官方 Node 24 Alpine、非 root Nginx 1.29 Alpine 镜像完成构建。仓库默认仍为官方镜像名，未修改全局 Docker 配置。默认分发入口在当前网络的可用性尚未验证恢复。
- Vite 提示约 1.13MB 的单个 JavaScript 包大于建议阈值，属于体积警告，构建退出码仍为 0。

## 配置与脚本

以下均已退出 0：

- PowerShell `scripts/deploy_test.ps1`、Bash `scripts/deploy_test.sh`：CLI 默认流程独立、API/Web 模式、既有凭据复用、基础 Web 不启动 AI、不下载模型、可选 AI 只验证缓存，以及健康失败返回非成功。
- CLI 基础 Compose、API/Web 组合 Compose 的 `config --quiet`；组合测试使用临时测试值，不读取或打印正式凭据。
- `scripts/web_config_test.mjs`：开发代理保留 `/api` 路径，使用服务端环境设置，拒绝携带凭据/路径的 origin，以及非法代理等待期限。
- PowerShell 脚本语法、Bash 部署脚本语法及相关文件空白检查。

脚本、配置和 README 中无个人路径、真实账号/凭据或本机临时 API 端口。默认开发代理为通用回环 origin；正式代理使用容器服务名，浏览器始终访问相对 `/api/v1`。

## 隔离运行验证

2026-10-10 使用 `scripts/web_proxy_test.ps1` 和缓存 Node 镜像执行伪 API 验证，最终退出 0。网络及容器采用唯一测试名称，Web 仅绑定动态主机回环端口；测试结束清理自身容器和网络。首次测试发现测试脚本将省略请求体的 GET 误选为 POST，已按显式参数是否存在修正并完成重测。

通过项包括：

- 根页面及 `/workspace/42` 深链返回相同应用页面。
- `/api/v1` 路径、查询参数和 `Authorization` 头原样转发。
- API 404 保持 API 响应，不回退为 SPA。
- API 响应包含 `Cache-Control: no-store`。
- 16384 字节的 JSON 完整转发，16385 字节被代理拒绝为 413。
- Web 容器用户 `101:101`、只读根文件系统及可写 `/tmp`。
- `nginx -t`、镜像健康检查命令及 Docker 实际 `healthy` 状态。
- 非法 `WEB_PROXY_TIMEOUT_SECONDS` 使容器启动失败。

本阶段没有停止、替换或重启正式 API、PostgreSQL、Ollama、Milvus，也没有操作正在使用的问答窗口。最终新增 Web 应使用同一 Compose 项目，仅构建 Web 并执行 `up -d --no-deps --wait web`，由独立浏览器验收负责人统一执行，见 [部署说明](../WEB_DEPLOYMENT.md)。

## 后续独立验收

真实浏览器的认证、项目归属、扫描分析、任务与问答流程，以及最终业务修复后的静态包与正式同源部署验证，由独立验收阶段完成。代理期限默认 330 秒，客户端问答 350 秒；后端 HTTP 写入期限仍为 300 秒，不承诺任意长 HTTP 问答。网页或 API 核心健康不代表 AI 模型在线。

> 后续：真实浏览器与正式同源代理全链路及默认 Docker Hub 拉取可用性已在 [最终验收记录](2026-10-10-final-acceptance.md) 完成。
