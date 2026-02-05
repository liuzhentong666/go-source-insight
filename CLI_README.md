# GoSource-Insight CLI

Go 代码分析和测试工具的命令行接口。

## 安装

```bash
go install ./...
```

## 使用

### 基本命令

```bash
# 查看帮助
go-ai-insight

# 列出所有可用工具
go-ai-insight list

# 分析代码
go-ai-insight analyze ./myproject

# 生成测试
go-ai-insight test ./myproject

# 安全扫描
go-ai-insight security ./myproject

# Bug 检测
go-ai-insight bug ./myproject

# 复杂度分析
go-ai-insight complexity ./myproject

# 扫描代码（暂未实现）
go-ai-insight scan ./myproject
```

### 全局选项

```
-c, --config <file>   配置文件路径
-f, --format <format> 输出格式 (json|text)
-o, --output <file>   输出文件路径
-v, --verbose         详细输出
--version             显示版本信息
```

### 示例

#### 基本使用

```bash
# 分析项目
go-ai-insight analyze ./myproject
```

#### 指定输出格式

```bash
# JSON 输出
go-ai-insight analyze ./myproject -f json -o result.json

# 文本输出（默认）
go-ai-insight analyze ./myproject -f text
```

#### 详细输出

```bash
# 显示详细信息
go-ai-insight security ./myproject -v
```

#### 使用配置文件

```bash
# 使用指定配置文件
go-ai-insight analyze ./myproject -c /path/to/config.json
```

#### 组合命令

```bash
# 先进行复杂度分析，再进行安全扫描
go-ai-insight complexity ./myproject && go-ai-insight security ./myproject
```

## 配置文件

配置文件位于 `~/.go-ai-insight/config.json`。

示例配置：

```json
{
  "default_output": "stdout",
  "default_format": "text",
  "verbose": false,
  "ollama_endpoint": "http://localhost:11434",
  "milvus_endpoint": "http://localhost:19530"
}
```

### 配置项说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `default_output` | 默认输出位置 | `stdout` |
| `default_format` | 默认输出格式 | `text` |
| `verbose` | 详细输出 | `false` |
| `ollama_endpoint` | Ollama 服务地址 | `http://localhost:11434` |
| `milvus_endpoint` | Milvus 服务地址 | `http://localhost:19530` |

### 配置优先级

命令行参数 > 环境变量 > 配置文件 > 默认值

## 输出格式

### 文本格式

```
[SUCCESS] 测试生成成功

测试用例总数: 5
```

### JSON 格式

```json
{
  "success": true,
  "result": "测试生成成功\n\n测试用例总数: 5"
}
```

## 环境变量

| 变量名 | 说明 |
|--------|------|
| `GO_AI_INSIGHT_VERBOSE` | 详细输出开关 |
| `GO_AI_INSIGHT_FORMAT` | 默认输出格式 |

## 开发

### 项目结构

```
go-ai-study/
├── cmd/
│   └── main.go           # 主入口
├── internal/
│   ├── cli/              # CLI 命令处理
│   │   ├── cli.go       # 主 CLI 结构
│   │   ├── commands/    # 命令实现
│   │   └── output/      # 输出格式化
│   ├── config/           # 配置管理
│   └── tools/           # 工具实现
├── config/
│   └── config.json      # 默认配置
└── README.md
```

### 运行

```bash
# 构建并运行
go run ./cmd/main.go list

# 构建
go build -o go-ai-insight ./cmd

# 运行
./go-ai-insight list
```

## 版本

当前版本：1.0.0

## 许可证

MIT
