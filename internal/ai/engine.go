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

type SourceInsightEngine struct {
	MilvusClient client.Client
	Embedder     embeddings.Embedder
	ChatModel    llms.Model
	History      []llms.MessageContent
}

func NewEngine(mc client.Client, e embeddings.Embedder, chat llms.Model) *SourceInsightEngine {
	return &SourceInsightEngine{
		MilvusClient: mc,
		Embedder:     e,
		ChatModel:    chat,
	}
}
func (e *SourceInsightEngine) Ask(ctx context.Context, question string, fileName string) {
	// 1. 【核心修复】：将路径统一转为 Linux 风格的正斜杠 (ToSlash)
	// 这样无论用户传 \ 还是 /，我们都统一处理
	cleanFileName := filepath.ToSlash(fileName)

	queryVec, err := e.Embedder.EmbedQuery(ctx, question)
	if err != nil {
		log.Printf("向量化失败: %v", err)
		return
	}

	// 2. 【核心修复】：精确控制过滤条件
	var filterExpr string
	if cleanFileName != "" {
		filterExpr = fmt.Sprintf("source == '%s'", cleanFileName)
	}
	// 注意：如果 cleanFileName 是空的，filterExpr 保持为空，后面 Search 就不传它了

	searchParam, _ := entity.NewIndexHNSWSearchParam(64)

	// 3. 执行搜索
	res, err := e.MilvusClient.Search(
		ctx,
		"code_segments",
		[]string{},
		filterExpr, // 这里的表达式现在很干净
		[]string{"content", "source"},
		[]entity.Vector{entity.FloatVector(queryVec)},
		"vector",
		entity.COSINE,
		5,
		searchParam,
	)

	if err != nil {
		log.Printf("检索失败: %v", err)
		return
	}
	// 4. 解析结果
	var builder strings.Builder
	if len(res) > 0 && res[0].IDs.Len() > 0 {
		sr := res[0]
		// 打印一下，方便我们调试
		fmt.Printf("成功检索到 %d 条代码片段\n", sr.IDs.Len())

		colContent := sr.Fields.GetColumn("content")
		for i := 0; i < sr.IDs.Len(); i++ {
			c, _ := colContent.Get(i)
			builder.WriteString(fmt.Sprintf("\n片段 %d:\n%s\n", i+1, c))
		}
	}

	relevantCode := builder.String()
	if relevantCode == "" {
		fmt.Println("AI 提示：在代码库中未找到相关逻辑。")
		// 调试小贴士：如果到这里还是空的，说明数据库里存的路径格式有问题
		return
	}

	// 5. 问 AI (保持不变)
	finalPrompt := fmt.Sprintf(`请参考代码回答问题：\n%s\n问题：%s`, relevantCode, question)
	resp, _ := e.ChatModel.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt),
	})
	messages := append(e.History, llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt))
	resp, err = e.ChatModel.GenerateContent(ctx, messages)
	if err != nil {
		log.Printf("AI 生成失败: %v", err)
		return
	}
	e.History = append(e.History, llms.TextParts(llms.ChatMessageTypeHuman, question))
	e.History = append(e.History, llms.TextParts(llms.ChatMessageTypeHuman, resp.Choices[0].Content))
	if len(e.History) >= 9 {
		e.History = e.History[2:]
	}
	fmt.Println("\n🔍 分析报告：")
	fmt.Println(resp.Choices[0].Content)
}
