# API 独立验收记录（2026-10-09）

本次独立审查和验证覆盖源码扫描、静态分析、PostgreSQL 持久任务、Ollama/Milvus 索引问答，以及本机部署和关闭恢复。未使用此前测试日志作为通过证据，未提交 Git 或创建 PR。

## 审查修复

- P2，`api/database/db.go`：直接拼接 PostgreSQL 关键词连接串会误解析包含空格或引号的环境注入密码。改用 URL 编码；新增 pgx 解析回归，并用真实 PostgreSQL 角色的空格/引号密码确认新镜像可以连接。
- `api/CONTRACT.md`：精确区分新源码/任务接口外国资源的 404 和旧项目元数据/历史接口保留的 403，保持兼容行为。
- `internal/ai/milvus_snapshot_live_test.go`：真实 Ollama/Milvus 快照测试支持环境端点，便于安全隔离，不再必须依赖固定服务别名。

## Fresh 本机检查

以下检查均完成，退出码为 0：

```text
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./api/jobs ./api/handlers ./api/storage ./api/sourcefiles ./internal/ai ./internal/source ./internal/tools
go test -count=1 ./api/database ./api/storage ./api/handlers
go vet ./api/...
go build ./cmd ./cmd/ai-app ./api
go test -race -count=1 ./api/database
```

数据库修复之后重新编译了实际 API 镜像并部署，通过核心就绪、真实特殊字符凭据连接及进程生命周期检查。默认 Docker 目标实际构建为 CLI，并在没有 API 密钥的环境中成功扫描虚构源码。PowerShell 和 Bash 部署脚本行为测试均重新运行通过。

普通受限执行环境的初始 root 测试长时间未完成，已主动停止并确认退出码 1；以上通过结果来自之后实际完成且明确退出 0 的 fresh 执行。本地 HTTP 在受限环境中被拒绝访问，真实联调使用获得自动批准的本地访问。一次 AI 离线检查与镜像重建重叠中断，之后在稳定镜像上重新串行执行通过。

## 真实部署与 HTTP

独立 Compose 项目使用自己的 PostgreSQL/Milvus 卷、源码根和随机凭据。最初直接复用既有 Ollama 服务；可选 AI 部署流程使用测试自己的 Ollama 容器，只读挂载已有缓存。确认已有 `bge-m3:latest` 和 `llama3:latest`，没有模型下载。

| 链路 | 实际结果 |
| --- | --- |
| PowerShell API 默认部署 | 构建、PostgreSQL/API 健康等待通过，退出 0 |
| PowerShell `-WithAI`、Bash `--with-ai` | 真实 Docker 启动与健康等待、现有 doctor 的模型/1024 维 embedding/Milvus 检查通过，退出 0；仅附加隔离测试覆盖，未使用模拟 Docker |
| 认证及归属 | 两用户注册/登录；无令牌和错误令牌 401；外国源码/任务/取消 404 |
| 输入及路径 | 穿越、绝对路径、客户端 collection/额外字段 400；路径/链接/Windows junction 有意义回归测试通过 |
| 扫描与分析 | 同步扫描 200；异步分析 202 后 completed；相对 `sample.go`，`//line` 后仍用物理位置；const/var 的 G101 在 4/5 行，未关闭文件 B102 仅在 9 行 |
| 旧分析兼容 | 统一任务提交 completed，旧 analysis ID 和历史查询可用 |
| 索引与问答 | 两项目分别 completed；真实 llama3 答案非空且引用相关源内容，路径为项目相对路径、物理行号 |
| 索引隔离与替换 | 两项目不会返回彼此文件；再次索引成功；不存在的 file filter 返回空答案/来源；移走源目录后仍可问答 |
| AI 真不可达 | 断开测试 API 对 Ollama 的网络；health/ready 正常、scan/analyze 完成，问答安全 503、索引持久化 ai_unavailable；网络恢复 |
| 真实容量和取消 | 暂停测试 Milvus 形成可控长任务；queue_full、project_busy、外国取消、queued/running 取消及队列/项目 gate 回收通过；恢复后再次索引 completed |
| 第二 API 实例 | 实际退出 1，诊断 instance ownership；不会改写原进程 running/queued 状态 |
| SIGKILL 和重启 | 遗留 running/queued 变 failed/interrupted，保留 publication 提示；无自动重放 |
| SIGTERM / SIGINT | 实际正常退出 0；SIGTERM 带活动/排队任务有界关闭并持久化 canceled；重启 ready |
| 专用 PG 会话丢失 | 实际终止 advisory-lock 会话，原 API 取消任务并退出 1，非终态任务为零；重启重新获得锁并 ready |

## 旧数据保护

真实隔离 Milvus 测试 `TestMilvusSnapshotLive` 退出 0。使用合成向量执行约 72 MB、1200 行的分批快照替换，验证新实例通过 alias 读取；注入第二批写入失败后旧 1200 行和 legacy 物理集合仍保留；空快照查询为空。真实 API 的索引/问答另行使用实际 Ollama embedding 与聊天模型。

确认发布优先于取消/超时/关闭失败、未知发布标记，以及持久化失败保留最后 durable 状态的故障组合由 fresh 单元/HTTP 回归覆盖。没有在真实 Milvus 上人为制造不可判定的 alias RPC 成功后断线，也未故意破坏真实数据库持久化；这些情况不混称为实测。

## 部署边界

支持本机、单 API 实例；实例连接由每两秒的监控检查并触发有界关闭，不承诺零延迟分布式 fencing。新数据库不可共享相同 API Milvus 命名空间。聊天模型 doctor 预检继续暂缓；Web、上传/Git 拉取、外部模型 API、多副本和历史快照自动清理未扩展。

启动和配置说明见 [API README](../../api/README.md)、[接口契约](../../api/CONTRACT.md)。

```powershell
./scripts/deploy.ps1 -Mode api
./scripts/deploy.ps1 -Mode api -WithAI
```

```bash
bash scripts/deploy.sh --mode api
bash scripts/deploy.sh --mode api --with-ai
```

验收后只清理本轮明确记录的测试容器、独立测试卷/网络、专属镜像、随机凭据、令牌及虚构源码。保留既有用户 PostgreSQL/Milvus 数据、Ollama 服务和模型缓存，以及工作区原有修改。脱敏证据保留在已忽略的本地验收目录。
