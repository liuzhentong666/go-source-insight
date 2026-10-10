package output

import "encoding/json"

type JSONFormatter struct{}

func NewJSONFormatter() *JSONFormatter { return &JSONFormatter{} }

// Format keeps structured results as JSON objects, rather than escaped strings.
func (j *JSONFormatter) Format(result string) string {
	var value any = result
	if json.Valid([]byte(result)) {
		value = json.RawMessage(result)
	}
	data, err := json.MarshalIndent(map[string]any{"success": true, "result": value}, "", "  ")
	if err != nil {
		return "{\"success\":false,\"error\":\"格式化失败\"}"
	}
	return string(data)
}
