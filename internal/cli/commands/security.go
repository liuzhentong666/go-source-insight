package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
	"os"
)

// SecurityCommand 安全扫描命令
type SecurityCommand struct {
	toolManager *tools.ToolManager
}

// NewSecurityCommand 创建安全扫描命令
func NewSecurityCommand(toolManager *tools.ToolManager) *SecurityCommand {
	return &SecurityCommand{
		toolManager: toolManager,
	}
}

// Name 命令名称
func (c *SecurityCommand) Name() string {
	return "security"
}

// Description 命令描述
func (c *SecurityCommand) Description() string {
	return "安全漏洞扫描"
}

// Run 执行命令
func (c *SecurityCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	target := args[0]

	// 读取文件内容
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 执行安全扫描
	securityResult, err := c.toolManager.Run(ctx, "security_scanner", string(content))
	if err != nil {
		return fmt.Errorf("安全扫描失败: %w", err)
	}

	// 输出结果
	fmt.Println(formatter.Format(securityResult.Result))

	return nil
}
