package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
	"os"
)

// AnalyzeCommand 分析命令
type AnalyzeCommand struct {
	toolManager *tools.ToolManager
}

// NewAnalyzeCommand 创建分析命令
func NewAnalyzeCommand(toolManager *tools.ToolManager) *AnalyzeCommand {
	return &AnalyzeCommand{
		toolManager: toolManager,
	}
}

// Name 命令名称
func (c *AnalyzeCommand) Name() string {
	return "analyze"
}

// Description 命令描述
func (c *AnalyzeCommand) Description() string {
	return "分析代码并提供智能建议"
}

// Run 执行命令
func (c *AnalyzeCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	target := args[0]

	// 读取文件内容
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 执行分析
	// 这里可以调用多个工具进行分析
	// 例如：复杂度分析 + 安全扫描 + Bug 检测

	// 执行复杂度分析
	complexityResult, err := c.toolManager.Run(ctx, "complexity_analyzer", string(content))
	if err != nil {
		return fmt.Errorf("复杂度分析失败: %w", err)
	}

	// 输出结果
	fmt.Println(formatter.Format(complexityResult.Result))

	return nil
}
