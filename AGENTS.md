# 项目协作指南

本文件适用于整个仓库。先阅读适用的分目录指南和相关契约，再修改代码；用户当次明确指令优先于本指南。

## 目标与范围

GoSource-Insight 提供 Go 源码扫描、静态分析和基于 Ollama / Milvus 的 RAG 源码问答，已有 CLI、HTTP API、React Web 工作台及跨平台部署入口。优先保证真实可用、数据安全和如实呈现结果。

- 扫描、静态分析不依赖 AI；索引、问答需要 Ollama、Milvus 和匹配的向量模型维度。
- Web 已支持注册登录、项目选择、扫描、分析、索引、问答及任务轮询/取消。这是私人本地项目，无需管理员或多人服务器流程；部署时通过 `-SourceDirectory` / `--source-directory` 选本机源码目录，各项目可直接填写该根内完整目录/文件路径，旧项目映射与 legacy 相对调用继续兼容。
- 尚不支持源码上传、Git 导入、外部模型供应商 API、流式问答或服务端多轮记忆。页面当前问答记录及旧分析历史不等于模型记忆，也不等于完整服务端任务历史。

## 协作方式

- 默认由主智能体派发、协调和验收，实现与测试交给 GPT6.1 Sol 子智能体；后续用户明确调整时遵从新指令。模型不可用时如实说明，不静默换模型。
- 并行前约定接口、文件负责人和构建输出目录。不得同时写同一文件、依赖锁文件或同一 `dist`；需要修改他人负责的文件时先协调交接。
- 独立验收须检查实际代码和当前结果，不能仅引用前人通过的测试报告。授权范围内持续完成工作，只有缺少必要信息或授权时才向用户澄清，不为日常可逆操作增加确认流程。
- 不在未获授权时对外发消息、发布成果或执行 Git 提交/推送。

## 架构与文件职责

仓库只有根目录 Go 模块 `go-ai-study`，工具链版本以 [go.mod](go.mod) 为准；不要在 `api/` 创建嵌套模块。

| 路径 | 职责 |
| --- | --- |
| `cmd/main.go`、`internal/cli/` | 统一 CLI 入口、命令调度和报告输出；`cmd/ai-app/` 仅兼容入口 |
| `internal/source/` | 共享源码发现、读取与扫描规则 |
| `internal/analysis/`、`internal/tools/` | 静态分析聚合、AST 规则与工具管理 |
| `internal/ai/` | Go 声明切块、Ollama、Milvus 快照、检索和来源依据 |
| `internal/config/`、`config/config.json` | 配置加载/校验及通用示例 |
| `api/main.go`、`api/routes/`、`api/handlers/` | HTTP 生命周期、路由、认证和请求边界 |
| `api/service/`、`api/jobs/`、`api/storage/`、`api/sourcefiles/` | 核心服务适配、持久任务、PostgreSQL/实例锁、受控源码目录 |
| `web/src/api/`、`web/src/types/` | 相对 API 客户端与接口类型 |
| `web/src/stores/`、`web/src/utils/`、`web/src/hooks/` | 会话、项目状态、任务恢复与轮询 |
| `web/src/pages/Workspace/`、`web/src/components/Workspace/` | 真实项目工作台与结果展示 |
| `Dockerfile`、`docker-compose*.yml`、`web/Dockerfile`、`web/nginx.conf` | CLI/API 镜像、服务编排及同源 Web 代理 |
| `scripts/`、`docs/` | 部署与隔离验证入口、契约和运维文档 |

## 开发与部署命令

以下 Go 命令从仓库根目录执行；API 原生启动前按 [api/.env.example](api/.env.example) 注入 PostgreSQL、强 `JWT_SECRET` 和受控 `SOURCE_ROOT`，不要把示例当作可用凭据。

```bash
go run ./cmd help
go run ./cmd scan ./internal
go run ./cmd analyze ./internal
go build -o ./bin/go-ai-insight ./cmd
go build -o ./bin/api-server ./api
go run ./api
go test -count=1 -timeout=120s ./internal/... ./cmd/... ./api/... ./scripts/source-binding/...
```

Windows 构建产物可加 `.exe`。测试按修改范围选择对应包，不要求每次执行上面的全部测试。`go test ./...` 的 Go 包发现可能纳入 `sources/` 中运行时源码副本或前端依赖附带的 Go 目录；不能为使该命令通过而删除用户源码/依赖，也不能将维护代码测试通过宣称为整个运行目录通过。维护代码范围使用上面的显式包列表。索引/问答和 `doctor` 的命令、collection 选择见 [CLI_README.md](CLI_README.md)。

Web 使用 Node 24（与构建镜像一致），在 `web/` 执行：

```bash
npm ci
npm run dev
npm test
npm run lint
npm run build
```

`build` 包含 TypeScript 检查。开发代理设置为服务端读取的 `WEB_API_PROXY_TARGET`，可放在已忽略的 `web/.env.local`；浏览器始终请求相对 `/api/v1`。首次安装依赖、拉取镜像或准备缺失模型可能需要联网。

部署需要 Docker Compose V2 和 Linux 容器引擎，在仓库根目录执行：

| 模式 | PowerShell | Bash |
| --- | --- | --- |
| CLI 与首次模型准备 | `./scripts/deploy.ps1 -Mode cli` | `bash scripts/deploy.sh --mode cli` |
| API 核心 | `./scripts/deploy.ps1 -Mode api` | `bash scripts/deploy.sh --mode api` |
| Web 与 API 核心 | `./scripts/deploy.ps1 -Mode web` | `bash scripts/deploy.sh --mode web` |
| API 与缓存 AI 检查 | `./scripts/deploy.ps1 -Mode api -WithAI` | `bash scripts/deploy.sh --mode api --with-ai` |
| Web 与缓存 AI 检查 | `./scripts/deploy.ps1 -Mode web -WithAI` | `bash scripts/deploy.sh --mode web --with-ai` |

私人本地 Web 常用入口为 `./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'` 或 `bash scripts/deploy.sh --mode web --source-directory '/workspace/project'`；脚本验证目录并完成只读挂载，不需要填写所有者/项目 ID。已有同项目 API 时，仅更新 API/Web 并保留数据库与模型服务；相同待应用配置可重试，不覆盖其他待应用配置。

无参数部署默认为 CLI；该模式只下载缺失模型并复用已有缓存。API/Web 的 `WithAI` 只启动并检查已有模型，不下载缺失模型；核心健康不代表 AI 在线。

API/Web 首次部署通过既有机制生成强凭据并保存到受限、已忽略的 `.runtime/api.env`，重复使用原文件；也可同时注入 `API_DB_PASSWORD`、`API_JWT_SECRET`。自选文件参数为 `-ApiEnvFile PATH` / `--env-file PATH`。不得打印凭据或完整容器环境/Compose 展开配置，不得把服务密钥放入 `VITE_`、构建参数或静态包。

部署脚本会更新指定服务，不能当作只读检查。已有同项目 API 正在运行时，只增加/更新 Web，按 [Web 部署说明的手动管理](docs/WEB_DEPLOYMENT.md#手动管理) 使用 `build web` 和 `up -d --no-deps --wait web`；先确认同项目网络的 `api` 别名，保留运行中的 API、数据库和模型服务。

## 不可破坏的契约

- CLI/API/索引复用共享扫描核心；读取失败不能算完整成功。扫描与索引包含测试源码，静态分析默认排除 `_test.go`，UI 范围说明须准确。Go 来源位置使用未经 `//line` 调整的物理行号（`PositionFor(..., false)`）。
- 服务端派生源码根、所有者、项目和 collection。客户端可提交相对目标或本地配置根内的宿主机完整路径，后端必须转换为相对目标；不得读取未授权主机路径或接受客户端模型/服务地址。`SOURCE_BINDINGS_FILE` 由本地部署入口生成，可含无需身份 ID 的 `local_root` 及兼容的项目 `bindings`；完整输入使用本地根，relative API 保留旧项目根，本地根失效/越界不得回退。保留 `os.Root`、链接/重解析点和逃逸拒绝。来源与 ask 文件过滤仍使用相对路径；所属项目的 source-info 返回 `source_mode` 和有效 Web 根，不暴露内部容器根。`binding_id` 仅标识配置变化，不能据此认为索引已更新。
- Milvus 新快照分批写入，刷新、建索引、加载及条数验证后才通过 alias 发布。发布前失败保留旧索引；发布结果未知可能已提交，不能当作“未执行”自动重放。已确认发布不会被取消回滚；旧快照不得擅自删除。索引子目录同样替换整项目快照，提交前须明确提示。
- API 一个数据库/索引空间只运行一个实例，保留专用 PostgreSQL advisory lock、先持久化再接受任务、真实状态与关闭流程。同项目 index/ask 互斥至资源关闭；`canceling` 等待真实终态，不伪造取消完成。启动将遗留任务标记 `interrupted`，不得自动重放可能已经发布的工作。
- 前端会话及任务按所有者/项目隔离；晚到请求、401、轮询结果不能串账号、覆盖新项目或使终态/取消状态倒退。恢复仅限已知任务 ID，不能暗示存在服务端任务列表。浏览器没有索引记录不代表服务端没有索引，仍允许查询既有索引。
- 静态规则和低置信度结果只称候选、待复核提示；区分未执行、空结果和零命中。模型输出、源码和错误作为不可信文本展示，保留 XSS 防护。不要伪造统计、流式输出或 AI 健康。
- 正式 Web 同源转发 `/api/*` 并保留路径；不要扩大 CORS 来修复代理错误。保持 SPA 刷新、认证/源码无缓存及请求体限制；调整长问答期限时同时核对后端 HTTP、模型、代理和客户端。

## 配置、数据与工作区保护

- 提交的配置、示例和文档必须通用，不含个人姓名、主机路径、私有地址或凭据。本地秘密仅放受限、已忽略文件，排除于镜像/浏览器包、日志、截图和报告；不要读取或展示无关秘密。
- 保留当前 dirty 改动，不通过 reset、stash 或覆盖他人文件恢复环境。不要删除用户数据卷、模型缓存、现存索引或凭据文件；凭据不匹配应核对配置，不能以清空数据库解决。
- 源码绑定工具只生成受限、已忽略的不可变 JSON 代文件和原子 Compose 入口，保留旧代及其他项目；根/排除变化不得自动索引。只授权目录，不接受单文件后默默挂载父目录。运行产物排除按实际项目根相对位置配置，不能全局禁止所有名为 `sources` 的业务目录。应用挂载是实际 API 更新，安排在途关闭并保留数据库/模型和用户 CLI 窗口。
- 不操作用户正在输入的 CLI 窗口，不对运行中的 CLI/API/Web/模型服务做无关停止或重启。必要升级遵从本次授权并处理在途操作。
- 测试使用独立账号/项目及明确归属的资源。全新 PostgreSQL 可能重新分配低位项目 ID，禁止直接连接已有 Milvus API collection 空间；隔离栈需独立数据库、Milvus 命名空间/卷、源码根及凭据。模型缓存仅按约定复用，不为测试重复下载或变更。清理只处理本次创建且归属已确认的测试资源。

## 验证与交付

- 依改动范围做必要验证；纯文档检查内容、链接和 `git diff --check` 即可，不跑全量测试。核心数据、目录边界或异步状态修改须覆盖相应失败/取消/恢复场景。
- 前端依赖或锁文件变更应验证标准干净 `npm ci`，不能让旧 `node_modules` 掩盖问题；执行相关状态测试、lint 和标准 build，同一输出目录只允许一个构建。
- 部署修改检查实际构建、运行镜像身份、健康、SPA 刷新和 API 代理。配置验证或 mock 不能替代真实 HTTP、浏览器、索引和 Ollama 问答；明确每项采用真实服务还是可控模拟。
- 现有验证入口：Go 各包 `*_test.go`，`web/tests/`；根目录 `./scripts/deploy_test.ps1` / `bash scripts/deploy_test.sh`、`node scripts/web_config_test.mjs`，以及需要 Docker 的 `./scripts/web_proxy_test.ps1`、`./scripts/source_binding_test.ps1`。后者仅启动一次无网络临时挂载容器；部署流程/代理配置测试含模拟依赖，应核对其实际边界。
- Go 真实 AI 测试通过 `GO_AI_INSIGHT_LIVE_TEST=1` 或 `GO_AI_INSIGHT_SNAPSHOT_LIVE=1` 显式开启；执行前先阅读对应 live test，使用隔离资源与已有模型，不默认启用。
- 新发现失败须明确报告，不能用旧通过日志遮盖。交付说明实际改动、执行过的验证及结果、保留/清理资源和剩余限制；证据不得包含 token、凭据或个人机器路径。

## 文档导航

- [README.md](README.md)：总体入口、配置和部署。
- [CLI_README.md](CLI_README.md)、[USER_MANUAL.md](USER_MANUAL.md)：命令、使用流程与索引保护/恢复。
- [api/CONTRACT.md](api/CONTRACT.md)、[api/README.md](api/README.md)：接口、任务/发布语义与 API 使用。
- [web/README.md](web/README.md)、[docs/WEB_DEPLOYMENT.md](docs/WEB_DEPLOYMENT.md)：工作台流程、代理与安全部署。
- [docs/SOURCE_BINDINGS.md](docs/SOURCE_BINDINGS.md)：宿主目录授权、完整路径映射、只读挂载与最小服务更新。
- [docs/TODO.md](docs/TODO.md)：尚未实现的范围；历史记录不能替代当前代码和契约。
