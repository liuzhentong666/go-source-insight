package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
	"os"
)

// ComplexityCommand 复杂度分析命令
type ComplexityCommand struct {
	toolManager *tools.ToolManager
}

// NewComplexityCommand 创建复杂度分析命令
func NewComplexityCommand(toolManager *tools.ToolManager) *ComplexityCommand {
	return &ComplexityCommand{
		toolManager: toolManager,
	}
}

// Name 命令名称
func (c *ComplexityCommand) Name() string {
	return "complexity"
}

// Description 命令描述
func (c *ComplexityCommand) Description() string {
	return "代码复杂度分析"
}

// Run 执行命令
func (c *ComplexityCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	target := args[0]

	// 读取文件内容
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 执行复杂度分析
	complexityResult, err := c.toolManager.Run(ctx, "complexity_analyzer", string(content))
	if err != nil {
		return fmt.Errorf("复杂度分析失败: %w", err)
	}

	// 输出结果
	if complexityResult != nil && complexityResult.Success {
		fmt.Println(formatter.Format(complexityResult.Result))
	} else {
		fmt.Println("[ERROR] 分析失败")
	}

	return nil
}

