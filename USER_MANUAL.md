# GoSource-Insight 使用文档

## 目录

1. [项目概述](#项目概述)
2. [项目结构](#项目结构)
3. [文件说明](#文件说明)
4. [快速开始](#快速开始)
5. [命令详解](#命令详解)
6. [使用示例](#使用示例)
7. [配置说明](#配置说明)
8. [输出格式](#输出格式)
9. [常见问题](#常见问题)
10. [开发指南](#开发指南)

---

## 项目概述

GoSource-Insight 是一个基于 Go 语言开发的代码分析和测试工具，提供以下功能：

- **代码复杂度分析** - 识别过于复杂的函数
- **安全漏洞扫描** - 检测常见安全问题
- **Bug 检测** - 识别常见编程错误
- **单元测试生成** - 自动生成 Table-driven 测试代码
- **命令行接口** - 统一的 CLI 工具

---

## 项目结构

```
go-ai-study/
├── cmd/
│   └── main.go                  # 主程序入口
├── internal/
│   ├── cli/                     # CLI 命令行工具
│   │   ├── cli.go              # CLI 核心结构
│   │   ├── commands/           # 命令实现
│   │   │   ├── command.go      # 命令接口定义
│   │   │   ├── analyze.go      # 分析命令
│   │   │   ├── test.go         # 测试生成命令
│   │   │   ├── security.go     # 安全扫描命令
│   │   │   ├── bug.go          # Bug 检测命令
│   │   │   ├── complexity.go   # 复杂度分析命令
│   │   │   ├── scan.go         # 扫描命令（未实现）
│   │   │   └── list.go         # 列出命令
│   │   └── output/             # 输出格式化
│   │       ├── formatter.go    # 格式化接口
│   │       ├── json.go         # JSON 格式化器
│   │       └── text.go         # 文本格式化器
│   ├── config/                  # 配置管理
│   │   └── config.go           # 配置加载和保存
│   └── tools/                   # 分析工具实现
│       ├── base_tool.go        # 工具基础实现
│       ├── tool.go             # 工具接口定义
│       ├── tool_manager.go     # 工具管理器
│       ├── tool_manager_test.go # 工具管理器测试
│       ├── logger.go           # 日志系统
│       ├── errors.go           # 错误定义
│       ├── complexity_analyzer.go      # 复杂度分析器
│       ├── complexity_analyzer_test.go # 复杂度分析器测试
│       ├── security_scanner.go         # 安全扫描器
│       ├── security_scanner_test.go    # 安全扫描器测试
│       ├── bug_detector.go             # Bug 检测器
│       ├── bug_detector_test.go        # Bug 检测器测试
│       ├── test_generator.go            # 测试生成器
│       └── test_generator_test.go       # 测试生成器测试
├── config/
│   └── config.json              # 默认配置文件
├── memory/
│   └── *.md                    # 学习记录
├── README.md                    # 项目说明
├── CLI_README.md                # CLI 使用文档
├── go.mod                       # Go 模块定义
├── go.sum                       # Go 依赖锁定
└── docker-compose.yml           # Docker 编排配置
```

---

## 文件说明

### 主程序

#### `cmd/main.go`
- **作用**: 程序的主入口点
- **功能**: 解析命令行参数，创建 CLI 实例，执行命令
- **关键代码**:
  ```go
  func main() {
      // 解析全局参数
      // 创建 CLI
      // 执行命令
  }
  ```

### CLI 核心

#### `internal/cli/cli.go`
- **作用**: CLI 核心结构，管理所有命令
- **功能**:
  - 创建 ToolManager
  - 注册所有工具和命令
  - 执行用户命令
  - 显示帮助信息
- **关键结构**:
  ```go
  type CLI struct {
      toolManager    *tools.ToolManager
      commandRegistry *commands.CommandRegistry
      config         *config.Config
      formatter      output.Formatter
  }
  ```

#### `internal/cli/commands/command.go`
- **作用**: 定义命令接口和命令注册表
- **功能**:
  - 定义 `Command` 接口
  - 提供命令注册功能
  - 列出所有已注册命令
- **关键接口**:
  ```go
  type Command interface {
      Name() string
      Description() string
      Run(ctx context.Context, args []string, formatter output.Formatter) error
  }
  ```

### 命令实现

#### `internal/cli/commands/analyze.go`
- **作用**: 分析命令，调用多个工具进行综合分析
- **功能**: 执行复杂度分析（可扩展其他分析）
- **使用**: `go-ai-insight analyze <file>`
- **输出**: 分析结果（文本或 JSON）

#### `internal/cli/commands/test.go`
- **作用**: 测试生成命令，调用测试生成器
- **功能**: 生成 Table-driven 单元测试
- **使用**: `go-ai-insight test <file>`
- **输出**: 生成的测试文件路径

#### `internal/cli/commands/security.go`
- **作用**: 安全扫描命令，调用安全扫描器
- **功能**: 检测安全漏洞
- **使用**: `go-ai-insight security <file>`
- **输出**: 安全报告

#### `internal/cli/commands/bug.go`
- **作用**: Bug 检测命令，调用 Bug 检测器
- **功能**: 识别常见编程错误
- **使用**: `go-ai-insight bug <file>`
- **输出**: Bug 报告

#### `internal/cli/commands/complexity.go`
- **作用**: 复杂度分析命令，调用复杂度分析器
- **功能**: 分析代码圈复杂度
- **使用**: `go-ai-insight complexity <file>`
- **输出**: 复杂度报告

#### `internal/cli/commands/scan.go`
- **作用**: 扫描命令，将代码存储到向量数据库
- **功能**: 代码扫描和存储（暂未实现）
- **使用**: `go-ai-insight scan <path>`

#### `internal/cli/commands/list.go`
- **作用**: 列出所有可用命令
- **功能**: 显示命令名称和描述
- **使用**: `go-ai-insight list`

### 输出格式化

#### `internal/cli/output/formatter.go`
- **作用**: 定义输出格式化接口
- **功能**:
  - 定义 `Formatter` 接口
  - 定义格式化选项
- **关键接口**:
  ```go
  type Formatter interface {
      Format(result string) string
  }
  ```

#### `internal/cli/output/json.go`
- **作用**: JSON 格式化器
- **功能**: 将结果格式化为 JSON
- **使用**: `-f json`

#### `internal/cli/output/text.go`
- **作用**: 文本格式化器
- **功能**: 将结果格式化为易读的纯文本
- **使用**: `-f text`（默认）

### 配置管理

#### `internal/config/config.go`
- **作用**: 配置加载和保存
- **功能**:
  - 从文件加载配置
  - 从环境变量加载配置
  - 保存配置到文件
- **配置项**:
  ```go
  type Config struct {
      DefaultOutput  string
      DefaultFormat  string
      Verbose        bool
      OllamaEndpoint string
      MilvusEndpoint string
  }
  ```

### 分析工具

#### `internal/tools/base_tool.go`
- **作用**: 工具的基础实现
- **功能**: 提供 Tool 接口的基础功能
- **关键方法**: `Name()`, `Description()`, `Validate()`

#### `internal/tools/tool.go`
- **作用**: 定义工具接口
- **功能**:
  - 定义 `Tool` 接口
  - 定义 `ToolResult` 结构
- **关键接口**:
  ```go
  type Tool interface {
      Name() string
      Description() string
      InputType() reflect.Type
      Validate(input any) error
      Run(ctx context.Context, input any) (string, error)
  }
  ```

#### `internal/tools/tool_manager.go`
- **作用**: 工具管理器，统一管理所有工具
- **功能**:
  - 注册工具
  - 获取工具
  - 执行工具（带超时和重试）
  - 列出所有工具
- **关键方法**: `Register()`, `Get()`, `Run()`, `List()`

#### `internal/tools/complexity_analyzer.go`
- **作用**: 代码复杂度分析器
- **功能**:
  - 计算圈复杂度
  - 识别复杂函数
  - 提供重构建议
- **指标**:
  - 圈复杂度（Cyclomatic Complexity）
  - 函数行数
  - 问题列表

#### `internal/tools/security_scanner.go`
- **作用**: 安全漏洞扫描器
- **功能**:
  - 检测硬编码密钥
  - 检测 SQL 注入风险
  - 检测 XSS 漏洞
  - 检测不安全的随机数生成
- **扫描规则**:
  - 硬编码密钥
  - SQL 注入
  - 命令注入
  - XSS
  - 不安全的随机数
  - 不安全的文件操作
  - 不安全的 HTTP 请求

#### `internal/tools/bug_detector.go`
- **作用**: Bug 检测器
- **功能**:
  - 检测空指针解引用
  - 检测资源泄漏
  - 检测整数溢出
  - 检测字符串比较错误
- **检测类型**:
  - Null Safety
  - Resource Management
  - Error Handling
  - Logic Errors

#### `internal/tools/test_generator.go`
- **作用**: 单元测试生成器
- **功能**:
  - 解析函数签名
  - 生成 Table-driven 测试代码
  - 生成 Mock 建议
  - 生成覆盖率报告
- **测试模式**:
  - `basic` - 基本测试
  - `table-driven` - 表驱动测试（推荐）
  - `mock` - Mock 测试

---

## 快速开始

### 1. 编译程序

```bash
cd /mnt/f/go-ai-study
go build -o go-ai-insight ./cmd
```

### 2. 查看帮助

```bash
./go-ai-insight
```

**理想输出**:
```
go-ai-insight - Go 代码分析和测试工具

使用:
  go-ai-insight <command> [options]

命令:
  scan        扫描代码并存储
  analyze     分析代码
  test        生成测试
  security    安全扫描
  bug         Bug 检测
  complexity  复杂度分析
  list        列出所有可用工具

全局选项:
  -c, --config <file>   配置文件路径
  -f, --format <format> 输出格式 (json|text)
  -o, --output <file>   输出文件路径
  -v, --verbose         详细输出
  --version             显示版本信息
```

### 3. 列出所有工具

```bash
./go-ai-insight list
```

**理想输出**:
```
可用命令:
  scan         扫描代码并存储到向量数据库
  list         列出所有可用工具
  analyze      分析代码并提供智能建议
  test         生成单元测试
  security     安全漏洞扫描
  bug          常见 Bug 检测
  complexity   代码复杂度分析
```

---

## 命令详解

### analyze - 分析命令

**语法**: `go-ai-insight analyze <file> [options]`

**描述**: 分析代码并提供智能建议

**参数**:
- `<file>` - 要分析的 Go 文件路径

**选项**:
- `-f, --format` - 输出格式（json|text）
- `-v, --verbose` - 详细输出

**使用示例**:
```bash
./go-ai-insight analyze ./mycode.go
./go-ai-insight analyze ./mycode.go -f json
./go-ai-insight analyze ./mycode.go -v
```

**理想输出（文本格式）**:
```
{
  "file": "",
  "total": 25,
  "functions": [
    {
      "name": "main",
      "line": 10,
      "complexity": 3,
      "lines": 15,
      "issues": null
    },
    ...
  ],
  "summary": "代码质量良好"
}
```

---

### test - 测试生成命令

**语法**: `go-ai-insight test <file> [options]`

**描述**: 为 Go 代码自动生成单元测试

**参数**:
- `<file>` - 要生成测试的 Go 文件路径

**选项**:
- `-f, --format` - 输出格式（json|text）
- `-v, --verbose` - 详细输出

**使用示例**:
```bash
./go-ai-insight test ./mycode.go
./go-ai-insight test ./mycode.go -f json
```

**理想输出**:
```
[SUCCESS] 测试生成成功

生成的测试文件数: 1
测试用例总数: 3

文件:
   - ./mycode_test.go
```

**生成的测试文件示例**:
```go
func TestAdd(t *testing.T) {
	type args struct {
		a int
		b int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "TODO: 测试用例描述",
			args: args{TODO_a, TODO_b},
			want: TODO_int,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Add(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
```

---

### security - 安全扫描命令

**语法**: `go-ai-insight security <file> [options]`

**描述**: 扫描代码中的安全漏洞

**参数**:
- `<file>` - 要扫描的 Go 文件路径

**选项**:
- `-f, --format` - 输出格式（json|text）
- `-v, --verbose` - 详细输出

**使用示例**:
```bash
./go-ai-insight security ./mycode.go
./go-ai-insight security ./mycode.go -f json
```

**理想输出（无安全问题）**:
```
{
  "file": "",
  "total": 0,
  "issues": [],
  "summary": "✅ 未检测到安全问题",
  "statistics": {
    "total_issues": 0,
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0
  }
}
```

**理想输出（有安全问题）**:
```
{
  "file": "",
  "total": 2,
  "issues": [
    {
      "rule": "hardcoded-secret",
      "severity": "high",
      "message": "检测到硬编码密钥",
      "line": 10,
      "suggestion": "使用环境变量或配置文件"
    },
    ...
  ],
  "summary": "⚠️ 检测到 2 个安全问题"
}
```

---

### bug - Bug 检测命令

**语法**: `go-ai-insight bug <file> [options]`

**描述**: 检测代码中的常见 Bug

**参数**:
- `<file>` - 要检测的 Go 文件路径

**选项**:
- `-f, --format` - 输出格式（json|text）
- `-v, --verbose` - 详细输出

**使用示例**:
```bash
./go-ai-insight bug ./mycode.go
./go-ai-insight bug ./mycode.go -f json
```

**理想输出**:
```
{
  "language": "go",
  "status": "success",
  "total_files": 1,
  "analyzed_files": 1,
  "total": 5,
  "bugs": [
    {
      "id": "bug-001",
      "rule_id": "B101",
      "severity": "Medium",
      "category": "Null Safety",
      "description": "对可能为 nil 的指针调用方法",
      "line": 15,
      "fix_suggestion": "检查 nil"
    },
    ...
  ]
}
```

---

### complexity - 复杂度分析命令

**语法**: `go-ai-insight complexity <file> [options]`

**描述**: 分析代码的圈复杂度

**参数**:
- `<file>` - 要分析的 Go 文件路径

**选项**:
- `-f, --format` - 输出格式（json|text）
- `-v, --verbose` - 详细输出

**使用示例**:
```bash
./go-ai-insight complexity ./mycode.go
./go-ai-insight complexity ./mycode.go -f json
./go-ai-insight complexity ./mycode.go -v
```

**理想输出**:
```
{
  "file": "",
  "total": 25,
  "functions": [
    {
      "name": "main",
      "line": 10,
      "complexity": 3,
      "lines": 15,
      "issues": null
    },
    {
      "name": "complexFunction",
      "line": 30,
      "complexity": 15,
      "lines": 60,
      "issues": [
        "⚠️ 圈复杂度偏高（>10），可能需要重构",
        "📏 函数较长（>50行），可考虑拆分"
      ]
    }
  ],
  "summary": {
    "total_functions": 2,
    "high_complexity": 1,
    "medium_complexity": 1,
    "low_complexity": 0
  }
}
```

---

### list - 列出命令

**语法**: `go-ai-insight list`

**描述**: 列出所有可用命令

**使用示例**:
```bash
./go-ai-insight list
```

**理想输出**:
```
可用命令:
  scan         扫描代码并存储到向量数据库
  analyze      分析代码并提供智能建议
  test         生成单元测试
  security     安全漏洞扫描
  bug          常见 Bug 检测
  complexity   代码复杂度分析
  list         列出所有可用工具
```

---

## 使用示例

### 示例 1：分析单个文件

```bash
# 编译程序
go build -o go-ai-insight ./cmd

# 分析文件
./go-ai-insight complexity internal/tools/complexity_analyzer.go
```

### 示例 2：生成测试

```bash
# 生成测试
./go-ai-insight test internal/tools/test_generator.go

# 查看生成的测试文件
cat internal/tools/test_generator_test.go
```

### 示例 3：安全扫描

```bash
# 安全扫描
./go-ai-insight security internal/tools/complexity_analyzer.go

# 查看详细输出
./go-ai-insight security internal/tools/complexity_analyzer.go -v
```

### 示例 4：批量分析

```bash
# 分析多个文件
for file in internal/tools/*.go; do
    echo "=== 分析 $file ==="
    ./go-ai-insight complexity "$file" | head -10
done
```

### 示例 5：生成 JSON 报告

```bash
# 生成 JSON 报告
./go-ai-insight complexity internal/tools/test_generator.go -f json > report.json

# 查看报告
cat report.json
```

### 示例 6：组合使用

```bash
# 先分析复杂度，再扫描安全
./go-ai-insight complexity ./mycode.go && \
./go-ai-insight security ./mycode.go

# 三步分析流程
./go-ai-insight complexity ./mycode.go && \
./go-ai-insight security ./mycode.go && \
./go-ai-insight bug ./mycode.go
```

---

## 配置说明

### 配置文件位置

默认配置文件：`~/.go-ai-insight/config.json`

### 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `default_output` | string | "stdout" | 默认输出位置 |
| `default_format` | string | "text" | 默认输出格式 |
| `verbose` | bool | false | 详细输出 |
| `ollama_endpoint` | string | "http://localhost:11434" | Ollama 服务地址 |
| `milvus_endpoint` | string | "http://localhost:19530" | Milvus 服务地址 |

### 配置文件示例

```json
{
  "default_output": "stdout",
  "default_format": "text",
  "verbose": false,
  "ollama_endpoint": "http://localhost:11434",
  "milvus_endpoint": "http://localhost:19530"
}
```

### 环境变量

| 变量名 | 说明 |
|--------|------|
| `GO_AI_INSIGHT_VERBOSE` | 详细输出开关 |
| `GO_AI_INSIGHT_FORMAT` | 默认输出格式 |

### 配置优先级

命令行参数 > 环境变量 > 配置文件 > 默认值

---

## 输出格式

### 文本格式

**特点**:
- 简洁易读
- 使用标签标记（[SUCCESS], [ERROR], [WARNING]）
- 支持 verbose 模式显示更多信息

**示例**:
```
[SUCCESS] 测试生成成功

生成的测试文件数: 1
测试用例总数: 3
```

### JSON 格式

**特点**:
- 机器可读
- 适合程序解析
- 包含完整的结构化数据

**示例**:
```json
{
  "success": true,
  "result": "测试生成成功\n\n生成的测试文件数: 1\n测试用例总数: 3"
}
```

---

## 常见问题

### Q1: 编译失败怎么办？

**A**: 确保 Go 版本 >= 1.23，并安装所有依赖：

```bash
go version
go mod download
go build -o go-ai-insight ./cmd
```

### Q2: 如何生成测试？

**A**: 使用 test 命令：

```bash
./go-ai-insight test ./mycode.go
```

### Q3: 如何查看详细输出？

**A**: 使用 `-v` 选项：

```bash
./go-ai-insight complexity ./mycode.go -v
```

### Q4: 如何保存分析结果？

**A**: 使用 JSON 格式并重定向输出：

```bash
./go-ai-insight complexity ./mycode.go -f json > report.json
```

### Q5: 如何分析整个目录？

**A**: 使用循环遍历目录：

```bash
for file in ./mydir/*.go; do
    ./go-ai-insight complexity "$file"
done
```

---

## 开发指南

### 添加新命令

1. 在 `internal/cli/commands/` 创建新的命令文件
2. 实现 `Command` 接口
3. 在 `internal/cli/cli.go` 中注册命令

### 添加新工具

1. 在 `internal/tools/` 创建新的工具文件
2. 实现 `Tool` 接口
3. 在 `internal/cli/cli.go` 中注册工具

### 运行测试

```bash
# 运行所有测试
go test ./internal/tools/ -v

# 运行特定测试
go test ./internal/tools/ -run TestComplexity -v
```

---

## 版本信息

当前版本：1.0.0

更新日期：2026-02-06

---

## 许可证

MIT
