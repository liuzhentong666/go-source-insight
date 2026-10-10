package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"go-ai-study/internal/ai"
	"go-ai-study/internal/cli/commands"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/config"
	"go-ai-study/internal/tools"
)

type CLI struct {
	toolManager     *tools.ToolManager
	commandRegistry *commands.CommandRegistry
	config          *config.Config
	formatter       output.Formatter
	outputPath      string
	writer          io.Writer
	ragFactory      commands.RAGFactory
}

func NewCLI(configPath, format, outputPath string, verbose bool, logLevel, logFormat, logOutput, logFilePath string) (*CLI, error) {
	cfg, err := config.LoadUnvalidated(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	if format != "" {
		cfg.DefaultFormat = format
	}
	if verbose {
		cfg.Verbose = true
	}
	if logLevel != "" {
		cfg.LogConfig.Level = logLevel
	}
	if logFormat != "" {
		cfg.LogConfig.Format = logFormat
	}
	if logOutput != "" {
		cfg.LogConfig.Output = logOutput
	}
	if logFilePath != "" {
		cfg.LogConfig.FilePath = logFilePath
	}
	// Reports own stdout. Logs must never corrupt JSON or redirected reports.
	if cfg.LogConfig.Output == "stdout" {
		cfg.LogConfig.Output = "stderr"
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	var formatter output.Formatter
	switch cfg.DefaultFormat {
	case "json":
		formatter = output.NewJSONFormatter()
	case "text":
		formatter = output.NewTextFormatter(output.Options{Verbose: cfg.Verbose})
	default:
		return nil, fmt.Errorf("不支持的输出格式: %s", cfg.DefaultFormat)
	}
	logger, err := tools.NewLoggerFactory(&cfg.LogConfig)
	if err != nil {
		return nil, fmt.Errorf("日志初始化失败: %w", err)
	}
	toolManager := tools.NewToolManager(logger)
	registerTools(toolManager)
	if outputPath == "" && cfg.DefaultOutput != "stdout" && cfg.DefaultOutput != "stderr" {
		outputPath = cfg.DefaultOutput
	}
	c := &CLI{toolManager: toolManager, config: cfg, formatter: formatter, outputPath: outputPath, writer: os.Stdout}
	if cfg.DefaultOutput == "stderr" {
		c.writer = os.Stderr
	}
	c.ragFactory = c.newRAG
	c.commandRegistry = commands.NewCommandRegistry()
	registerCommands(c.commandRegistry, toolManager)
	c.commandRegistry.Register(commands.NewIndexCommand(func(ctx context.Context, collection string) (commands.RAGClient, error) {
		return c.ragFactory(ctx, collection)
	}))
	c.commandRegistry.Register(commands.NewAskCommand(func(ctx context.Context, collection string) (commands.RAGClient, error) {
		return c.ragFactory(ctx, collection)
	}))
	c.commandRegistry.Register(commands.NewDoctorCommand(func(ctx context.Context, collection string) (commands.RAGClient, error) {
		return c.ragFactory(ctx, collection)
	}))
	return c, nil
}

func (c *CLI) newRAG(ctx context.Context, collection string) (commands.RAGClient, error) {
	cfg := *c.config
	if collection != "" {
		cfg.RAGConfig.Collection = collection
	}
	if err := cfg.ValidateRAG(); err != nil {
		return nil, fmt.Errorf("AI 配置无效: %w", err)
	}
	r := cfg.RAGConfig
	return ai.NewOllamaRAGService(ctx, ai.FactoryOptions{
		OllamaEndpoint: cfg.OllamaEndpoint, MilvusEndpoint: cfg.MilvusEndpoint,
		Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		RAG: ai.RAGOptions{Collection: r.Collection, EmbeddingModel: r.EmbeddingModel, ChatModel: r.ChatModel,
			VectorDim: r.VectorDim, TopK: r.TopK, ChunkMaxLines: r.ChunkMaxLines, ChunkMinLines: r.ChunkMinLines},
	})
}

func registerTools(tm *tools.ToolManager) {
	logger := tm.GetLogger()
	tm.Register(tools.NewTestGenerator(logger), tools.DefaultToolConfig("test_generator"))
	tm.Register(tools.NewComplexityAnalyzer(), tools.DefaultToolConfig("complexity_analyzer"))
	tm.Register(tools.NewSecurityScanner(), tools.DefaultToolConfig("security_scanner"))
	tm.Register(tools.NewBugDetector(), tools.DefaultToolConfig("bug_detector"))
}
func registerCommands(registry *commands.CommandRegistry, tm *tools.ToolManager) {
	registry.Register(commands.NewAnalyzeCommand(tm))
	registry.Register(commands.NewTestCommand(tm))
	registry.Register(commands.NewSecurityCommand(tm))
	registry.Register(commands.NewBugCommand(tm))
	registry.Register(commands.NewComplexityCommand(tm))
	registry.Register(commands.NewScanCommand())
	registry.Register(commands.NewListCommand(registry))
}

func (c *CLI) Run(ctx context.Context, args []string) error {
	var buffer bytes.Buffer
	formatter := output.WithWriter(c.formatter, &buffer)
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		if err := output.Print(formatter, helpText); err != nil {
			return err
		}
	} else {
		cmd, ok := c.commandRegistry.Get(args[0])
		if !ok {
			return fmt.Errorf("未知命令: %s；运行 list 查看可用命令", args[0])
		}
		if err := cmd.Run(ctx, args[1:], formatter); err != nil {
			return err
		}
	}
	if c.outputPath != "" {
		if err := os.WriteFile(c.outputPath, buffer.Bytes(), 0644); err != nil {
			return fmt.Errorf("写入报告失败: %w", err)
		}
		return nil
	}
	_, err := io.Copy(c.writer, &buffer)
	return err
}

const helpText = `go-ai-insight - Go 源码扫描、静态分析与 Ollama 问答

使用: go-ai-insight [全局参数] COMMAND [命令参数]

核心命令:
  scan PATH                  扫描 Go 源码结构
  analyze PATH               执行静态规则分析
  index [--collection NAME] PATH
                             建立/更新当前代码库索引
  ask [--collection NAME] [--file RELATIVE_PATH] QUESTION
                             基于已有索引问答

其他命令: doctor（服务检查）、complexity、security、bug、test（测试骨架）、list

全局参数（放在命令前）:
  -c, --config FILE          配置文件
  -f, --format text|json     报告格式
  -o, --output FILE          写入报告文件
  -v, --verbose             详细输出
  --log-level LEVEL         日志级别
  --log-format text|json     日志格式
  --log-output stderr|file  日志输出
  --log-file FILE           日志文件
  --version                 版本信息

示例:
  go-ai-insight scan ./project
  go-ai-insight -f json -o report.json analyze ./project
  go-ai-insight index --collection sample_code ./project
  go-ai-insight ask --collection sample_code "项目的入口在哪里？"

外部模型 API、Web 平台：TODO。`
