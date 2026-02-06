package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"log"
	"path/filepath"
	"strings"
)

type SourceInsightEngine struct {
	MilvusClient client.Client
	Embedder     embeddings.Embedder
	ChatModel    llms.Model
	History      []llms.MessageContent
	logger       *Logger
}

func NewEngine(mc client.Client, e embeddings.Embedder, chat llms.Model, logger *Logger) *SourceInsightEngine {
	return &SourceInsightEngine{
		MilvusClient: mc,
		Embedder:     e,
		ChatModel:    chat,
		logger:       logger,
	}
}

func (e *SourceInsightEngine) Ask(ctx context.Context, question string, fileName string) {
	// 1. 【路径标准化】：解决 Windows 斜杠问题
	cleanFileName := filepath.ToSlash(fileName)

	// 2. 【RAG 检索】：从 Milvus 找相关代码
	queryVec, err := e.Embedder.EmbedQuery(ctx, question)
	if err != nil {
		e.logger.Error("向量化失败", "error", err)
		return
	}

	searchParam, _ := entity.NewIndexHNSWSearchParam(64)
	var filterExpr string
	if cleanFileName != "" {
		filterExpr = fmt.Sprintf("source == '%s'", cleanFileName)
	}

	res, err := e.MilvusClient.Search(ctx, "code_segments", []string{}, filterExpr,
		[]string{"content", "source"}, []entity.Vector{entity.FloatVector(queryVec)},
		"vector", entity.COSINE, 3, searchParam)

	if err != nil {
		e.logger.Error("Milvus 搜索失败", "error", err)
		return
	}

	// 3. 【解析 RAG 结果】
	var builder strings.Builder
	if len(res) > 0 && res[0].IDs.Len() > 0 {
		sr := res[0]
		for i := 0; i < sr.IDs.Len(); i++ {
			c, _ := sr.Fields.GetColumn("content").Get(i)
			builder.WriteString(fmt.Sprintf("\n代码片段 %d:\n%s\n", i+1, c))
		}
	}
	relevantCode := builder.String()

	// 4. 【逻辑降噪】：如果是问时间，不传代码干扰 AI
	var finalPrompt string
	if strings.Contains(question, "时间") || strings.Contains(question, "几点") {
		finalPrompt = question
	} else {
		finalPrompt = fmt.Sprintf("参考代码：\n%s\n问题：%s", relevantCode, question)
	}

	// 5. 【构造 System Prompt】：下达死命令
	cleanSystemPrompt := `你是一个代码助手。  
【工具调用法律】：  
1. 查时间必须调用 get_current_time。  
2. 找文件必须调用 search_file。  
3. 如果你要调用工具，请直接发送 JSON 信号。如果你无法发送信号，请在回复中包含 {"tool_call": "工具名", "arguments": {...}} 格式。`

	// 6. 【组装消息流】：System -> History -> Human
	var messages []llms.MessageContent
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, cleanSystemPrompt))
	messages = append(messages, e.History...)
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt))

	// 7. 【第一次呼叫 AI】：开启工具箱
	resp, err := e.ChatModel.GenerateContent(ctx, messages, llms.WithTools(TotalTools))
	if err != nil {
		e.logger.Error("AI 请求失败", "error", err)
		return
	}

	// 检查响应是否有选择项
	if len(resp.Choices) == 0 {
		e.logger.Error("AI 响应中没有选择项")
		return
	}

	choice := resp.Choices[0]
	var toolExecuted bool
	var toolResult string

	// 8. 【双模拦截逻辑】
	// 模式 A：正式信号 (ToolCalls > 0)
	if len(choice.ToolCalls) > 0 {
		e.logger.Info("检测到正式 ToolCall 信号")
		toolCall := choice.ToolCalls[0]
		if fn, ok := ToolFunctions[toolCall.FunctionCall.Name]; ok {
			toolResult = fn(toolCall.FunctionCall.Arguments)
			toolExecuted = true
			// 反馈给 AI 的正式格式
			messages = append(messages, llms.TextParts(llms.ChatMessageTypeAI, choice.Content))
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{llms.ToolCallResponse{
					ToolCallID: toolCall.ID,
					Name:       toolCall.FunctionCall.Name,
					Content:    toolResult,
				}},
			})
		}
	} else if strings.Contains(choice.Content, "{") {
		// 模式 B：手动拦截 (AI 乱打字)
		e.logger.Info("检测到文字中的 JSON 指令，开始智能调度")
		aiSay := choice.Content
		start := strings.Index(aiSay, "{")
		end := strings.LastIndex(aiSay, "}")

		if start != -1 && end != -1 && end > start {
			jsonStr := aiSay[start : end+1]

			// 提取 AI 乱起的工具名
			var temp struct {
				ToolCall string `json:"tool_call"`
			}
			json.Unmarshal([]byte(jsonStr), &temp)
			tName := strings.ToLower(temp.ToolCall)

			// 模糊匹配分发
			if strings.Contains(tName, "time") {
				toolResult = WrappedTimeFunc(jsonStr)
				toolExecuted = true
			} else if strings.Contains(tName, "search") || strings.Contains(tName, "code") || strings.Contains(tName, "file") {
				toolResult = WrappedSearchFunc(jsonStr)
				toolExecuted = true
			}

			if toolExecuted {
				e.logger.Info("手动分发成功", "result", toolResult)
				// 二次闭环需要的消息
				messages = append(messages, llms.TextParts(llms.ChatMessageTypeAI, aiSay))
				messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, "系统反馈工具结果: "+toolResult))
			}
		}
	}

	// 9. 【二次反馈】：如果动用了工具，让 AI 重新组织语言
	if toolExecuted {
		resp, err = e.ChatModel.GenerateContent(ctx, messages)
		if err != nil {
			e.logger.Error("AI 二次请求失败", "error", err)
			return
		}
		// 再次检查响应是否有选择项
		if len(resp.Choices) == 0 {
			e.logger.Error("AI 二次响应中没有选择项")
			return
		}
	}

	// 10. 【存入记忆】：只存人类问题和最终的 AI 回答
	e.History = append(e.History, llms.TextParts(llms.ChatMessageTypeHuman, question))
	e.History = append(e.History, llms.TextParts(llms.ChatMessageTypeAI, resp.Choices[0].Content))

	// 保持记忆不要太长 (只存最近 3 轮对话)
	if len(e.History) > 6 {
		e.History = e.History[2:]
	}

	// 11. 【最终输出】
	fmt.Println("\n🔍 分析报告：")
	fmt.Println(resp.Choices[0].Content)
}