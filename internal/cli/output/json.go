package output

import (
	"encoding/json"
)

// JSONFormatter JSON 格式化器
type JSONFormatter struct{}

// NewJSONFormatter 创建 JSON 格式化器
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format 格式化输出为 JSON
func (j *JSONFormatter) Format(result string) string {
	// 将结果封装为 JSON
	output := map[string]interface{}{
		"success": true,
		"result":  result,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return `{"success": false, "error": "格式化失败"}`
	}

	return string(data)
}
