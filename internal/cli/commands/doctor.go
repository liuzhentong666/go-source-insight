package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"go-ai-study/internal/cli/output"
)

type DoctorCommand struct{ factory RAGFactory }

func NewDoctorCommand(factory RAGFactory) *DoctorCommand { return &DoctorCommand{factory} }
func (c *DoctorCommand) Name() string                    { return "doctor" }
func (c *DoctorCommand) Description() string {
	return "检查 Ollama 模型、向量维度与 Milvus 连接"
}
func (c *DoctorCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	opts, err := parseRAGArgs(args, false)
	if err != nil {
		return err
	}
	if opts.help {
		return output.Print(formatter, "使用: doctor [--collection NAME]")
	}
	if len(opts.operands) != 0 {
		return fmt.Errorf("使用: doctor [--collection NAME]")
	}
	svc, err := c.factory(ctx, opts.collection)
	if err != nil {
		return err
	}
	if err := svc.Close(); err != nil {
		return fmt.Errorf("关闭 AI 连接失败: %w", err)
	}
	data, err := json.Marshal(map[string]any{"ready": true, "provider": "ollama", "checks": []string{"models", "embedding_dimension", "milvus_connection"}})
	if err != nil {
		return err
	}
	return output.Print(formatter, string(data))
}
