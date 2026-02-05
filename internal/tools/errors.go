package tools

import "errors"

// 工具错误类型
var (
	ErrToolNotFound    = errors.New("工具不存在")
	ErrToolDisabled    = errors.New("工具已禁用")
	ErrInvalidInput    = errors.New("无效的输入")
	ErrToolTimeout     = errors.New("工具执行超时")
	ErrToolExecution   = errors.New("工具执行失败")
	ErrInputValidation = errors.New("输入验证失败")
)

// IsToolError 判断是否是工具相关错误
func IsToolError(err error) bool {
	return errors.Is(err, ErrToolNotFound) ||
		errors.Is(err, ErrToolDisabled) ||
		errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrToolTimeout) ||
		errors.Is(err, ErrToolExecution) ||
		errors.Is(err, ErrInputValidation)
}
