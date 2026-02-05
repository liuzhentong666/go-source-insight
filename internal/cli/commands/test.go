package commands

import (
	"context"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/tools"
)

// TestCommand 测试生成命令
type TestCommand struct {
	toolManager *tools.ToolManager
}

// NewTestCommand 创建测试生成命令
func NewTestCommand(toolManager *tools.ToolManager) *TestCommand {
	return &TestCommand{
		toolManager: toolManager,
	}
}

// Name 命令名称
func (c *TestCommand) Name() string {
	return "test"
}

// Description 命令描述
func (c *TestCommand) Description() string {
	return "生成单元测试"
}

// Run 执行命令
func (c *TestCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径或文件")
	}

	target := args[0]

	// 判断是文件还是目录
	req := tools.GenerateRequest{
		TestMode:    tools.TestModeTableDriven,
		WithMock:    false,
		WithCoverage: false,
	}

	// 根据参数类型决定
	if len(args) > 1 && args[1] == "--dir" {
		req.DirPath = target
	} else if len(args) > 1 && args[1] == "--function" && len(args) > 2 {
		req.FilePath = args[2]
		req.FunctionName = args[2]
	} else {
		req.FilePath = target
	}

	// 执行测试生成
	result, err := c.toolManager.Run(ctx, "test_generator", req)
	if err != nil {
		return fmt.Errorf("生成测试失败: %w", err)
	}

	// 输出结果
	fmt.Println(formatter.Format(result.Result))

	return nil
}
