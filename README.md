# GoSource-Insight

GoSource-Insight 提供 Go 源码扫描、静态分析，以及基于 Ollama 和 Milvus 的代码问答。CLI、HTTP API 和 Web 工作台复用核心服务；扫描和静态分析可独立运行。Web 支持本机项目工作台，外部模型供应商 API 仍暂缓。

## 核心命令

| 命令 | 职责 | 依赖 |
|---|---|---|
| `scan PATH` | 发现 Go 文件，统计生产代码、测试代码和行数 | 无 AI 服务依赖 |
| `analyze PATH` | 对生产 Go 文件执行复杂度、安全和常见 Bug 规则 | 无 AI 服务依赖 |
| `index [--collection NAME] PATH` | 扫描、按 Go 声明切块、向量化，更新代码库索引 | Ollama、Milvus |
| `ask [--collection NAME] [--file RELATIVE_PATH] QUESTION` | 检索已索引代码，回答问题并返回源码依据 | Ollama、Milvus |

`complexity`、`security`、`bug` 是单项检查入口；`test` 目前生成测试骨架，不属于核心验收功能。静态规则结果是待检查的线索，不等同于已确认缺陷。

## 直接运行扫描和分析

需要 Go 1.25.5 或兼容工具链。在项目根目录执行：

```bash
go run ./cmd help
go run ./cmd scan ./internal
go run ./cmd analyze ./internal
go run ./cmd -f json -o analysis.json analyze ./internal
```

也可以构建统一的 CLI：

```bash
go build -o go-ai-insight ./cmd
./go-ai-insight scan ./internal
```

Windows 可构建为 `go-ai-insight.exe`。全局选项放在命令前；`index` 和 `ask` 的命令选项放在命令后。

## 一键准备完整问答环境

需要已启动的 Docker 引擎和 Docker Compose V2，运行 Linux 容器。首次运行需要联网下载镜像和模型，并预留模型权重、向量数据所需的内存和磁盘空间。默认使用 CPU；GPU 配置作为后续增强。

PowerShell：

```powershell
./scripts/deploy.ps1
```

Linux / macOS：

```bash
bash scripts/deploy.sh
```

脚本会检查 Docker、构建 CLI 镜像、等待 Ollama / Milvus 健康，再通过一次性 `models` 服务准备 `bge-m3:latest` 和 `llama3:latest`。已安装模型直接复用，仅下载缺失模型；重复执行时不会为已有模型联网检查更新。最后执行 `doctor` 检查模型实际向量维度和 Milvus 连接，成功后才提示部署就绪；检查不会创建源码索引。模型下载显示进度；任一步失败，脚本返回非零状态，修复后可重新运行。

Milvus 采用官方单容器方案，内嵌 etcd 并使用本地存储，减少运行依赖。模型和数据库使用命名卷持久化；默认不向宿主机发布依赖服务端口。部署脚本不写入本地 `.env` 或修改配置；`.env` 可按需提供通用参数覆盖，并已被忽略。

完成后，在项目根目录运行：

```bash
docker compose run --rm insight scan /workspace
docker compose run --rm insight analyze /workspace
docker compose run --rm insight index --collection example_project /workspace
docker compose run --rm insight ask --collection example_project "源码扫描和静态分析如何衔接？"
```

`insight` 是运行后退出的 CLI 服务，默认命令为 `help`。`/workspace` 默认只读挂载当前项目。分析其他目录时，仅通过本地环境变量设置通用挂载入口；例如：

```powershell
$env:SOURCE_DIR = './sample-project'
docker compose run --rm insight index --collection sample_project /workspace
```

```bash
SOURCE_DIR=./sample-project docker compose run --rm insight index --collection sample_project /workspace
```

相对挂载路径以 Compose 文件所在目录为基准。之后的扫描或索引命令使用同样的 `SOURCE_DIR`。问答读取向量库，不需要重新扫描目录。

容器的源码挂载只读。需要导出 JSON 报告时，可由宿主机接收输出：

```bash
docker compose run --rm -T insight -f json analyze /workspace > analysis.json
```

服务检查和停止：

```bash
docker compose ps
docker compose run --rm -T insight -f json doctor
docker compose logs --tail 100 milvus ollama
docker compose down
```

`docker compose down` 保留命名卷，再次运行部署脚本可恢复服务。

部署依据 [Milvus 2.5 官方单容器文档](https://milvus.io/docs/v2.5.x/install_standalone-docker.md)及[固定版本的内嵌配置](https://github.com/milvus-io/milvus/blob/v2.5.27/configs/milvus.yaml)，模型服务依据 [Ollama Docker 文档](https://docs.ollama.com/docker)。

## 本机 Web 工作台

需要 Docker Compose V2。在仓库根目录执行：

```powershell
./scripts/deploy.ps1 -Mode web
# 已有模型缓存时，可另外启动/检查 AI，不下载模型：
./scripts/deploy.ps1 -Mode web -WithAI
```

```bash
bash scripts/deploy.sh --mode web
bash scripts/deploy.sh --mode web --with-ai
```

默认访问 `http://127.0.0.1:3000`，`WEB_PORT` 可覆盖。部署复用 API 凭据和数据库，等待 PostgreSQL/API/Web 健康；核心就绪不表示 AI 在线。这是私人本机工具，常用启动命令为 `./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'` 或 `bash scripts/deploy.sh --mode web --source-directory '/workspace/project'`。脚本验证并只读挂载源码目录；注册登录后新建/选择项目即可在网页填写该根内完整目录或 `.go` 文件路径，无需管理员、所有者/项目 ID 或独立绑定步骤。详见 [本地源码目录](docs/SOURCE_BINDINGS.md)。后端只读取授权根内内容，结果仍为相对路径。索引任务需要已有 AI 服务及模型；索引子目录也会替换整项目快照。网页不提供上传或 Git 导入。

正式 Web 使用非 root Nginx 同源代理 `/api/*`，支持工作台路由直接刷新，服务密钥不进入静态包。CLI/API 不要求 Web 构建依赖。工作台完整流程见 [Web 说明](web/README.md)，环境配置、代理期限与安全隔离测试见 [部署说明](docs/WEB_DEPLOYMENT.md)。

## 本机 API 部署

原 CLI 启动命令保持有效。另有独立 API 模式：

```powershell
./scripts/deploy.ps1 -Mode api
./scripts/deploy.ps1 -Mode api -WithAI
```

```bash
bash scripts/deploy.sh --mode api
bash scripts/deploy.sh --mode api --with-ai
```

默认 API 模式只启动 PostgreSQL 和 API，等待核心就绪后成功退出；AI 离线不影响扫描/分析。可选 AI 模式复用已有 Ollama/Milvus 和缓存模型，不下载模型。首次生成的强服务凭据存于忽略的 `.runtime/api.env`，重复启动复用；源码位于忽略的 `sources/` 并只读挂载。也支持同时通过环境注入 `API_DB_PASSWORD`、`API_JWT_SECRET`。现有凭据文件无效时明确失败，不重置数据库密码。

API 默认仅发布到 `http://127.0.0.1:8087`，依赖端口不公开。API 镜像非 root 运行；PostgreSQL 使用独立持久卷。基础 `docker-compose.yml` 不要求 API 密钥；仅 API 模式显式合并 `docker-compose.api.yml`。每个数据库/索引空间只运行一个 API 实例。

完整注册→登录→创建项目→准备源码→扫描→分析/任务查询→索引→问答→取消流程见 [API 使用说明](api/README.md)。没有预置账户；部署时通过 `-SourceDirectory` / `--source-directory` 选择本地根，各项目可输入该根内完整路径；旧相对 API 调用与项目映射继续兼容。绑定变化不自动更新已有索引，需要用户明确确认重新建立索引。接口与中断/发布不确定的语义见 [API 契约](api/CONTRACT.md)。

## 索引和问答语义

索引切块覆盖函数、方法、类型声明（含结构体和接口）、常量、变量和 import，并保留包声明及声明间的注释。超长声明继续拆分；语法解析失败时回退到按源码行分块，不会只保留函数而遗漏其他声明。

首次整库索引需要为各代码块生成向量，耗时取决于源码规模、模型和硬件，CPU 运行时可能较长。执行过程中 CLI 可能持续等待；成功以返回索引摘要和退出码 0 为准。可按 Ctrl+C 取消，取消或失败后按需重新执行 `index`。

一个 collection 对应一个代码库索引；`index` 更新同一 collection 时替换旧内容，避免重复追加。不同项目应使用不同的 `--collection`，问答必须选择对应的 collection。源码变更后重新执行 `index`。

更新时先把全部内容分批写入独立快照，完成刷新、建索引、加载和条数校验后，再通过 Milvus 内部别名切换查询入口。发布前任何一步失败都保留原索引；发布请求超时时会再次确认入口，无法确认则保留两份数据并报错。已有同名 collection 会作为首次更新前的读取来源，原数据不会被删除。

旧快照保留在数据卷中，更新成功后仅卸载其内存数据，因此历史索引会持续占用磁盘。同一 collection 不要并发索引或在更新期间问答。恢复旧版本及清理的说明见 [使用手册](USER_MANUAL.md#索引数据保护与恢复)，历史管理命令见 [TODO](docs/TODO.md)。

索引保存源码相对路径、函数和行号等依据，不保存源目录的个人绝对路径。`ask --file` 使用相对于索引根目录的文件路径，例如：

```bash
docker compose run --rm insight ask --collection example_project --file internal/ai/rag_service.go "索引是如何更新的？"
```

回答依赖实际检索到的源码。没有相关上下文时返回明确说明；AI 回答应结合返回的代码来源核对。当前只支持 Ollama；外部模型 API 暂列 TODO。

询问具体实现时，在问题中写出函数或限定名，例如 `Scan`、`NewCLI` 或 `os.Lstat`，有助于优先检索相应定义；`--file` 可进一步限定源码范围。

## 通用配置

程序使用内置默认值，可通过 `-c config/config.json` 显式加载通用配置文件。配置优先级为：内置默认值 → 显式配置文件 → 环境变量 → 命令行选项。个人覆盖文件可命名为 `config/config.local.json`，该文件已被忽略；仓库和文档仅保留通用示例。

AI 环境变量统一使用 `GO_AI_INSIGHT_` 前缀：

| 环境变量 | 本地默认值 |
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

本地直接执行 `index` / `ask` 时，需要自行准备可连接的 Ollama 和 Milvus；这些依赖的连接配置不会影响 `scan` / `analyze`。

## 输出与退出状态

默认输出为文本。`-f json` 输出结构化结果：

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

`scan` 包含文件总数、生产/测试文件数、行数和跳过目录；`analyze` 默认分析生产 Go 文件，不分析 `_test.go`。扫描过程跳过符号链接，入口本身为符号链接时返回错误；忽略依赖与产物目录 `.git`、`vendor`、`node_modules`、`dist`、`build`，以及 IDE / agent 工具目录 `.idea`、`.vscode`、`.hermes`、`.qoder`、`.codex`。未被忽略的源码或目录读取失败时，扫描、分析和索引均返回错误，不将缺失数据当成完整结果。代码分块和 Bug/安全问题的来源行号使用文件中的物理行号，不受 `//line` 指令影响。

## 核心架构与开发边界

```text
cmd/                  统一 CLI 入口
internal/cli/         命令调度与报告输出
internal/source/      共享源码发现能力
internal/tools/       AST 静态规则与工具管理
internal/analysis/    CLI/API 共享静态分析聚合
internal/ai/          源码切块、索引、检索、Ollama 问答
internal/config/      配置加载、环境覆盖与校验
config/config.json    通用配置示例
scripts/              跨平台部署入口
api/                  HTTP、持久任务和认证入口
web/                  React 项目工作台及同源 Web 镜像
```

扫描和分析直接调用核心服务；AI 客户端仅在执行 `index` / `ask` / `doctor` 时创建。API 复用根模块服务；Web 通过相对 `/api/v1` 使用同一项目、归属和任务契约。

详细命令说明见 [CLI_README.md](CLI_README.md)，使用流程见 [USER_MANUAL.md](USER_MANUAL.md)，后续范围见 [docs/TODO.md](docs/TODO.md)。

## 验证

```bash
go test ./internal/... ./cmd/...
go build -o ./bin/go-ai-insight ./cmd
docker compose --profile cli --profile setup config --quiet
```

Compose 配置可直接验证，无需预先生成本地配置或凭据。配置验证和单元测试不能替代真实的镜像构建、服务启动、源码索引和问答验证。
