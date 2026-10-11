# GoSource-Insight

GoSource-Insight 提供 Go 源码扫描、静态分析，以及基于 Ollama 和 Milvus 的代码问答。CLI、HTTP API 和 Web 工作台复用核心服务；扫描和静态分析可独立运行。Web 支持本机项目工作台，外部模型供应商 API 仍暂缓。

GoSource-Insight provides Go source scanning, static analysis, and code Q&A powered by Ollama and Milvus. The CLI, HTTP API, and Web workspace share core services; scanning and static analysis can run independently. The Web workspace supports local projects. External model provider APIs remain deferred.

<a id="写给学习者与-go-开发者"></a>

## 写给学习者与 Go 开发者 / For learners and Go developers

这个项目是一半 vibe coding，对于非计算机专业的学生也可以学习；对于计算机专业的学生，这个项目包含了向量检索、数据库、Go 语言 Web 开发、Ollama 本地部署、RAG 检索增强，如果你有兴趣，还可以自行扩展，接入使用 API key 的外部模型服务，是一个学习 AI 应用的好项目。

对于 Go 语言开发者，我们完全尊重你的知识和你的技术，所以我们没有内置外部模型 API。你的代码应该属于你，无论你手敲还是 vibe coding，都是你宝贵的知识成果，希望这个项目可以帮到你。

最后，如果喜欢我们的项目，可以点一颗 star，我们会持续更新。

<a id="for-learners-and-go-developers"></a>

About half of this project was developed through vibe coding, and students from non-computer-science backgrounds can learn from it too. For computer science students, it brings together vector search, databases, web development in Go, local Ollama deployment, and retrieval-augmented generation (RAG). If you are interested, you can extend it yourself to connect to external model providers using an API key. It is a good project for learning how to build AI applications.

For Go developers, we fully respect your knowledge and technical skills. That is why we have kept the model integration local and have not built in external model APIs. Your code should belong to you: whether you write it by hand or through vibe coding, it is valuable intellectual work. We hope this project can help you.

Finally, if you like the project, please give it a star. We will keep updating it.

<a id="核心命令"></a>

## 核心命令 / Core commands

| 命令 / Command | 职责 / Purpose | 依赖 / Dependencies |
|---|---|---|
| `scan PATH` | 发现 Go 文件，统计生产代码、测试代码和行数<br>Discover Go files; count production/test files and lines | 无 AI 服务依赖<br>No AI services required |
| `analyze PATH` | 对生产 Go 文件执行复杂度、安全和常见 Bug 规则<br>Run complexity, security, and common bug rules on production Go files | 无 AI 服务依赖<br>No AI services required |
| `index [--collection NAME] PATH` | 扫描、按 Go 声明切块、向量化，更新代码库索引<br>Scan, chunk by Go declarations, embed, and update the repository index | Ollama、Milvus / Ollama, Milvus |
| `ask [--collection NAME] [--file RELATIVE_PATH] QUESTION` | 检索已索引代码，回答问题并返回源码依据<br>Retrieve indexed code, answer questions, and return source evidence | Ollama、Milvus / Ollama, Milvus |

`complexity`、`security`、`bug` 是单项检查入口；`test` 目前生成测试骨架，不属于核心验收功能。静态规则结果是待检查的线索，不等同于已确认缺陷。

`complexity`, `security`, and `bug` run individual checks. `test` currently generates test skeletons and is outside the core acceptance scope. Static rule results are leads to review, not confirmed defects.

<a id="直接运行扫描和分析"></a>

## 直接运行扫描和分析 / Run scanning and analysis directly

需要 Go 1.25.5 或兼容工具链。在项目根目录执行：

Go 1.25.5 or a compatible toolchain is required. Run these commands from the project root:

```bash
go run ./cmd help
go run ./cmd scan ./internal
go run ./cmd analyze ./internal
go run ./cmd -f json -o analysis.json analyze ./internal
```

也可以构建统一的 CLI：

You can also build the unified CLI:

```bash
go build -o go-ai-insight ./cmd
./go-ai-insight scan ./internal
```

Windows 可构建为 `go-ai-insight.exe`。全局选项放在命令前；`index` 和 `ask` 的命令选项放在命令后。

On Windows, build it as `go-ai-insight.exe`. Global options precede the command; options for `index` and `ask` follow it.

<a id="一键准备完整问答环境"></a>

## 一键准备完整问答环境 / Set up the complete code Q&A environment

需要已启动的 Docker 引擎和 Docker Compose V2，运行 Linux 容器。首次运行需要联网下载镜像和模型，并预留模型权重、向量数据所需的内存和磁盘空间。默认使用 CPU；GPU 配置作为后续增强。

A running Docker engine and Docker Compose V2 with Linux containers are required. The first run needs internet access to download images and models, plus enough memory and disk space for model weights and vector data. CPU execution is the default; GPU configuration is planned as a future enhancement.

PowerShell：

PowerShell:

```powershell
./scripts/deploy.ps1
```

Linux / macOS：

Linux / macOS:

```bash
bash scripts/deploy.sh
```

脚本会检查 Docker、构建 CLI 镜像、等待 Ollama / Milvus 健康，再通过一次性 `models` 服务准备 `bge-m3:latest` 和 `llama3:latest`。已安装模型直接复用，仅下载缺失模型；重复执行时不会为已有模型联网检查更新。最后执行 `doctor` 检查模型实际向量维度和 Milvus 连接，成功后才提示部署就绪；检查不会创建源码索引。模型下载显示进度；任一步失败，脚本返回非零状态，修复后可重新运行。

The script checks Docker, builds the CLI image, waits for Ollama / Milvus to become healthy, and then prepares `bge-m3:latest` and `llama3:latest` through the one-off `models` service. Installed models are reused; only missing models are downloaded. Repeated runs do not check online for updates to existing models. Finally, `doctor` checks the actual embedding dimension and the Milvus connection. Readiness is reported only after those checks succeed; the checks do not create a source index. Model downloads show progress. Any failed step causes a nonzero exit status; rerun the script after fixing the issue.

Milvus 采用官方单容器方案，内嵌 etcd 并使用本地存储，减少运行依赖。模型和数据库使用命名卷持久化；默认不向宿主机发布依赖服务端口。部署脚本不写入本地 `.env` 或修改配置；`.env` 可按需提供通用参数覆盖，并已被忽略。

Milvus uses the official single-container deployment with embedded etcd and local storage to reduce runtime dependencies. Models and database data persist in named volumes; dependency service ports are not published to the host by default. The deployment script does not write a local `.env` or modify configuration. An optional `.env` can override general parameters and is ignored by Git.

完成后，在项目根目录运行：

After setup, run these commands from the project root. The Q&A example asks how source scanning and static analysis fit together:

```bash
docker compose run --rm insight scan /workspace
docker compose run --rm insight analyze /workspace
docker compose run --rm insight index --collection example_project /workspace
docker compose run --rm insight ask --collection example_project "源码扫描和静态分析如何衔接？"
```

`insight` 是运行后退出的 CLI 服务，默认命令为 `help`。`/workspace` 默认只读挂载当前项目。分析其他目录时，仅通过本地环境变量设置通用挂载入口；例如：

`insight` is a CLI service that exits after running its command; its default command is `help`. `/workspace` mounts the current project read-only by default. To analyze another directory, set the general mount entry through a local environment variable, for example:

```powershell
$env:SOURCE_DIR = './sample-project'
docker compose run --rm insight index --collection sample_project /workspace
```

```bash
SOURCE_DIR=./sample-project docker compose run --rm insight index --collection sample_project /workspace
```

相对挂载路径以 Compose 文件所在目录为基准。之后的扫描或索引命令使用同样的 `SOURCE_DIR`。问答读取向量库，不需要重新扫描目录。

Relative mount paths are resolved from the directory containing the Compose file. Use the same `SOURCE_DIR` for subsequent scanning or indexing commands. Q&A reads the vector database and does not need to scan the directory again.

容器的源码挂载只读。需要导出 JSON 报告时，可由宿主机接收输出：

The source mount in the container is read-only. To export a JSON report, capture the output on the host:

```bash
docker compose run --rm -T insight -f json analyze /workspace > analysis.json
```

服务检查和停止：

Check and stop services:

```bash
docker compose ps
docker compose run --rm -T insight -f json doctor
docker compose logs --tail 100 milvus ollama
docker compose down
```

`docker compose down` 保留命名卷，再次运行部署脚本可恢复服务。

`docker compose down` preserves named volumes. Run the deployment script again to restore the services.

部署依据 [Milvus 2.5 官方单容器文档](https://milvus.io/docs/v2.5.x/install_standalone-docker.md)及[固定版本的内嵌配置](https://github.com/milvus-io/milvus/blob/v2.5.27/configs/milvus.yaml)，模型服务依据 [Ollama Docker 文档](https://docs.ollama.com/docker)。

Deployment follows the [official Milvus 2.5 single-container documentation](https://milvus.io/docs/v2.5.x/install_standalone-docker.md) and the [embedded configuration for the pinned version](https://github.com/milvus-io/milvus/blob/v2.5.27/configs/milvus.yaml). The model service follows the [Ollama Docker documentation](https://docs.ollama.com/docker).

<a id="本机-web-工作台"></a>

## 本机 Web 工作台 / Local Web workspace

需要 Docker Compose V2。在仓库根目录执行：

Docker Compose V2 is required. Run these commands from the repository root:

```powershell
./scripts/deploy.ps1 -Mode web
# 已有模型缓存时，可另外启动/检查 AI，不下载模型 / With cached models, start/check AI without downloading:
./scripts/deploy.ps1 -Mode web -WithAI
```

```bash
bash scripts/deploy.sh --mode web
bash scripts/deploy.sh --mode web --with-ai
```

默认访问 `http://127.0.0.1:3000`，`WEB_PORT` 可覆盖。部署复用 API 凭据和数据库，等待 PostgreSQL/API/Web 健康；核心就绪不表示 AI 在线。这是私人本机工具，常用启动命令为 `./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'` 或 `bash scripts/deploy.sh --mode web --source-directory '/workspace/project'`。脚本验证并只读挂载源码目录；注册登录后新建/选择项目即可在网页填写该根内完整目录或 `.go` 文件路径，无需管理员、所有者/项目 ID 或独立绑定步骤。详见 [本地源码目录](docs/SOURCE_BINDINGS.md)。后端只读取授权根内内容，结果仍为相对路径。索引任务需要已有 AI 服务及模型；索引子目录也会替换整项目快照。网页不提供上传或 Git 导入。

The default address is `http://127.0.0.1:3000`; override it with `WEB_PORT`. Deployment reuses the API credentials and database and waits for PostgreSQL/API/Web to become healthy. Core readiness does not mean AI is online. This is a private local tool. Typical startup commands are `./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'` or `bash scripts/deploy.sh --mode web --source-directory '/workspace/project'`. The script validates and mounts the source directory read-only. After registering, logging in, and creating or selecting a project, enter a full directory or `.go` file path within that root in the browser. No administrator, owner/project IDs, or separate binding step is needed. See [local source directories](docs/SOURCE_BINDINGS.md). The backend reads only content within the authorized root, and results still use relative paths. Indexing requires existing AI services and models; indexing a subdirectory also replaces the whole project snapshot. The Web workspace does not provide uploads or Git imports.

正式 Web 使用非 root Nginx 同源代理 `/api/*`，支持工作台路由直接刷新，服务密钥不进入静态包。CLI/API 不要求 Web 构建依赖。工作台完整流程见 [Web 说明](web/README.md)，环境配置、代理期限与安全隔离测试见 [部署说明](docs/WEB_DEPLOYMENT.md)。

The production Web deployment uses non-root Nginx to proxy `/api/*` on the same origin and supports direct refreshes of workspace routes. Service secrets do not enter the static bundle. The CLI/API do not require Web build dependencies. See the [Web guide](web/README.md) for the complete workspace flow and the [deployment guide](docs/WEB_DEPLOYMENT.md) for environment configuration, proxy timeouts, and security isolation tests.

<a id="本机-api-部署"></a>

## 本机 API 部署 / Local API deployment

原 CLI 启动命令保持有效。另有独立 API 模式：

The original CLI startup commands remain valid. A separate API mode is also available:

```powershell
./scripts/deploy.ps1 -Mode api
./scripts/deploy.ps1 -Mode api -WithAI
```

```bash
bash scripts/deploy.sh --mode api
bash scripts/deploy.sh --mode api --with-ai
```

默认 API 模式只启动 PostgreSQL 和 API，等待核心就绪后成功退出；AI 离线不影响扫描/分析。可选 AI 模式复用已有 Ollama/Milvus 和缓存模型，不下载模型。首次生成的强服务凭据存于忽略的 `.runtime/api.env`，重复启动复用；源码位于忽略的 `sources/` 并只读挂载。也支持同时通过环境注入 `API_DB_PASSWORD`、`API_JWT_SECRET`。现有凭据文件无效时明确失败，不重置数据库密码。

By default, API mode starts only PostgreSQL and the API, then exits successfully after the core services are ready. Scanning and analysis work while AI is offline. Optional AI mode reuses existing Ollama/Milvus services and cached models without downloading models. Strong service credentials generated on the first run are stored in the ignored `.runtime/api.env` and reused on later starts. Source code resides in the ignored `sources/` directory and is mounted read-only. You can also supply both `API_DB_PASSWORD` and `API_JWT_SECRET` through the environment. An invalid existing credentials file causes an explicit failure; the database password is not reset.

API 默认仅发布到 `http://127.0.0.1:8087`，依赖端口不公开。API 镜像非 root 运行；PostgreSQL 使用独立持久卷。基础 `docker-compose.yml` 不要求 API 密钥；仅 API 模式显式合并 `docker-compose.api.yml`。每个数据库/索引空间只运行一个 API 实例。

By default, the API is published only at `http://127.0.0.1:8087`; dependency ports are not exposed. The API image runs as a non-root user, and PostgreSQL uses a separate persistent volume. The base `docker-compose.yml` does not require API secrets; only API mode explicitly merges `docker-compose.api.yml`. Run only one API instance per database/index space.

完整注册→登录→创建项目→准备源码→扫描→分析/任务查询→索引→问答→取消流程见 [API 使用说明](api/README.md)。没有预置账户；部署时通过 `-SourceDirectory` / `--source-directory` 选择本地根，各项目可输入该根内完整路径；旧相对 API 调用与项目映射继续兼容。绑定变化不自动更新已有索引，需要用户明确确认重新建立索引。接口与中断/发布不确定的语义见 [API 契约](api/CONTRACT.md)。

See the [API guide](api/README.md) for the complete flow: register → log in → create a project → prepare source code → scan → analyze/query tasks → index → ask → cancel. There are no preset accounts. Select a local root at deployment with `-SourceDirectory` / `--source-directory`; projects can accept full paths within that root. Legacy relative API calls and project mappings remain compatible. Binding changes do not automatically update existing indexes; rebuilding the index requires explicit user confirmation. See the [API contract](api/CONTRACT.md) for interface semantics, interruptions, and uncertain publication outcomes.

<a id="索引和问答语义"></a>

## 索引和问答语义 / Indexing and Q&A semantics

索引切块覆盖函数、方法、类型声明（含结构体和接口）、常量、变量和 import，并保留包声明及声明间的注释。超长声明继续拆分；语法解析失败时回退到按源码行分块，不会只保留函数而遗漏其他声明。

Index chunks cover functions, methods, type declarations (including structs and interfaces), constants, variables, and imports, while preserving package declarations and comments between declarations. Long declarations are split further. If syntax parsing fails, chunking falls back to source lines rather than retaining only functions and omitting other declarations.

首次整库索引需要为各代码块生成向量，耗时取决于源码规模、模型和硬件，CPU 运行时可能较长。执行过程中 CLI 可能持续等待；成功以返回索引摘要和退出码 0 为准。可按 Ctrl+C 取消，取消或失败后按需重新执行 `index`。

The first full-repository index must generate an embedding for every code chunk. Duration depends on source size, the model, and hardware; CPU execution can take a long time. The CLI may continue waiting during execution. Success requires an index summary and exit code 0. Press Ctrl+C to cancel, and rerun `index` as needed after cancellation or failure.

一个 collection 对应一个代码库索引；`index` 更新同一 collection 时替换旧内容，避免重复追加。不同项目应使用不同的 `--collection`，问答必须选择对应的 collection。源码变更后重新执行 `index`。

One collection represents one code repository index. Updating the same collection with `index` replaces its old content to avoid duplicate appends. Use different `--collection` values for different projects, and choose the matching collection for Q&A. Run `index` again after source changes.

更新时先把全部内容分批写入独立快照，完成刷新、建索引、加载和条数校验后，再通过 Milvus 内部别名切换查询入口。发布前任何一步失败都保留原索引；发布请求超时时会再次确认入口，无法确认则保留两份数据并报错。已有同名 collection 会作为首次更新前的读取来源，原数据不会被删除。

An update first writes all content in batches to a separate snapshot. After flushing, building the index, loading it, and validating the record count, it switches the query entry point through an internal Milvus alias. Any failure before publication preserves the original index. If the publication request times out, the entry point is checked again; if the outcome cannot be confirmed, both datasets are retained and an error is reported. An existing collection with the same name serves as the read source before the first update; its original data is not deleted.

旧快照保留在数据卷中，更新成功后仅卸载其内存数据，因此历史索引会持续占用磁盘。同一 collection 不要并发索引或在更新期间问答。恢复旧版本及清理的说明见 [使用手册](USER_MANUAL.md#索引数据保护与恢复)，历史管理命令见 [TODO](docs/TODO.md)。

Old snapshots remain in the data volume. After a successful update, only their in-memory data is unloaded, so historical indexes continue using disk space. Do not index the same collection concurrently or ask questions during an update. See the [user manual](USER_MANUAL.md#索引数据保护与恢复) for restoring old versions and cleanup, and [TODO](docs/TODO.md) for planned history management commands.

索引保存源码相对路径、函数和行号等依据，不保存源目录的个人绝对路径。`ask --file` 使用相对于索引根目录的文件路径，例如：

The index stores evidence such as source-relative paths, functions, and line numbers, not personal absolute source-directory paths. `ask --file` takes a file path relative to the index root. The following example asks how the index is updated:

```bash
docker compose run --rm insight ask --collection example_project --file internal/ai/rag_service.go "索引是如何更新的？"
```

回答依赖实际检索到的源码。没有相关上下文时返回明确说明；AI 回答应结合返回的代码来源核对。当前只支持 Ollama；外部模型 API 暂列 TODO。

Answers depend on the source code actually retrieved. If no relevant context is available, the response says so explicitly. Check AI answers against the returned code sources. Only Ollama is currently supported; external model APIs remain on the TODO list.

询问具体实现时，在问题中写出函数或限定名，例如 `Scan`、`NewCLI` 或 `os.Lstat`，有助于优先检索相应定义；`--file` 可进一步限定源码范围。

When asking about a specific implementation, include a function or qualified name such as `Scan`, `NewCLI`, or `os.Lstat` to help prioritize its definition during retrieval. Use `--file` to narrow the source scope further.

<a id="通用配置"></a>

## 通用配置 / General configuration

程序使用内置默认值，可通过 `-c config/config.json` 显式加载通用配置文件。配置优先级为：内置默认值 → 显式配置文件 → 环境变量 → 命令行选项。个人覆盖文件可命名为 `config/config.local.json`，该文件已被忽略；仓库和文档仅保留通用示例。

The program uses built-in defaults. Load a general configuration file explicitly with `-c config/config.json`. Configuration precedence is: built-in defaults → explicit configuration file → environment variables → command-line options. A personal override file can be named `config/config.local.json`, which is ignored by Git. The repository and documentation contain only general examples.

AI 环境变量统一使用 `GO_AI_INSIGHT_` 前缀：

AI environment variables consistently use the `GO_AI_INSIGHT_` prefix:

| 环境变量 / Environment variable | 本地默认值 / Local default |
|---|---|
| `GO_AI_INSIGHT_OLLAMA_ENDPOINT` | `http://localhost:11434` |
| `GO_AI_INSIGHT_MILVUS_ENDPOINT` | `localhost:19530` |
| `GO_AI_INSIGHT_EMBEDDING_MODEL` | `bge-m3:latest` |
| `GO_AI_INSIGHT_CHAT_MODEL` | `llama3:latest` |
| `GO_AI_INSIGHT_COLLECTION` | `code_segments` |
| `GO_AI_INSIGHT_VECTOR_DIM` | `1024` |
| `GO_AI_INSIGHT_TOP_K` | `3` |
| `GO_AI_INSIGHT_CHUNK_MAX_LINES` | `120` |
| `GO_AI_INSIGHT_CHUNK_MIN_LINES` | `5` |
| `GO_AI_INSIGHT_TIMEOUT_SECONDS` | `300` |

Compose 内部使用服务名连接 Ollama 和 Milvus。模型、collection、维度和检索参数可在本地 `.env` 或调用者环境中覆盖。更换模型后重新运行部署脚本准备模型；更换向量模型时必须使用对应维度，并重新索引。不要用新模型直接查询旧模型生成的向量。

Inside Compose, service names connect to Ollama and Milvus. Override models, the collection, dimensions, and retrieval parameters in a local `.env` or the caller's environment. After changing models, rerun the deployment script to prepare them. When changing the embedding model, use its matching dimension and rebuild the index. Do not query vectors generated by the old model directly with the new model.

本地直接执行 `index` / `ask` 时，需要自行准备可连接的 Ollama 和 Milvus；这些依赖的连接配置不会影响 `scan` / `analyze`。

To run `index` / `ask` directly on the host, prepare reachable Ollama and Milvus services yourself. Their connection settings do not affect `scan` / `analyze`.

<a id="输出与退出状态"></a>

## 输出与退出状态 / Output and exit status

默认输出为文本。`-f json` 输出结构化结果：

Text is the default output format. `-f json` produces structured results:

```json
{
  "success": true,
  "result": {
    "files_analyzed": 1,
    "functions_analyzed": 2,
    "complexity_issues": 0,
    "security_issues": 0,
    "bug_issues": 0
  }
}
```

示例省略了文件列表等字段；`result` 是对象，不需要二次解析。日志默认写入 stderr，报告写入 stdout；`-o REPORT` 将报告写入指定文件。命令成功返回 0，输入、配置或服务执行失败返回非零状态，并输出错误信息。

The example omits fields such as the file list. `result` is an object and does not require a second parsing step. Logs go to stderr by default, and reports go to stdout. `-o REPORT` writes the report to the specified file. Successful commands return 0; failures in input, configuration, or service execution return a nonzero status and an error message.

`scan` 包含文件总数、生产/测试文件数、行数和跳过目录；`analyze` 默认分析生产 Go 文件，不分析 `_test.go`。扫描过程跳过符号链接，入口本身为符号链接时返回错误；忽略依赖与产物目录 `.git`、`vendor`、`node_modules`、`dist`、`build`，以及 IDE / agent 工具目录 `.idea`、`.vscode`、`.hermes`、`.qoder`、`.codex`。未被忽略的源码或目录读取失败时，扫描、分析和索引均返回错误，不将缺失数据当成完整结果。代码分块和 Bug/安全问题的来源行号使用文件中的物理行号，不受 `//line` 指令影响。

`scan` reports the total file count, production/test file counts, line counts, and skipped directories. `analyze` analyzes production Go files by default and excludes `_test.go`. Scanning skips symbolic links and returns an error if the entry point itself is a symbolic link. It ignores dependency and output directories `.git`, `vendor`, `node_modules`, `dist`, and `build`, plus IDE / agent tool directories `.idea`, `.vscode`, `.hermes`, `.qoder`, and `.codex`. If a source file or directory that is not ignored cannot be read, scanning, analysis, and indexing all return an error rather than treating missing data as a complete result. Source line numbers for code chunks and bug/security issues are physical line numbers in the file, unaffected by `//line` directives.

<a id="核心架构与开发边界"></a>

## 核心架构与开发边界 / Core architecture and development boundaries

```text
cmd/                  统一 CLI 入口 / Unified CLI entry point
internal/cli/         命令调度与报告输出 / Command dispatch and reports
internal/source/      共享源码发现能力 / Shared source discovery
internal/tools/       AST 静态规则与工具管理 / AST rules and tool management
internal/analysis/    CLI/API 共享静态分析聚合 / Shared CLI/API static analysis aggregation
internal/ai/          源码切块、索引、检索、Ollama 问答 / Source chunking, indexing, retrieval, Ollama Q&A
internal/config/      配置加载、环境覆盖与校验 / Configuration loading, environment overrides, validation
config/config.json    通用配置示例 / General configuration example
scripts/              跨平台部署入口 / Cross-platform deployment entry points
api/                  HTTP、持久任务和认证入口 / HTTP, persistent tasks, authentication
web/                  React 项目工作台及同源 Web 镜像 / React project workspace and same-origin Web image
```

扫描和分析直接调用核心服务；AI 客户端仅在执行 `index` / `ask` / `doctor` 时创建。API 复用根模块服务；Web 通过相对 `/api/v1` 使用同一项目、归属和任务契约。

Scanning and analysis call core services directly. AI clients are created only for `index` / `ask` / `doctor`. The API reuses services from the root module. The Web workspace uses relative `/api/v1` requests and the same project, ownership, and task contracts.

详细命令说明见 [CLI_README.md](CLI_README.md)，使用流程见 [USER_MANUAL.md](USER_MANUAL.md)，后续范围见 [docs/TODO.md](docs/TODO.md)。

See [CLI_README.md](CLI_README.md) for detailed commands, [USER_MANUAL.md](USER_MANUAL.md) for usage flows, and [docs/TODO.md](docs/TODO.md) for future scope.

<a id="验证"></a>

## 验证 / Validation

```bash
go test ./internal/... ./cmd/...
go build -o ./bin/go-ai-insight ./cmd
docker compose --profile cli --profile setup config --quiet
```

Compose 配置可直接验证，无需预先生成本地配置或凭据。配置验证和单元测试不能替代真实的镜像构建、服务启动、源码索引和问答验证。

Compose configuration can be validated directly without generating local configuration or credentials first. Configuration validation and unit tests do not replace real image builds, service startup, source indexing, and Q&A validation.
