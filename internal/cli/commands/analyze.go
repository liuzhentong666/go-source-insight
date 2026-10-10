package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
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
	return "对 Go 文件或目录执行静态规则分析"
}

// Run 执行命令
func (c *AnalyzeCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	result, err := analyzeTarget(ctx, c.toolManager, args[0])
	if err != nil {
		return fmt.Errorf("静态分析失败: %w", err)
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化分析结果失败: %w", err)
	}

	return output.Print(formatter, string(jsonBytes))
}
