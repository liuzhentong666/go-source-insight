package cli

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/commands"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/config"
	"go-ai-study/internal/tools"
)

// CLI 主 CLI 结构
type CLI struct {
	toolManager    *tools.ToolManager
	commandRegistry *commands.CommandRegistry
	config         *config.Config
	formatter      output.Formatter
}

// NewCLI 创建 CLI
func NewCLI(configPath, format string, outputPath string, verbose bool) (*CLI, error) {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// 命令行参数优先级高于配置文件
	if format != "text" {
		cfg.DefaultFormat = format
	}
	if verbose {
		cfg.Verbose = true
	}

	// 创建输出格式化器
	var formatter output.Formatter
	outputOptions := output.Options{
		Verbose: cfg.Verbose,
	}

	switch cfg.DefaultFormat {
	case "json":
		formatter = output.NewJSONFormatter()
	case "text":
		formatter = output.NewTextFormatter(outputOptions)
	default:
		return nil, fmt.Errorf("不支持的输出格式: %s", cfg.DefaultFormat)
	}

	// 创建 ToolManager
	toolManager := tools.NewToolManager(tools.NewNoopLogger())

	// 注册所有工具
	registerTools(toolManager)

	// 创建命令注册表
	commandRegistry := commands.NewCommandRegistry()
	registerCommands(commandRegistry, toolManager)

	return &CLI{
		toolManager:    toolManager,
		commandRegistry: commandRegistry,
		config:         cfg,
		formatter:      formatter,
	}, nil
}

// registerTools 注册所有工具
func registerTools(tm *tools.ToolManager) {
	// 注册测试生成器
	tm.Register(
		tools.NewTestGenerator(tools.NewNoopLogger()),
		tools.DefaultToolConfig("test_generator"),
	)

	// 注册复杂度分析器
	tm.Register(
		tools.NewComplexityAnalyzer(),
		tools.DefaultToolConfig("complexity_analyzer"),
	)

	// 注册安全扫描器
	tm.Register(
		tools.NewSecurityScanner(),
		tools.DefaultToolConfig("security_scanner"),
	)

	// 注册 Bug 检测器
	tm.Register(
		tools.NewBugDetector(),
		tools.DefaultToolConfig("bug_detector"),
	)
}

// registerCommands 注册所有命令
func registerCommands(registry *commands.CommandRegistry, toolManager *tools.ToolManager) {
	registry.Register(commands.NewAnalyzeCommand(toolManager))
	registry.Register(commands.NewTestCommand(toolManager))
	registry.Register(commands.NewSecurityCommand(toolManager))
	registry.Register(commands.NewBugCommand(toolManager))
	registry.Register(commands.NewComplexityCommand(toolManager))
	registry.Register(commands.NewScanCommand())
	registry.Register(commands.NewListCommand(registry))
}

// Run 执行 CLI
func (c *CLI) Run(ctx context.Context, args []string) error {
	// 如果没有参数，显示帮助
	if len(args) == 0 {
		return c.printHelp()
	}

	commandName := args[0]
	commandArgs := args[1:]

	// 获取命令
	cmd, ok := c.commandRegistry.Get(commandName)
	if !ok {
		return fmt.Errorf("未知命令: %s\n运行 'go-ai-insight list' 查看可用命令", commandName)
	}

	// 执行命令
	return cmd.Run(ctx, commandArgs, c.formatter)
}

// printHelp 打印帮助信息
func (c *CLI) printHelp() error {
	fmt.Println("go-ai-insight - Go 代码分析和测试工具")
	fmt.Println("")
	fmt.Println("使用:")
	fmt.Println("  go-ai-insight <command> [options]")
	fmt.Println("")
	fmt.Println("命令:")
	fmt.Println("  scan        扫描代码并存储")
	fmt.Println("  analyze     分析代码")
	fmt.Println("  test        生成测试")
	fmt.Println("  security    安全扫描")
	fmt.Println("  bug         Bug 检测")
	fmt.Println("  complexity  复杂度分析")
	fmt.Println("  list        列出所有可用工具")
	fmt.Println("")
	fmt.Println("全局选项:")
	fmt.Println("  -c, --config <file>   配置文件路径")
	fmt.Println("  -f, --format <format> 输出格式 (json|text)")
	fmt.Println("  -o, --output <file>   输出文件路径")
	fmt.Println("  -v, --verbose         详细输出")
	fmt.Println("  --version             显示版本信息")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  go-ai-insight analyze ./myproject")
	fmt.Println("  go-ai-insight test ./myproject -f json -o result.json")
	fmt.Println("  go-ai-insight security ./myproject -v")

	return nil
}
