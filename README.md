# GoSource-Insight 🚀

这是一个基于 Go 语言开发的 AI 源码分析助手。

## 功能特性

- 📂 源码扫描：全自动扫描 Go 项目并进行智能切片
- 🧠 向量检索：集成 Milvus 向量数据库，支持语义级代码搜索
- 💬 交互对话：支持在终端与你的代码进行连续对话
- 🔒 隐私保护：完全基于本地 Ollama 模型，代码不外传
- 📊 结构化日志：支持 Text/JSON 格式，灵活的日志级别控制

## 技术栈

- Go 1.23+
- Milvus (向量存储)
- Ollama (LLM 引擎: Llama3 & Nomic-Embed)
- LangChainGo (AI 框架)

## 日志配置

### 命令行参数

```bash
# 日志级别
--log-level <debug|info|warn|error>    # 默认：info

# 日志格式
--log-format <text|json>              # 默认：text

# 日志输出
--log-output <stdout|stderr|file>      # 默认：stdout

# 日志文件路径（当 log-output=file 时使用）
--log-file <path>
```

### 使用示例

```bash
# 使用 JSON 格式日志
go-ai-insight --log-format json analyze ./myproject

# 输出日志到文件
go-ai-insight --log-output file --log-file /var/log/insight.log test ./myproject

# 使用 debug 级别
go-ai-insight --log-level debug security ./myproject

# 组合使用
go-ai-insight --log-format json --log-level debug --log-file debug.json list
```

### 配置文件

在 `~/.go-ai-insight/config.json` 中配置：

```json
{
  "default_output": "stdout",
  "default_format": "text",
  "verbose": false,
  "ollama_endpoint": "http://localhost:11434",
  "milvus_endpoint": "http://localhost:19530",
  "log_config": {
    "level": "info",
    "format": "text",
    "output": "stdout",
    "file_path": ""
  }
}
```

**注意：** 命令行参数优先级 > 配置文件

### 日志级别说明

| 级别 | 说明 | 使用场景 |
|------|------|----------|
| debug | 最详细的日志 | 开发调试 |
| info | 常规信息 | 正常运行 |
| warn | 警告信息 | 潜在问题 |
| error | 错误信息 | 发生错误 |

### 日志输出格式

**Text 格式（默认）：**
```
time=2026-02-06T23:09:24.277+08:00 level=INFO msg=工具注册成功 tool=test_generator enabled=true
```

**JSON 格式：**
```json
{"time":"2026-02-06T23:09:26.94069409+08:00","level":"INFO","msg":"工具注册成功","tool":"test_generator","enabled":true}
```

### 生产环境推荐配置

```json
{
  "log_config": {
    "level": "info",
    "format": "json",
    "output": "file",
    "file_path": "/var/log/go-ai-insight/app.log"
  }
}
```