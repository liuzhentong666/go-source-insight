package tools

import (
	"context"
	"reflect"
)

// Tool 所有工具必须实现的接口
type Tool interface {
	// Name 工具的唯一标识符
	// 示例: "complexity_analyzer", "security_scanner"
	Name() string

	// Description 工具的描述（给 AI 看）
	// AI 会根据这个决定是否调用该工具
	// 示例: "分析 Go 代码的圈复杂度，识别过于复杂的函数"
	Description() string

	// InputType 工具输入参数的类型
	// 示例: reflect.TypeOf("") 表示输入是字符串
	InputType() reflect.Type

	// Validate 验证输入参数是否合法
	// 示例: 检查代码是否为空，是否是有效的 Go 代码
	Validate(input any) error

	// Run 执行工具的核心逻辑
	// ctx: 上下文（用于超时控制、取消等）
	// input: 输入参数（类型由 InputType() 决定）
	// 返回: 工具执行结果（字符串形式）和错误
	Run(ctx context.Context, input any) (string, error)
}

// ToolResult 工具执行结果
type ToolResult struct {
	// Success 是否成功
	Success bool

	// Result 结果数据（JSON 格式）
	Result string

	// Error 错误信息（如果失败）
	Error string

	// ExecutionTime 执行时间（毫秒）
	ExecutionTime int64

	// Metadata 额外元数据
	Metadata map[string]any
}

// NewToolResult 创建工具结果
func NewToolResult(success bool, result, errorMsg string, executionTime int64) *ToolResult {
	return &ToolResult{
		Success:       success,
		Result:        result,
		Error:         errorMsg,
		ExecutionTime: executionTime,
		Metadata:      make(map[string]any),
	}
}
