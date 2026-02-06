package ai

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"log"
	"path/filepath"
	"strings"
)

func Consult(ctx context.Context, mc client.Client, e embeddings.Embedder, chatLLM llms.Model, question string, targetFileName string, logger *Logger) {
	logger.Info("正在理解您的问题...")
	queryVec, _ := e.EmbedQuery(ctx, question)
	logger.Info("正在从代码库中寻找相关片段...")
	searchParam, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		logger.Error("搜索失败", "error", err)
		return
	}
	filterExpr := fmt.Sprintf("source == '%s'", filepath.ToSlash(targetFileName))
	res, err := mc.Search(ctx, "code_segments", []string{}, filterExpr, []string{"content"},
		[]entity.Vector{entity.FloatVector(queryVec)}, "vector",
		entity.COSINE, 3, searchParam)
	if err != nil {
		logger.Error("搜索失败", "error", err)
		return
	}
	var builder strings.Builder
	if len(res) > 0 {
		searchResult := res[0]
		logger.Info("查到相关片段", "count", searchResult.IDs.Len())
		col := res[0].Fields.GetColumn("content")
		for i := 0; i < res[0].IDs.Len(); i++ {
			val, _ := col.Get(i)
			score := searchResult.Scores[i] // 获取分数
			logger.Info("片段信息", "index", i+1, "score", fmt.Sprintf("%.4f", score))
			builder.WriteString(fmt.Sprintf("代码片段 %d:\n%s\n", i+1, val))
		}
	}
	relevantCode := builder.String()
	// 增加这行打印，看看数据库到底给了 AI 什么资料
	fmt.Println("--- 数据库检索到的参考代码如下 ---")
	fmt.Println(relevantCode)
	fmt.Println("-------------------------------")
	finalPrompt := fmt.Sprintf(`你是一个资深 Go 语言架构师。
请参考以下从项目中搜索到的【代码片段】来回答【问题】。
如果代码中没有相关逻辑，请直接说"我在当前代码库中没找到相关实现"。

【代码片段】：
%s

【问题】：
%s`, relevantCode, question)
	logger.Info("AI 正在组织语言，请稍候...")
	resp, err := chatLLM.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt),
	})

	if err != nil {
		logger.Error("AI 思考失败", "error", err)
		return
	}

	fmt.Println("\n--- 源码专家分析结果 ---")
	fmt.Println(resp.Choices[0].Content)
	fmt.Println("-----------------------")
}
