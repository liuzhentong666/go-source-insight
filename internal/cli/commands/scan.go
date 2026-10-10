package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"go-ai-study/internal/cli/output"
	"go-ai-study/internal/source"
)

// ScanCommand 扫描命令
type ScanCommand struct{}

// NewScanCommand 创建扫描命令
func NewScanCommand() *ScanCommand {
	return &ScanCommand{}
}

// Name 命令名称
func (c *ScanCommand) Name() string {
	return "scan"
}

// Description 命令描述
func (c *ScanCommand) Description() string {
	return "扫描 Go 项目结构并输出文件统计"
}

// Run 执行命令
func (c *ScanCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if len(args) == 0 {
		return fmt.Errorf("需要指定路径")
	}

	result, err := source.Scan(ctx, args[0])
	if err != nil {
		return err
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化扫描结果失败: %w", err)
	}

	return output.Print(formatter, string(jsonBytes))
}
