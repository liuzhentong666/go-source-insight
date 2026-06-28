# GoSource-Insight CLI

GoSource-Insight CLI 是当前项目的主线入口，用于对 Go 项目执行结构扫描和静态规则分析。

## 安装 / 运行

```bash
# 直接运行
go run ./cmd list

# 构建二进制
go build -o go-ai-insight ./cmd
./go-ai-insight list
```

## 命令

```bash
# 查看帮助
go run ./cmd

# 列出所有可用工具
go run ./cmd list

# 扫描项目结构：只统计 Go 文件，不执行规则分析
go run ./cmd scan ./internal/tools

# 静态分析：聚合复杂度、安全风险、Bug 检测
go run ./cmd analyze ./internal/tools

# 单项工具
go run ./cmd complexity ./internal/tools/security_scanner.go
go run ./cmd security ./internal/tools/security_scanner.go
go run ./cmd bug ./internal/tools/bug_detector.go
```

## scan 与 analyze 的区别

| 命令 | 职责 |
|---|---|
| `scan` | 文件发现和结构统计，不做静态规则分析，不写入向量库 |
| `analyze` | 复用文件发现能力，对生产 Go 文件执行复杂度、安全和 Bug 分析 |

未来 RAG 能力会新增独立命令：

| 命令 | 计划职责 |
|---|---|
| `index` | 代码切块、向量化、写入 Milvus |
| `ask` | 基于已索引代码进行问答 |

## 全局选项

```bash
-c, --config <file>   配置文件路径
-f, --format <format> 输出格式 (json|text)
-o, --output <file>   输出文件路径
-v, --verbose         详细输出
--version             显示版本信息
```

注意：当前 CLI 使用 Go 标准库 `flag`，全局参数需要放在命令前：

```bash
go run ./cmd -f json scan ./internal/tools
go run ./cmd -f json analyze ./internal/tools
```

## 输出格式

### Text 默认输出

```bash
go run ./cmd scan ./internal/tools
```

### JSON 输出

```bash
go run ./cmd -f json scan ./internal/tools
go run ./cmd -f json analyze ./internal/tools
```

## scan 输出字段

- `target_path`
- `total_files`
- `production_file_count`
- `test_file_count`
- `total_lines`
- `skipped_dirs`
- `files`
- `production_files`
- `test_files`

`scan` 会跳过：

- `.git`
- `vendor`
- `node_modules`
- `dist`
- `build`
- `.idea`
- `.vscode`

## analyze 输出字段

- `target_path`
- `files_analyzed`
- `functions_analyzed`
- `complexity_issues`
- `security_issues`
- `bug_issues`
- `files`

当前 `analyze` 默认只分析生产 Go 文件，不分析 `_test.go` 文件。

## 验证

```bash
go test ./internal/cli/commands ./internal/tools
go run ./cmd scan ./internal/cli/commands
go run ./cmd analyze ./internal/cli/commands
go run ./cmd -f json scan ./internal/cli/commands
```
