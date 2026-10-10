# GoSource-Insight 使用手册

本项目提供 `go-ai-insight` CLI、HTTP API 和 Web 工作台，使用同一套源码扫描、静态分析、Ollama 代码索引及问答能力。CLI 默认部署保留；API/Web 通过独立模式启动。上传、Git 导入和外部模型供应商 API 暂列 [TODO](docs/TODO.md)。

## 1. 扫描和静态分析

安装 Go 1.25.5 或兼容工具链后，在项目根目录运行：

```bash
go run ./cmd help
go run ./cmd scan ./internal
go run ./cmd analyze ./internal
```

- `scan PATH` 发现 Go 文件，统计生产/测试文件数、总行数和跳过目录。
- `analyze PATH` 对生产 Go 文件执行复杂度、安全风险和常见 Bug 规则，默认不分析 `_test.go`。
- 两条命令可接受单文件或目录，不依赖 Ollama 或 Milvus。

扫描过程跳过符号链接；入口本身是符号链接时返回错误。默认忽略 `.git`、`vendor`、`node_modules`、`dist`、`build`，以及 IDE / agent 工具目录 `.idea`、`.vscode`、`.hermes`、`.qoder`、`.codex`。

规则命中是需要进一步检查的线索，不代表已确认缺陷。单项检查可使用 `complexity FILE`、`security FILE`、`bug FILE`；`test FILE` 当前仅生成需要手动完善的测试骨架。

## 2. 一键准备问答环境

先启动 Docker 引擎，确认 Docker Compose V2 可用，并使用 Linux 容器。首次执行需联网下载镜像及模型，为模型权重和向量数据库预留足够资源。

PowerShell：

```powershell
./scripts/deploy.ps1
```

Linux / macOS：

```bash
bash scripts/deploy.sh
```

部署脚本执行以下流程：

1. 检查 Docker 和 Compose 配置。
2. 构建统一 CLI 镜像，启动并等待 Ollama、Milvus 健康。
3. 通过一次性 `models` 服务准备 `bge-m3:latest` 和 `llama3:latest`，复用已安装模型，仅下载缺失模型。
4. 执行 `doctor` 检查模型、实际向量维度和 Milvus 连接，成功后提示就绪。

脚本每一步失败都会返回非零状态；修复问题后可重新运行。重复执行时不会为已安装模型联网检查更新，最后仍由 `doctor` 校验实际模型和服务。`doctor` 不创建源码索引。默认使用 CPU，依赖服务不发布宿主机端口。

Milvus 使用官方单容器方案，在容器内运行内嵌 etcd 和本地存储。模型及数据库使用命名卷，停止服务后保留数据。部署脚本不创建 `.env` 或修改已有配置；本地 `.env` 仅按需用于参数覆盖，并已被忽略。

## 3. 索引源码并问答

部署成功后运行：

```bash
docker compose run --rm insight scan /workspace
docker compose run --rm insight analyze /workspace
docker compose run --rm insight index --collection example_project /workspace
docker compose run --rm insight ask --collection example_project "源码扫描和静态分析如何衔接？"
```

`insight` 是运行后退出的 CLI，默认命令为 `help`。`/workspace` 默认只读挂载当前项目。索引切块覆盖完整 Go 声明，包括函数、方法、类型（结构体及接口）、常量、变量、import，并保留包声明和注释。超长内容继续拆分，解析失败时回退到按源码行分块。

首次整库 `index` 的耗时取决于源码规模、模型和硬件，CPU 运行时可能较长。CLI 等待期间仍可能在生成向量；收到索引摘要且退出码为 0 才表示成功。可按 Ctrl+C 取消，取消或失败后按需重新执行索引。

一个 collection 对应一个代码库。不同项目使用不同的 `--collection`，索引与问答必须选择同名 collection。源码修改或删除后重新执行 `index`。更新采用独立快照，验证成功后切换内部查询别名；同一 collection 不要并发索引或在更新过程中问答。

### 索引数据保护与恢复

每次更新分批写入新的物理 collection，完成刷新、向量索引、加载和条数校验后，才切换查询入口。发布前失败时，旧索引保持可用，并尝试清理未发布的新快照；清理失败会明确报错。发布请求超时会重新确认别名是否已切换；仍无法确认时保留新旧数据，不删除可能已经生效的新快照。

配置中的 collection 名称保持不变。内部入口名为 `gsi_active_<项目名哈希>`，快照名为 `gsi_snapshot_<项目名哈希>_<随机标识>`；哈希为 collection 名称的 SHA-256 前 16 字节的十六进制表示。没有内部别名时，仍可读取原来的同名 collection。首次成功更新后也保留原 collection。

历史快照保存在 Milvus 数据卷中，仅卸载内存，不自动删除。历史列表、回滚和清理命令暂列 TODO。需要手工恢复时，先停止该 collection 的索引和问答，用 Milvus 管理接口核对旧快照的 schema、维度、项目标记及内容，加载目标快照后，将内部别名重新指向它。若恢复首次迁移前的原 collection，则移除内部别名，使读取回退到原 collection。保留已确认可用的数据后再清理其他快照，勿删除正在使用的快照或数据卷；磁盘容量应覆盖新旧索引同时存在的开销。

读取源码或目录失败会中止索引，避免将无法读取的文件当成已删除。代码分块、复杂度及 Bug/安全问题使用实际文件的物理行号，忽略 `//line` 提供的映射位置。

资源关闭检测会检查直接关闭、常见的 defer 关闭、变量遮蔽及重新赋值，不要求调用方关闭 `os.WriteFile` 的内部句柄。它仍是局部语法分析：跨函数转移资源所有权等复杂写法可能保留告警，需结合源码判断。硬编码凭据检测包含赋值语句，以及包级、局部和分组的 const/var 声明。

按文件限定检索时，`--file` 使用相对于索引根目录的路径：

```bash
docker compose run --rm insight ask --collection example_project --file internal/ai/rag_service.go "索引是如何更新的？"
```

问答返回回答和源码依据，索引元数据仅保存代码库相对路径。没有相关源码时返回明确说明；应结合返回的来源核对模型回答。

分析其他目录时，通过本地环境变量选择挂载源。例如，在项目根目录存在通用示例目录 `sample-project` 时：

```powershell
$env:SOURCE_DIR = './sample-project'
docker compose run --rm insight index --collection sample_project /workspace
```

```bash
SOURCE_DIR=./sample-project docker compose run --rm insight index --collection sample_project /workspace
```

相对挂载路径以 Compose 文件所在目录为基准。之后扫描或索引该项目时使用同样的挂载源；问答直接读取向量库。

若使用独立部署的 Ollama 和 Milvus，也可运行本地 CLI：

```bash
go run ./cmd doctor
go run ./cmd index --collection example_project ./internal
go run ./cmd ask --collection example_project --file ai/rag_service.go "如何检索源码？"
```

此例索引根目录为 `./internal`，因此文件过滤路径为 `ai/rag_service.go`。

## 4. 配置与隐私

配置优先级为：内置默认值 → `-c FILE` 显式配置文件 → 环境变量 → 对应命令行选项。不指定配置文件时使用内置默认值；`config/config.json` 是通用示例。

所有模型及检索参数使用 `GO_AI_INSIGHT_` 前缀，包括：

- `OLLAMA_ENDPOINT`、`MILVUS_ENDPOINT`
- `EMBEDDING_MODEL`、`CHAT_MODEL`、`COLLECTION`
- `VECTOR_DIM`、`TOP_K`、`CHUNK_MAX_LINES`、`CHUNK_MIN_LINES`
- `TIMEOUT_SECONDS`

完整默认值见 [README 的通用配置](README.md#通用配置)。Compose 使用容器服务名连接后端，并支持在本地 `.env` 或调用者环境中覆盖模型和检索参数。更换模型后重新执行部署脚本；更换向量模型时匹配其实际向量维度并重新索引。

真实地址、个人绝对路径、账号或凭据仅通过本地环境或已忽略的 `config/config.local.json` 覆盖，不写入仓库、文档或通用示例。不要提交本地 `.env`。部署脚本保留已有配置，源码索引不保存个人目录路径。

## 5. 报告与错误

全局选项放在命令前；`index` / `ask` 的命令选项放在命令后。

```bash
go run ./cmd -f json scan ./internal
go run ./cmd -f json -o analysis.json analyze ./internal
go run ./cmd -c config/config.json index --collection example_project ./internal
```

JSON 报告结构为 `{"success":true,"result":{...}}`，`result` 是对象，直接解析即可。日志默认写入 stderr，报告写入 stdout；`-o FILE` 导出报告文件。成功返回 0，参数、配置或执行失败返回非零状态，错误写入 stderr。

容器源码挂载只读，导出报告可由宿主机接收 stdout：

```bash
docker compose run --rm -T insight -f json analyze /workspace > analysis.json
```

完整参数和报告字段见 [CLI_README.md](CLI_README.md)。

## 6. 检查和停止

```bash
docker compose ps
docker compose run --rm -T insight -f json doctor
docker compose logs --tail 100 milvus ollama
docker compose down
```

如果 `doctor` 报告模型缺失，重新运行部署脚本准备模型；若维度不匹配，修正向量模型或 `VECTOR_DIM` 后重新索引；连接失败时检查对应服务状态和本地配置。问答无结果时确认 collection、是否已完成索引及 `--file` 路径。

开发验证：

```bash
go test ./internal/... ./cmd/...
go build -o ./bin/go-ai-insight ./cmd
```

Web 工作台可用；外部模型 API、增量索引、索引回滚及其他后续工作见 [docs/TODO.md](docs/TODO.md)。

## Web 工作台使用

在仓库根目录运行 `./scripts/deploy.ps1 -Mode web` 或 `bash scripts/deploy.sh --mode web`，默认打开 `http://127.0.0.1:3000`。已有模型缓存时添加 `-WithAI` / `--with-ai`，不会下载模型。脚本复用 API 凭据和 PostgreSQL 卷，网页就绪只表示核心可用，不保证 AI 在线。

先使用 `./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'` 或 `bash scripts/deploy.sh --mode web --source-directory '/workspace/project'` 选已有本地源码目录。注册登录，在 `/workspace` 新建/选择项目，直接填写该根内完整目录或 `.go` 文件路径；无需管理员、所有者/项目 ID 或独立绑定。旧相对 API 调用与项目映射继续兼容。详见 [本地源码目录](docs/SOURCE_BINDINGS.md)。路径属于 API 宿主机授权范围，不能任意读取远程浏览器电脑或其他项目目录。「扫描源码」后「静态分析」，在「分析结果」核对规则线索。AI 已准备好后「建立索引」，确认替换整个项目范围，等待任务完成，再到「源码问答」发送问题并核对相对来源与物理行号。

换根/排除项只改变后续源码读取，不自动更新旧索引；需要用户在新范围下明确确认建立索引。未配置映射、越界、链接或不可读目标会给出状态指引；目录部署入口会更新 API/Web 并处理在途任务；底层配置生成本身不重启服务。工具只接受目录根，单文件是目录内操作范围，不会偷偷授权整个父目录。

「任务记录」只记录本浏览器、当前账号和项目提交过的任务 ID，轮询服务端状态；当前没有完整服务端任务列表、流式问答或持久会话历史。运行中止可能处于 `canceling`，已发布的索引不回滚。子目录索引会替换整项目范围；中断或发布不确定时先核对索引再重试。问答等待有界，默认模型 300 秒、代理 330 秒、客户端 350 秒，后端 HTTP 写入仍受 300 秒限制。详细操作与配置见 [Web 说明](web/README.md) 和 [部署说明](docs/WEB_DEPLOYMENT.md)。

## HTTP API 使用

在仓库根目录运行 `./scripts/deploy.ps1 -Mode api` 或 `bash scripts/deploy.sh --mode api`。这会生成并复用忽略目录 `.runtime/api.env` 中的服务凭据，构建非 root API 镜像，启动持久 PostgreSQL，并等待 `/ready`。默认 API 只绑定主机 `127.0.0.1:8087`；依赖不发布端口。`/health` 反映存活，`/ready` 反映本实例仍接受操作及必需数据库/实例会话状态，不要求 AI 在线。

已有模型缓存时，添加 `-WithAI` / `--with-ai` 启动并检查 Ollama/Milvus，不下载模型。原 CLI 部署入口仍可用于首次准备模型。API 扫描/静态分析无需 AI；模型配置按需初始化。

完整可执行 PowerShell/Bash 示例见 [api/README.md](api/README.md)：选择自己的账号注册、登录，部署时选本地源码目录，创建项目后直接提交该根内完整路径，无需手填所有者/项目 ID。API 只读读取配置范围，不提供上传、Git 拉取或不受控制的主机路径选择。Linux 上容器 UID 10001 需能读取源码及遍历祖先目录。

用相对 `target` 或已映射根内的完整目标扫描/提交分析，拿到 `job_id` 后查询 `/api/v1/jobs/:id`。索引同样是异步任务；完成后调用 `ask`，可选相对 `file` 限定来源。任务/来源路径保持相对，内容与物理行号保持原样，读取已发布索引不依赖源码目录仍存在。

同项目索引/问答忙时返回 409。索引 `pkg` 会替换整个项目快照，仅留下 `pkg`；再次索引 `.` 才恢复整库。历史快照保留并消耗磁盘，当前没有自动清理。取消排队任务释放容量；运行取消只是请求，确认已发布的索引仍完成且不回滚。重启遗留任务变为 `interrupted`，发布状态不明为 `index_publication_unknown`，两者都可能已发布新快照，不能当成确定未执行。详见 [接口契约](api/CONTRACT.md)。

只能运行单 API 实例；PostgreSQL 会话锁阻止第二实例在启动时回收现有任务。更换 JWT 密钥后需要重新登录；数据库凭据必须和持久卷匹配，脚本不自动修改已有密码。只停止服务时使用 API Compose 文件组合的 `stop api postgres`，保留凭据及命名卷，不执行删除数据卷的清理。独立验收需要隔离 PostgreSQL/Milvus，不能用新数据库的相同项目 ID 覆盖已有索引。
