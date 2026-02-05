package output

import (
	"fmt"
	"strings"
)

// TextFormatter 文本格式化器
type TextFormatter struct {
	options Options
}

// NewTextFormatter 创建文本格式化器
func NewTextFormatter(options Options) *TextFormatter {
	return &TextFormatter{
		options: options,
	}
}

// Format 格式化输出为纯文本
func (t *TextFormatter) Format(result string) string {
	// 简单的文本格式化
	lines := strings.Split(result, "\n")

	var formatted strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "✅") {
			formatted.WriteString("[SUCCESS] " + strings.TrimSpace(strings.TrimPrefix(line, "✅")) + "\n")
		} else if strings.HasPrefix(line, "❌") {
			formatted.WriteString("[ERROR] " + strings.TrimSpace(strings.TrimPrefix(line, "❌")) + "\n")
		} else if strings.HasPrefix(line, "⚠️") {
			formatted.WriteString("[WARNING] " + strings.TrimSpace(strings.TrimPrefix(line, "⚠️")) + "\n")
		} else if strings.HasPrefix(line, "📊") {
			if t.options.Verbose {
				formatted.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "📊")) + "\n")
			}
		} else if strings.HasPrefix(line, "📁") {
			if t.options.Verbose {
				formatted.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "📁")) + "\n")
			}
		} else if strings.HasPrefix(line, "📝") {
			formatted.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "📝")) + "\n")
		} else if strings.HasPrefix(line, "📈") {
			if t.options.Verbose {
				formatted.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "📈")) + "\n")
			}
		} else {
			formatted.WriteString(line + "\n")
		}
	}

	return formatted.String()
}

// FormatError 格式化错误信息
func (t *TextFormatter) FormatError(err error) string {
	return fmt.Sprintf("[ERROR] %v\n", err)
}

// FormatSuccess 格式化成功信息
func (t *TextFormatter) FormatSuccess(msg string) string {
	return fmt.Sprintf("[SUCCESS] %s\n", msg)
}
