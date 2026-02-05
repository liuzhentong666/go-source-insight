package tools

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// MockTool 测试用的模拟工具
type MockTool struct {
	*BaseTool
	runFunc func(ctx context.Context, input any) (string, error)
}

func NewMockTool(name string, runFunc func(ctx context.Context, input any) (string, error)) *MockTool {
	return &MockTool{
		BaseTool: NewBaseTool(name, "Mock tool for testing", reflect.TypeOf("")),
		runFunc:  runFunc,
	}
}

func (mt *MockTool) Run(ctx context.Context, input any) (string, error) {
	if mt.runFunc != nil {
		return mt.runFunc(ctx, input)
	}
	return "mock result", nil
}

// 测试工具注册
func TestToolManager_Register(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	tool := NewMockTool("test_tool", nil)
	config := DefaultToolConfig("test_tool")

	err := tm.Register(tool, config)
	if err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}

	// 检查工具是否注册成功
	_, _, err = tm.Get("test_tool")
	if err != nil {
		t.Fatalf("获取工具失败: %v", err)
	}

	// 测试重复注册
	err = tm.Register(tool, config)
	if err == nil {
		t.Fatal("重复注册应该返回错误")
	}
}

// 测试获取工具
func TestToolManager_Get(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	tool := NewMockTool("test_tool", nil)
	config := DefaultToolConfig("test_tool")
	tm.Register(tool, config)

	// 测试获取存在的工具
	retrievedTool, _, err := tm.Get("test_tool")
	if err != nil {
		t.Fatalf("获取工具失败: %v", err)
	}
	if retrievedTool.Name() != "test_tool" {
		t.Fatalf("工具名称不匹配: 期望 test_tool, 实际 %s", retrievedTool.Name())
	}

	// 测试获取不存在的工具
	_, _, err = tm.Get("nonexistent")
	if err == nil {
		t.Fatal("获取不存在的工具应该返回错误")
	}
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("错误类型不匹配: 期望 %v, 实际 %v", ErrToolNotFound, err)
	}
}

// 测试禁用工具
func TestToolManager_Disable(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	tool := NewMockTool("test_tool", nil)
	config := DefaultToolConfig("test_tool")
	tm.Register(tool, config)

	// 禁用工具
	err := tm.Disable("test_tool")
	if err != nil {
		t.Fatalf("禁用工具失败: %v", err)
	}

	// 尝试获取已禁用的工具
	_, _, err = tm.Get("test_tool")
	if err == nil {
		t.Fatal("获取已禁用的工具应该返回错误")
	}
	if !errors.Is(err, ErrToolDisabled) {
		t.Fatalf("错误类型不匹配: 期望 %v, 实际 %v", ErrToolDisabled, err)
	}

	// 重新启用工具
	err = tm.Enable("test_tool")
	if err != nil {
		t.Fatalf("启用工具失败: %v", err)
	}

	// 现在应该可以获取
	_, _, err = tm.Get("test_tool")
	if err != nil {
		t.Fatalf("获取工具失败: %v", err)
	}
}

// 测试工具执行
func TestToolManager_Run(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	// 创建一个返回固定结果的工具
	tool := NewMockTool("test_tool", func(ctx context.Context, input any) (string, error) {
		return "success result", nil
	})
	config := DefaultToolConfig("test_tool")
	tm.Register(tool, config)

	// 执行工具
	result, err := tm.Run(context.Background(), "test_tool", "test input")
	if err != nil {
		t.Fatalf("执行工具失败: %v", err)
	}

	if !result.Success {
		t.Fatalf("工具应该执行成功")
	}

	if result.Result != "success result" {
		t.Fatalf("结果不匹配: 期望 'success result', 实际 '%s'", result.Result)
	}

	// 执行时间可能非常小（<1ms），所以只要 >= 0 就可以
	if result.ExecutionTime < 0 {
		t.Fatalf("执行时间应该 >= 0")
	}
}

// 测试工具超时
func TestToolManager_Timeout(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	// 创建一个模拟超时的工具（检查 context 超时）
	tool := NewMockTool("slow_tool", func(ctx context.Context, input any) (string, error) {
		// 模拟长时间执行，定期检查 context 超时
		for i := 0; i < 20; i++ {
			select {
			case <-ctx.Done():
				return "", ctx.Err() // 返回 context 的错误（DeadlineExceeded）
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		return "result", nil
	})

	config := DefaultToolConfig("slow_tool")
	config.Timeout = 100 // 100ms 超时
	tm.Register(tool, config)

	// 执行工具（应该超时）
	result, err := tm.Run(context.Background(), "slow_tool", "test input")
	if err != nil {
		t.Fatalf("执行工具失败: %v", err)
	}

	t.Logf("Result: Success=%v, Error=%q, ExecutionTime=%d", result.Success, result.Error, result.ExecutionTime)

	if result.Success {
		t.Fatal("工具应该因超时而失败")
	}

	if result.Error != ErrToolTimeout.Error() {
		t.Fatalf("错误不匹配: 期望 '%s', 实际 '%s'", ErrToolTimeout.Error(), result.Error)
	}
}

// 测试工具返回错误
func TestToolManager_RunWithError(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	// 创建一个返回错误的工具
	tool := NewMockTool("error_tool", func(ctx context.Context, input any) (string, error) {
		return "", errors.New("tool execution error")
	})

	config := DefaultToolConfig("error_tool")
	tm.Register(tool, config)

	// 执行工具
	result, err := tm.Run(context.Background(), "error_tool", "test input")
	if err != nil {
		t.Fatalf("执行工具失败: %v", err)
	}

	if result.Success {
		t.Fatal("工具应该失败")
	}

	if result.Error != "tool execution error" {
		t.Fatalf("错误不匹配: 期望 'tool execution error', 实际 '%s'", result.Error)
	}
}

// 测试列出所有工具
func TestToolManager_List(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	// 注册多个工具
	tm.Register(NewMockTool("tool1", nil), DefaultToolConfig("tool1"))
	tm.Register(NewMockTool("tool2", nil), DefaultToolConfig("tool2"))
	tm.Register(NewMockTool("tool3", nil), DefaultToolConfig("tool3"))

	// 列出工具
	tools := tm.List()
	if len(tools) != 3 {
		t.Fatalf("工具数量不匹配: 期望 3, 实际 %d", len(tools))
	}

	// 检查是否包含所有工具
	toolSet := make(map[string]bool)
	for _, name := range tools {
		toolSet[name] = true
	}

	for _, name := range []string{"tool1", "tool2", "tool3"} {
		if !toolSet[name] {
			t.Fatalf("工具列表中缺少: %s", name)
		}
	}
}

// 测试 BaseTool 验证
func TestBaseTool_Validate(t *testing.T) {
	tool := NewBaseTool("test", "Test tool", reflect.TypeOf(""))

	// 测试有效输入
	err := tool.Validate("valid input")
	if err != nil {
		t.Fatalf("有效输入验证失败: %v", err)
	}

	// 测试空字符串
	err = tool.Validate("")
	if err == nil {
		t.Fatal("空字符串应该验证失败")
	}

	// 测试 nil
	err = tool.Validate(nil)
	if err == nil {
		t.Fatal("nil 输入应该验证失败")
	}

	// 测试类型错误
	err = tool.Validate(123)
	if err == nil {
		t.Fatal("错误类型应该验证失败")
	}
}
