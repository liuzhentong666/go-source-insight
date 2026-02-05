package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
)

// ListCommand 列出所有命令
type ListCommand struct {
	registry *CommandRegistry
}

// NewListCommand 创建列出命令
func NewListCommand(registry *CommandRegistry) *ListCommand {
	return &ListCommand{
		registry: registry,
	}
}

// Name 命令名称
func (c *ListCommand) Name() string {
	return "list"
}

// Description 命令描述
func (c *ListCommand) Description() string {
	return "列出所有可用工具"
}

// Run 执行命令
func (c *ListCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	commands := c.registry.List()

	fmt.Println("可用命令:")
	for _, cmd := range commands {
		fmt.Printf("  %-12s %s\n", cmd.Name(), cmd.Description())
	}

	return nil
}
