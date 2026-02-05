package tools

import (
	"fmt"
	"reflect"
)

// BaseTool 提供工具的基础实现
type BaseTool struct {
	name        string
	description string
	inputType   reflect.Type
}

// NewBaseTool 创建基础工具
func NewBaseTool(name, description string, inputType reflect.Type) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		inputType:   inputType,
	}
}

func (bt *BaseTool) Name() string {
	return bt.name
}

func (bt *BaseTool) Description() string {
	return bt.description
}

func (bt *BaseTool) InputType() reflect.Type {
	return bt.inputType
}

// Validate 默认验证逻辑：检查输入类型和是否为空
func (bt *BaseTool) Validate(input any) error {
	if input == nil {
		return ErrInvalidInput
	}

	// 类型检查
	inputType := reflect.TypeOf(input)
	if inputType != bt.inputType {
		return fmt.Errorf("输入类型错误: 期望 %v, 实际 %v", bt.inputType, inputType)
	}

	// 字符串类型的空值检查
	if bt.inputType == reflect.TypeOf("") {
		str, ok := input.(string)
		if !ok || str == "" {
			return ErrInvalidInput
		}
	}

	return nil
}
