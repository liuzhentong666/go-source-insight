package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
	"os"
)

// BugCommand Bug 检测命令
type BugCommand struct {
	toolManager *tools.ToolManager
}

// NewBugCommand 创建 Bug 检测命令
func NewBugCommand(toolManager *tools.ToolManager) *BugCommand {
	return &BugCommand{
		toolManager: toolManager,
	}
}

// Name 命令名称
func (c *BugCommand) Name() string {
	return "bug"
}

// Description 命令描述
func (c *BugCommand) Description() string {
	return "常见 Bug 检测"
}

// Run 执行命令
func (c *BugCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	target := args[0]

	// 读取文件内容
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 执行 Bug 检测
	bugResult, err := c.toolManager.Run(ctx, "bug_detector", string(content))
	if err != nil {
		return fmt.Errorf("Bug 检测失败: %w", err)
	}

	// 输出结果
	fmt.Println(formatter.Format(bugResult.Result))

	return nil
}
