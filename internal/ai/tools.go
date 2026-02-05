package ai

import (
	"encoding/json"
	"fmt"
	"github.com/tmc/langchaingo/llms"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var TimeTool = llms.Tool{
	Type: "function",
	Function: &llms.FunctionDefinition{
		Name: "get_current_time",
		// 修改 TimeTool 的 Description
		Description: "获取当前准确时间。注意：必须使用 get_current_time 这个名字。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}
var SearchTool = llms.Tool{
	Type: "function",
	Function: &llms.FunctionDefinition{
		Name:        "search_file",
		Description: "当你需要查找项目中某个文件的具体位置时调用",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_name": map[string]any{
					"type":        "string",
					"description": "文件名，例如 scanner.go",
				},
			},
			"required": []string{"file_name"},
		},
	},
}

func WrappedTimeFunc(input string) string {
	return GetCurrentTime()
}

var ToolFunctions = map[string]func(string) string{
	"get_current_time": WrappedTimeFunc,
	"search_file":      WrappedSearchFunc,
}

func SearchFile(name string) string {
	result := "没找到文件"
	root := "F:/go-ai-study"
	targetName := strings.ToLower(name)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 3. 只看文件，不看文件夹
		if !info.IsDir() {
			// 拿到当前文件的名字，也转成小写
			// 4. 比对（只要包含这个名字就算找到了）
			if strings.EqualFold(filepath.Base(path), targetName) {
				result = "找到了！路径在: " + path
				return fmt.Errorf("stop") // 找到了就停下
			}
		}
		return nil
	})
	return result
}

type SearchArgs struct {
	FileName string `json:"file_name"`
	Name     string `json:"name"`
}
type AIInvokeSignal struct {
	ToolCall  string     `json:"tool_call"`
	Arguments SearchArgs `json:"arguments"` // 注意：这里套用了上面的小盒子
}

func GetCurrentTime() string {

	return time.Now().Format("2006-01-02 15:04:05")
}
func WrappedSearchFunc(jsonInput string) string {
	var signal AIInvokeSignal // 使用我们的大包裹结构体
	err := json.Unmarshal([]byte(jsonInput), &signal)
	if err != nil {
		return "解析参数失败: " + err.Error()
	}

	// 从大包裹的 Arguments 字段里拿名字
	finalName := signal.Arguments.FileName
	if finalName == "" {
		finalName = signal.Arguments.Name
	}

	if finalName == "" {
		// 调试用：如果还是空，打印出我们收到的到底是什么
		return fmt.Sprintf("错误：AI 提供的参数盒子里没有名字。收到的 JSON 是: %s", jsonInput)
	}

	return SearchFile(finalName)
}

var TotalTools = []llms.Tool{
	TimeTool,
	SearchTool,
}
