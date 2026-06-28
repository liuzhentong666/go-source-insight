# GoSource-Insight

GoSource-Insight 是一个面向 Go 项目的源码分析工具。当前主线能力是 CLI 静态分析：扫描 Go 项目结构，基于 Go AST 执行复杂度、安全风险与常见 Bug 检测，并支持 Text/JSON 输出。

说明：Milvus + Ollama 的 RAG 代码问答链路仍处于实验阶段，后续会拆分为独立的 `index` / `ask` 命令；当前 `scan` 不负责向量入库。

## 当前可用能力

- `scan`：扫描 Go 文件结构，统计生产代码、测试代码、总行数和跳过目录
- `analyze`：对 Go 文件或目录执行静态规则分析
- `complexity`：单文件复杂度分析
- `security`：单文件安全风险检测
- `bug`：常见 Bug 检测
- `test`：生成单元测试
- Text / JSON 两种输出格式
- 统一的错误模型：所有命令在工具执行失败时正确返回非 0 退出码

## 技术栈

- Go 1.25.5
- Go AST
- Gin / GORM / PostgreSQL（Web API 模块）
- React + TypeScript + Vite + Ant Design（Web UI）
- Milvus + Ollama + LangChainGo（实验性 RAG 模块）

## 快速开始：CLI

```bash
git clone <your-repository-url>
cd GoSource-Insight

go test ./internal/...
go run ./cmd list
go run ./cmd scan ./internal/tools
go run ./cmd analyze ./internal/tools
```

JSON 输出：

```bash
go run ./cmd -f json scan ./internal/tools
go run ./cmd -f json analyze ./internal/tools
```

## 命令语义

| 命令 | 当前职责 | 后续关系 |
|---|---|---|
| `scan` | 只扫描项目结构，发现 Go 文件并统计基础信息 | 可被 `analyze` 和未来 `index` 复用 |
| `analyze` | 执行静态规则分析，聚合复杂度/安全/Bug 统计 | 静态分析主入口 |
| `index` | 暂未实现 | 未来负责代码切块、向量化和写入 Milvus |
| `ask` | 暂未实现 | 未来基于已索引代码进行 RAG 问答 |

## scan 示例

```bash
go run ./cmd scan ./internal/cli/commands
```

输出包含：

- `total_files`
- `production_file_count`
- `test_file_count`
- `total_lines`
- `skipped_dirs`
- `production_files`
- `test_files`

`scan` 会跳过这些目录：

- `.git`
- `vendor`
- `node_modules`
- `dist`
- `build`
- `.idea`
- `.vscode`

原因：这些目录不是当前项目源码，纳入扫描会污染统计结果或造成无意义的性能开销。

## analyze 示例

```bash
go run ./cmd analyze ./internal/cli/commands
```

输出包含：

- `files_analyzed`
- `functions_analyzed`
- `complexity_issues`
- `security_issues`
- `bug_issues`
- 每个文件的分析摘要

当前 `analyze` 默认只分析生产 Go 文件，不分析 `_test.go` 文件。测试代码后续可以通过 `--include-tests` 或 `--scope` 参数单独支持。

## 错误模型

所有 CLI 命令统一采用 ToolManager 的双通道错误模型：

- **框架级错误**（ToolManager 内部异常）：`err != nil`，直接返回
- **工具级失败**（工具执行失败，如非法 Go 文件）：`result.Success == false`，由 `result.Error` 携带错误信息

每个命令在 `err != nil` 检查后，追加 `!result.Success` 检查，确保工具失败时正确退出：

```go
result, err := tm.Run(ctx, "tool_name", input)
if err != nil {
    return fmt.Errorf("...: %w", err)
}
if !result.Success {
    return fmt.Errorf("...: %s", result.Error)
}
```

已验证行为：

- 正常 Go 文件 → exit 0，输出 JSON 结果
- 非 Go 文件 / 非法 Go 代码 → exit 1，输出错误信息
- 文件不存在 → exit 1，输出错误信息

## Web API / Web UI

Web API 与 Web UI 仍保留，但不是当前第一轮升级的主线。API 模块位于 `api/`，前端位于 `web/`。

API 环境变量示例见：

```bash
api/.env.example
```

不要提交真实 `.env` 文件。

## 配置示例

`api/.env.example`：

```env
PORT=8087
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=change...
DB_NAME=goaiinsight
JWT_SECRET=change...
```

## 项目结构

```text
GoSource-Insight/
├── api/                  # Web API 服务
├── cmd/                  # CLI 入口
├── internal/
│   ├── ai/               # 实验性 RAG/LLM 代码问答模块
│   ├── cli/              # CLI 命令、输出格式化
│   └── tools/            # 静态分析工具和规则引擎
├── web/                  # Web UI
├── config/               # CLI 配置
└── docker-compose.yml
```

## 验证命令

```bash
go test ./internal/...           # 全量测试（当前 63 PASS）
go build -o /dev/null ./cmd      # 编译检查
go run ./cmd scan ./internal/cli/commands
go run ./cmd analyze ./internal/cli/commands
```

## 最近变更

### w9 — bug 命令错误传播修复

- `BugDetector.Run()`：全部 Go 文件解析失败时返回 error（保持部分失败时返回 partial JSON 的语义）
- `bug` 命令：补 `!result.Success` 检查，与非 Go 文件/非法代码输入时正确返回 exit 1
- 所有命令的错误模型现在一致（security / complexity / bug / test）
- analyze 路径无影响：非法 Go 文件由前置的 complexity_analyzer 先拦截

### w7-w8 — CLI 错误模型统一

- `security`、`complexity` 命令补 `!result.Success` 检查
- `scan` 修复 `walkErr` 中止扫描的问题
- `isGoFile` 改为严格匹配 `.go` 后缀
- ToolManager 在 `execErr != nil` 时正确设置 `Success=false`

## 当前阶段边界

当前项目仍在演进中：

- CLI 静态扫描与分析是当前主线
- RAG 代码问答仍是实验模块
- Web UI 是展示入口，不作为当前核心验收链路
- 生产环境部署、权限隔离、多租户和增量索引属于后续阶段
