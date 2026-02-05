package tools

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ToolConfig 工具的配置项
type ToolConfig struct {
	// Name 工具名称
	Name string

	// Enabled 是否启用
	Enabled bool

	// Timeout 超时时间（毫秒）
	Timeout int64

	// MaxRetries 最大重试次数
	MaxRetries int

	// CustomConfig 自定义配置（工具特定）
	CustomConfig map[string]any
}

// DefaultToolConfig 默认工具配置
func DefaultToolConfig(name string) ToolConfig {
	return ToolConfig{
		Name:        name,
		Enabled:     true,
		Timeout:     30000, // 30秒默认超时
		MaxRetries:  1,
		CustomConfig: make(map[string]any),
	}
}

// ToolManager 工具管理器
type ToolManager struct {
	tools   map[string]Tool       // 工具注册表
	configs map[string]ToolConfig // 工具配置
	mu      sync.RWMutex          // 读写锁
	logger  Logger                // 日志记录器
}

// NewToolManager 创建工具管理器
func NewToolManager(logger Logger) *ToolManager {
	return &ToolManager{
		tools:   make(map[string]Tool),
		configs: make(map[string]ToolConfig),
		logger:  logger,
	}
}

// Register 注册工具
func (tm *ToolManager) Register(tool Tool, config ToolConfig) error {
	if tool == nil {
		return ErrInvalidInput
	}

	name := tool.Name()
	if name == "" {
		return ErrInvalidInput
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tools[name]; exists {
		return fmt.Errorf("工具 %s 已注册", name)
	}

	tm.tools[name] = tool
	tm.configs[name] = config

	if tm.logger != nil {
		tm.logger.Info("工具注册成功", "tool", name, "enabled", config.Enabled)
	}

	return nil
}

// Get 获取工具
func (tm *ToolManager) Get(name string) (Tool, ToolConfig, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tool, exists := tm.tools[name]
	if !exists {
		return nil, ToolConfig{}, ErrToolNotFound
	}

	config := tm.configs[name]
	if !config.Enabled {
		return nil, ToolConfig{}, ErrToolDisabled
	}

	return tool, config, nil
}

// List 列出所有工具
func (tm *ToolManager) List() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	names := make([]string, 0, len(tm.tools))
	for name := range tm.tools {
		names = append(names, name)
	}
	return names
}

// ListWithStatus 列出所有工具及其状态
func (tm *ToolManager) ListWithStatus() []ToolStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	status := make([]ToolStatus, 0, len(tm.tools))
	for name, tool := range tm.tools {
		config := tm.configs[name]
		status = append(status, ToolStatus{
			Name:        name,
			Description: tool.Description(),
			Enabled:     config.Enabled,
			Timeout:     config.Timeout,
		})
	}
	return status
}

// ToolStatus 工具状态
type ToolStatus struct {
	Name        string
	Description string
	Enabled     bool
	Timeout     int64
}

// Run 执行工具
func (tm *ToolManager) Run(ctx context.Context, toolName string, input any) (*ToolResult, error) {
	// 1. 获取工具
	tool, config, err := tm.Get(toolName)
	if err != nil {
		if tm.logger != nil {
			tm.logger.Error("获取工具失败", "tool", toolName, "error", err)
		}
		return nil, err
	}

	// 2. 验证输入
	if err := tool.Validate(input); err != nil {
		if tm.logger != nil {
			tm.logger.Error("输入验证失败", "tool", toolName, "error", err)
		}
		return NewToolResult(false, "", fmt.Sprintf("输入验证失败: %v", err), 0), nil
	}

	// 3. 创建带超时的上下文
	runCtx := ctx
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Millisecond)
		defer cancel()
	}

	// 4. 执行工具（带重试）
	startTime := time.Now()
	var result string
	var execErr error

	for retry := 0; retry <= config.MaxRetries; retry++ {
		if retry > 0 {
			if tm.logger != nil {
				tm.logger.Info("重试工具执行", "tool", toolName, "attempt", retry)
			}
		}

		result, execErr = tool.Run(runCtx, input)
		if execErr == nil {
			break
		}

		if errors.Is(execErr, context.DeadlineExceeded) {
			if tm.logger != nil {
				tm.logger.Error("工具执行超时", "tool", toolName, "timeout", config.Timeout)
			}
			execErr = ErrToolTimeout
			break
		}
	}

	executionTime := time.Since(startTime).Milliseconds()

	// 5. 构建结果
	toolResult := NewToolResult(
		execErr == nil,
		result,
		"",
		executionTime,
	)

	if execErr != nil {
		toolResult.Error = execErr.Error()
		if tm.logger != nil {
			tm.logger.Error("工具执行失败", "tool", toolName, "error", execErr, "time", executionTime)
		}
	} else {
		if tm.logger != nil {
			tm.logger.Info("工具执行成功", "tool", toolName, "time", executionTime)
		}
	}

	return toolResult, nil
}

// Enable 启用工具
func (tm *ToolManager) Enable(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tools[name]; !exists {
		return ErrToolNotFound
	}

	config := tm.configs[name]
	config.Enabled = true
	tm.configs[name] = config

	if tm.logger != nil {
		tm.logger.Info("工具已启用", "tool", name)
	}
	return nil
}

// Disable 禁用工具
func (tm *ToolManager) Disable(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tools[name]; !exists {
		return ErrToolNotFound
	}

	config := tm.configs[name]
	config.Enabled = false
	tm.configs[name] = config

	if tm.logger != nil {
		tm.logger.Info("工具已禁用", "tool", name)
	}
	return nil
}

// UpdateConfig 更新工具配置
func (tm *ToolManager) UpdateConfig(name string, config ToolConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tools[name]; !exists {
		return ErrToolNotFound
	}

	tm.configs[name] = config
	if tm.logger != nil {
		tm.logger.Info("工具配置已更新", "tool", name)
	}
	return nil
}
