# Tools Package

GoSource-Insight 工具系统 - 为 AI 提供 Tool Calling 能力的核心组件。

## 架构设计

```
ToolManager (管理器)
  ├─ Register()     注册工具
  ├─ Get()          获取工具
  ├─ Run()          执行工具
  ├─ Enable()       启用工具
  ├─ Disable()      禁用工具
  └─ List()         列出工具

Tool (工具接口)
  ├─ Name()         工具名称
  ├─ Description()  工具描述
  ├─ InputType()    输入类型
  ├─ Validate()     验证输入
  └─ Run()          执行逻辑

BaseTool (基础实现)
  └─ 提供通用功能

Logger (日志系统)
  ├─ DefaultLogger     文本日志
  ├─ JSONLogger        JSON 日志
  ├─ FileLogger        文件日志
  └─ NoopLogger        空日志（测试用）
```

## 快速开始

### 1. 创建工具

```go
package tools

import (
    "context"
    "reflect"
)

type MyTool struct {
    *BaseTool
}

func NewMyTool() *MyTool {
    return &MyTool{
        BaseTool: NewBaseTool(
            "my_tool",                           // 名称
            "我的工具描述",                        // 描述
            reflect.TypeOf(""),                   // 输入类型（字符串）
        ),
    }
}

func (t *MyTool) Run(ctx context.Context, input any) (string, error) {
    code := input.(string)
    // 实现你的工具逻辑...
    return "result", nil
}
```

### 2. 注册和使用工具

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "go-ai-study/internal/tools"
)

func main() {
    // 创建工具管理器
    logger := tools.NewDefaultLogger(slog.LevelInfo)
    toolManager := tools.NewToolManager(logger)

    // 注册工具
    myTool := tools.NewMyTool()
    config := tools.DefaultToolConfig("my_tool")
    config.Timeout = 5000  // 5秒超时

    if err := toolManager.Register(myTool, config); err != nil {
        panic(err)
    }

    // 执行工具
    result, err := toolManager.Run(
        context.Background(),
        "my_tool",
        "input data",
    )

    if err != nil {
        panic(err)
    }

    // 输出结果
    if result.Success {
        fmt.Printf("成功: %s\n", result.Result)
    } else {
        fmt.Printf("失败: %s\n", result.Error)
    }
}
```

## 核心接口

### Tool 接口

所有工具必须实现 `Tool` 接口：

```go
type Tool interface {
    Name() string
    Description() string
    InputType() reflect.Type
    Validate(input any) error
    Run(ctx context.Context, input any) (string, error)
}
```

### ToolResult

工具执行结果：

```go
type ToolResult struct {
    Success       bool           // 是否成功
    Result        string         // 结果数据（JSON）
    Error         string         // 错误信息
    ExecutionTime int64          // 执行时间（毫秒）
    Metadata      map[string]any // 元数据
}
```

## 配置选项

### ToolConfig

```go
type ToolConfig struct {
    Name         string         // 工具名称
    Enabled      bool           // 是否启用
    Timeout      int64          // 超时时间（毫秒）
    MaxRetries   int            // 最大重试次数
    CustomConfig map[string]any // 自定义配置
}
```

### 默认配置

```go
config := tools.DefaultToolConfig("my_tool")
// 默认值:
//   Enabled: true
//   Timeout: 30000 (30秒)
//   MaxRetries: 1
```

## 错误处理

工具系统提供统一的错误类型：

```go
var (
    ErrToolNotFound    = errors.New("工具不存在")
    ErrToolDisabled    = errors.New("工具已禁用")
    ErrInvalidInput    = errors.New("无效的输入")
    ErrToolTimeout     = errors.New("工具执行超时")
    ErrToolExecution   = errors.New("工具执行失败")
    ErrInputValidation = errors.New("输入验证失败")
)
```

## 日志系统

### 创建日志记录器

```go
// 文本日志（默认）
logger := tools.NewDefaultLogger(slog.LevelInfo)

// JSON 日志
logger := tools.NewJSONLogger(slog.LevelDebug)

// 文件日志
logger, err := tools.NewFileLogger(slog.LevelInfo, "/var/log/tools.log")

// 空日志（测试用）
logger := tools.NewNoopLogger()
```

## 工具管理

### 注册工具

```go
err := toolManager.Register(tool, config)
```

### 执行工具

```go
result, err := toolManager.Run(ctx, "tool_name", input)
```

### 启用/禁用工具

```go
// 禁用工具
err := toolManager.Disable("tool_name")

// 启用工具
err := toolManager.Enable("tool_name")
```

### 列出工具

```go
// 获取所有工具名称
tools := toolManager.List()

// 获取工具及其状态
statuses := toolManager.ListWithStatus()
```

## 线程安全

`ToolManager` 使用读写锁保证线程安全，可以在多个 goroutine 中安全使用。

```go
// 可以安全地并发调用
go toolManager.Run(ctx, "tool1", input1)
go toolManager.Run(ctx, "tool2", input2)
go toolManager.Run(ctx, "tool3", input3)
```

## 测试

运行测试：

```bash
cd /path/to/go-ai-study
go test ./internal/tools/... -v
```

## 最佳实践

### 1. 工具命名

使用描述性的名称，使用下划线分隔：

```go
✅ "complexity_analyzer"
✅ "security_scanner"
✅ "bug_detector"

❌ "ca"
❌ "security"
❌ "bug"
```

### 2. 工具描述

提供清晰的描述，让 AI 知道何时调用该工具：

```go
✅ "分析 Go 代码的圈复杂度，识别过于复杂的函数（圈复杂度 > 10）"

❌ "分析复杂度"
```

### 3. 错误处理

总是返回详细的错误信息：

```go
✅ return "", fmt.Errorf("解析代码失败: %v, 文件: %s", err, file)

❌ return "", errors.New("解析失败")
```

### 4. 超时控制

为长时间运行的工具设置合理的超时：

```go
config := tools.DefaultToolConfig("slow_tool")
config.Timeout = 60000  // 60秒
```

### 5. 输入验证

在 `Run()` 前验证输入：

```go
func (t *MyTool) Run(ctx context.Context, input any) (string, error) {
    // 验证输入
    code, ok := input.(string)
    if !ok || code == "" {
        return "", tools.ErrInvalidInput
    }

    // 执行逻辑...
}
```

## 示例工具

### 代码复杂度分析器

```go
type ComplexityAnalyzer struct {
    *BaseTool
}

func NewComplexityAnalyzer() *ComplexityAnalyzer {
    return &ComplexityAnalyzer{
        BaseTool: NewBaseTool(
            "complexity_analyzer",
            "分析 Go 代码的圈复杂度，识别过于复杂的函数",
            reflect.TypeOf(""),
        ),
    }
}

func (ca *ComplexityAnalyzer) Run(ctx context.Context, input any) (string, error) {
    code := input.(string)
    // 计算圈复杂度...
    return `{"complexity": 15, "functions": [...]}`, nil
}
```

## 贡献

欢迎贡献新的工具！

1. 创建新工具文件（如 `my_tool.go`）
2. 实现 `Tool` 接口
3. 编写测试
4. 更新文档

## 许可证

MIT License
