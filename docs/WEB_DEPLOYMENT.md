# Web 本机部署和运维

## 启动与配置

私人本机常用入口为 `./scripts/deploy.ps1 -Mode web` 或 `bash scripts/deploy.sh --mode web`。默认 host 模式要求宿主有与 `go.mod` 兼容的 Go 工具链，Bash 还需 `curl`、`openssl` 及常见系统工具 `ps`、`find`、`cksum`；入口自动准备宿主读取服务，网页可直接输入任意本机可读目录/Go 文件完整路径，无需预选源码根。显式 `SOURCE_ACCESS_MODE=mapped` 时，`-SourceDirectory` / `--source-directory` 保留旧只读挂载流程。首次部署构建 API/Web、启动 PostgreSQL 并等待健康；已有同项目 API 时仅通过 `--no-deps` 更新 API/Web，保留运行中的数据库和模型；核心健康不证明读取服务或 AI 的具体操作成功。已有模型缓存时添加 `-WithAI` / `--with-ai`，不下载缺失模型。

脚本默认复用已忽略的 `.runtime/api.env`。自选凭据文件使用 `-ApiEnvFile PATH` / `--env-file PATH`，文件必须与已存在的 PostgreSQL 卷凭据一致。首次生成凭据，重跑保留文件；缺失、弱或不匹配的既有配置应先核对，禁止通过删除卷或凭据文件解决。环境注入同时提供 `API_DB_PASSWORD`、`API_JWT_SECRET` 时可以不创建文件。不要打印完整 Compose 配置或容器环境。

| 设置 | 默认 | 用途 |
| --- | --- | --- |
| `WEB_PORT` | `3000` | 主机回环地址上的 Web 端口 |
| `API_PORT` | `8087` | 主机回环地址上的独立 API 端口；默认端口被占用时改为本机空闲端口，否则 API 启动失败 |
| `API_SOURCE_DIR` | `./sources` | 已有受控源码根，只读挂载 |
| `WEB_PROXY_TIMEOUT_SECONDS` | `330` | Web/Vite 代理等待期限，30–3600 秒 |
| `WEB_API_PROXY_TARGET` | `http://127.0.0.1:8087` | 仅 Vite 开发服务的上游 origin |
| `GO_AI_INSIGHT_TIMEOUT_SECONDS` | `300` | 后端完整模型操作期限 |

设置本地端口或源码根可通过调用者环境或未提交的凭据环境文件。Vite 开发设置放在 `web/.env.local`，不得使用 `VITE_` 前缀传入服务密钥。正式上游固定为容器内部 `http://api:8080`，不需要浏览器知道后端地址或放开 CORS。

API `/ready` 检查数据库、实例会话和接收操作状态，不检查 AI 模型；Web `/web-health` 仅检查 Nginx 存活。Compose 等待 API 健康再启动 Web。数据库、Ollama、Milvus 均不发布主机端口。

## 手动管理

以下命令使用同一项目与持久凭据文件；仅环境注入时省略 `--env-file`：

```bash
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env config --quiet
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env up -d --wait postgres api web
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env ps
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env logs --tail 100 web api
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env stop web api postgres
```

停止命令保留数据库和模型数据。不要执行删除命名卷的清理命令。若已有服务正在处理问答，升级应等待操作结束并安排停止；部署脚本会按指定服务更新容器，不能作为只读验证命令使用。

如果同一个 Compose 项目的 API 已健康运行，只添加或更新网页时可只构建 Web，再用 `--no-deps` 启动 Web，保留运行中的 API、数据库和模型服务：

```bash
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env build web
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env up -d --no-deps --wait web
```

这要求已有 API 使用同一项目网络中的 `api` 服务别名；不创建第二个 API 实例，也不迁移数据库。Web 反代所需 API 应先独立确认核心健康。

## 代理与运行边界

Web 使用独立 `web/Dockerfile`，构建阶段执行 `npm ci` 和 `npm run build`，运行阶段使用非 root Nginx（UID 101）。根文件系统只读，只在 `/tmp` 的 32MiB tmpfs 写入模板展开、进程和临时文件。API 镜像继续使用根 Dockerfile 的 `api` 目标，根 Dockerfile 默认目标仍是 CLI。Web 未启用时无需其配置或构建依赖。

Nginx 原样转发 `/api/*`，不会把 API 错误替换为网页。静态路径回退到 `index.html`，直接打开或刷新 `/workspace/:projectId` 可工作。响应设置 `Cache-Control: no-store`；认证和源码响应不经过代理缓存。请求体上限为 16KiB（16384 字节），与 API 的 JSON 上限一致，超限返回 413，不截断内容。服务密钥不在构建参数或静态包中。

默认代理等待 330 秒，是模型默认期限 300 秒加 30 秒余量；连接上游最多等待 10 秒。前端普通请求 60 秒、问答 350 秒且可中止。后端 HTTP 写入期限固定 300 秒，配置更长 AI 期限仍可能被该限制截断；如需长期 HTTP 问答，需同时协调后端、代理和客户端，单独提高 `WEB_PROXY_TIMEOUT_SECONDS` 不足以支持。分析/索引是异步任务，单次提交与查询无需等待全部任务执行。

host 模式通过宿主读取服务访问完整路径，结果保持相对路径；mapped 模式保留本地根挂载，旧相对 API 调用与项目映射兼容。已有同项目 API 时仅更新必要 API/Web 服务并保留数据库、模型和索引；API 更新可能中断在途操作。路径变化不会自动索引，单文件或子目录索引都会替换整项目快照。单数据库/索引空间只运行一个 API 实例。历史快照尚无自动清理。任务中断和发布不确定时先核对索引再重试。Web 不包含上传、Git 导入、外部模型供应商、流式问答或持久会话历史。

## 安全隔离验证

脚本流程回归不调用真实 Docker，通过隔离测试文件验证参数、凭据复用、健康失败传播和 CLI 保持独立：

```powershell
./scripts/deploy_test.ps1
```

```bash
bash scripts/deploy_test.sh
```

安装 Web 开发依赖后，从仓库根目录执行 `node scripts/web_config_test.mjs`，验证开发代理保留路径、默认期限，以及拒绝携带凭据或非法期限的设置，不联系实际 API。

代理验证应使用独立容器名、独立网络、动态 loopback 端口与伪 API，检查 SPA 刷新、原样 API 路径、授权头、无缓存和 body 限制，不能替换运行中的正式服务。完整索引问答验收使用独立 PostgreSQL/Milvus 卷、源码根及凭据文件；新数据库的项目 ID 可能与已有索引重合，禁止共用 Milvus API 索引空间。模型缓存仅明确只读复用。旧 API 验收记录见 [独立验收记录](progress/2026-10-09-api-validation.md)。本地源码目录入口及 Web 真实浏览器、HTTP、索引和 Ollama 问答全链路已在独立隔离栈完成，见 [最终验收记录](progress/2026-10-10-final-acceptance.md)。

构建并执行隔离代理回归（需要 Docker，引导伪 API 使用 `node:24-alpine`，不会访问数据库或模型）：

```powershell
docker build -f web/Dockerfile -t go-source-insight-web:local web
./scripts/web_proxy_test.ps1
```

手工直接使用基础 Compose 时默认为 mapped；deploy 入口默认为 host 并通过本地 override 启用宿主读取服务。显式 `SOURCE_ACCESS_MODE=mapped` 跳过读取服务准备，沿用原目录挂载契约。
