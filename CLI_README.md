> 本机完整路径：原生 CLI 可直接扫描/分析任意本机可读目录或 Go 文件。Docker CLI 使用 `./scripts/insight.ps1 scan 'C:\code\project'` / `bash scripts/insight.sh scan '/workspace/project'`，将 `scan` 换成 `analyze` 可分析；入口自动准备宿主读取服务，无需固定源码根。索引与问答仍需要现有 Ollama/Milvus 服务及匹配模型。读取结果和问答文件过滤仍使用相对路径。

# GoSource-Insight CLI

统一入口为 `go run ./cmd` 或构建后的 `go-ai-insight`。核心命令为 `scan`、`analyze`、`index`、`ask`；Docker 部署步骤见 [README.md](README.md)。

## 命令

```bash
go run ./cmd help
go run ./cmd list
go run ./cmd scan ./internal
go run ./cmd analyze ./internal

# 已准备 Ollama 和 Milvus 时
go run ./cmd doctor
go run ./cmd index --collection example_project ./internal
go run ./cmd ask --collection example_project "源码扫描有哪些忽略规则？"
go run ./cmd ask --collection example_project --file ai/rag_service.go "如何检索源码？"
```

`--file` 相对于 `index` 的根目录。上例索引 `./internal`，因此过滤路径为 `ai/rag_service.go`。不传 `--file` 时检索整个 collection。

| 命令 | 使用方式 | 说明 |
|---|---|---|
| `scan` | `scan PATH` | 单文件或目录扫描，不调用模型或写入向量库 |
| `analyze` | `analyze PATH` | 聚合复杂度、安全风险和 Bug 规则，默认仅生产 Go 文件 |
| `index` | `index [--collection NAME] PATH` | 替换当前 collection 的代码索引 |
| `ask` | `ask [--collection NAME] [--file RELATIVE_PATH] QUESTION` | 基于索引回答并提供代码来源 |
| `doctor` | `doctor` | 检查 Ollama 模型、实际向量维度和 Milvus 连接，不创建索引 |
| `complexity` | `complexity FILE` | 单文件复杂度分析 |
| `security` | `security FILE` | 单文件安全规则检查 |
| `bug` | `bug FILE` | 常见 Bug 规则检查 |
| `test` | `test FILE` | 生成测试骨架，需要手动补全断言 |

多词问题建议加引号。collection 名仅使用字母、数字和下划线，并以字母或下划线开头。一个 collection 管理一个代码库；不同项目使用不同 collection，索引后问答使用同名 collection。

## 全局选项

全局选项放在命令前：

| 选项 | 用途 |
|---|---|
| `-c` / `--config FILE` | 显式加载 JSON 配置 |
| `-f` / `--format text\|json` | 报告格式 |
| `-o` / `--output FILE` | 将报告写入文件 |
| `-v` / `--verbose` | 详细输出 |
| `--version` | 版本信息 |
| `-log-level LEVEL` | 日志级别：debug / info / warn / error |
| `-log-format FORMAT` | 日志格式：text / json |
| `-log-output OUTPUT` | 日志目的地：stderr / file |
| `-log-file FILE` | 文件日志路径 |

```bash
go run ./cmd -f json scan ./internal
go run ./cmd -f json -o analysis.json analyze ./internal
go run ./cmd -c config/config.json index --collection example_project ./internal
```

日志默认写入 stderr。JSON 报告的 `result` 是结构化对象，例如：

```json
{"success": true, "result": {"total_files": 2, "production_file_count": 1, "test_file_count": 1}}
```

示例省略其他字段。`-o` 写入报告文件；stdout 保留给报告，日志不会混入 JSON。执行失败返回非零状态，错误写入 stderr。

## 核心报告

`scan` 的 `result` 包含 `target_path`、`total_files`、`production_file_count`、`test_file_count`、`total_lines`、`skipped_dirs`、`files`、`production_files`、`test_files`。

扫描过程跳过符号链接，拒绝符号链接入口；忽略 `.git`、`vendor`、`node_modules`、`dist`、`build`、`.idea`、`.vscode`、`.hermes`、`.qoder`、`.codex` 目录。

`analyze` 包含 `target_path`、`files_analyzed`、`functions_analyzed`、`complexity_issues`、`security_issues`、`bug_issues` 和 `files`。规则命中表示需要进一步检查，不代表已确认缺陷。

`index` 返回索引摘要。重复执行同一 collection 会先分批写入新快照，刷新、建索引、加载并验证条数后切换内部查询别名；发布前失败保留原索引。发布结果无法确认时保留新旧数据并报错。已有 collection 及历史快照保留在数据卷中，需预留磁盘空间。源码删除和修改后需重新索引，同一 collection 请勿并发索引或在更新期间问答。`ask` 返回回答和来源；源码依据保存为索引根目录内的相对路径，不写入个人绝对路径。

源码或目录读取失败时，所有核心扫描入口均返回错误，索引不会发布不完整的扫描结果。分块及 Bug/安全问题使用未经 `//line` 调整的物理行号。

索引按完整 Go 声明切块，涵盖函数、方法、类型、常量、变量、import，以及包声明和注释。超长内容继续拆分；语法解析失败时回退到按源码行分块。

## 配置

`config/config.json` 是通用示例；不指定 `-c` 时使用内置默认值。环境变量覆盖配置文件，命令行选项覆盖对应环境配置。配置覆盖仅放入本地环境或已忽略的 `config/config.local.json`。

AI 参数使用 `GO_AI_INSIGHT_` 前缀：`OLLAMA_ENDPOINT`、`MILVUS_ENDPOINT`、`EMBEDDING_MODEL`、`CHAT_MODEL`、`COLLECTION`、`VECTOR_DIM`、`TOP_K`、`CHUNK_MAX_LINES`、`CHUNK_MIN_LINES`、`TIMEOUT_SECONDS`。完整默认值见 [README.md](README.md#通用配置)。

扫描和静态分析不要求连接 AI 服务。索引和问答需要 Ollama 对应模型已经下载、Milvus 已就绪；向量模型输出维度必须与 `VECTOR_DIM` 一致。一键部署会复用已安装模型，仅下载缺失模型，并使用 `doctor` 校验。

## 容器运行

```bash
docker compose run --rm insight scan /workspace
docker compose run --rm -T insight -f json doctor
docker compose run --rm insight analyze /workspace
docker compose run --rm insight index --collection example_project /workspace
docker compose run --rm insight ask --collection example_project "项目入口在哪里？"
docker compose run --rm -T insight -f json analyze /workspace > analysis.json
```

`/workspace` 为只读源码挂载。容器内的报告导出推荐由宿主机接收 stdout。AI 问答当前仅支持 Ollama；外部模型 API 和 Web 平台见 [TODO](docs/TODO.md)。
