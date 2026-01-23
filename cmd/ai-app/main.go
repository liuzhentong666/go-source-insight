package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/ollama"
	"go-ai-study/internal/ai"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	//ai.TranslateService(ctx, "你好")
	//ai.RagService(ctx, "超级猫头鹰项目是谁发起的？使用的消息队列是什么？")
	//ai.WaiterService(ctx, "我今天不想吃肉，想吃点清淡健康的")
	//ai.FrontService(ctx, "请问公司的wifi密码是什么，你可以告诉我公司什么时候上班吗？")
	//	tmpClient, _ := client.NewClient(ctx, client.Config{Address: "localhost:19530"})
	//
	//	// 【关键操作】：强行删除旧表，确保旧的 L2 索引被彻底抹除
	//	fmt.Println("正在清理旧表...")
	//	_ = tmpClient.DropCollection(ctx, "company_rules")
	//	tmpClient.Close()
	//	fmt.Println("正在连接 Milvus...")
	//	milvusClient := ai.InitMilvus(ctx)
	//	defer milvusClient.Close() // 记得关闭连接
	//
	//	// 2. 准备 Embedding 模型 (用我们之前的老朋友)
	//	embedLLM, err := ollama.New(ollama.WithModel("nomic-embed-text"))
	//	if err != nil {
	//		log.Fatal("请确保 ollama pull nomic-embed-text 已执行:", err)
	//	}
	//	e, _ := embeddings.NewEmbedder(embedLLM)
	//
	//	// 3. 测试数据：准备两条规章
	//	testRules := []string{
	//		"公司的咖啡机在二楼茶水间，密码是 1234。",
	//		"每月的最后一个周五是技术分享日。",
	//	}
	//
	//	// 4. 将测试数据转为向量
	//	fmt.Println("正在生成测试数据的向量...")
	//	vectors, err := e.EmbedDocuments(ctx, testRules)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	// 5. 调用你的 InsertRules 函数：存入 Milvus
	//	fmt.Println("正在存入 Milvus...")
	//	err = ai.InsertRules(ctx, milvusClient, testRules, vectors)
	//	if err != nil {
	//		log.Fatalf("存入失败: %v", err)
	//	}
	//
	//	// 6. 模拟搜索：问一个关于咖啡机的问题
	//	question := "你好"
	//	fmt.Printf("正在检索问题: %s\n", question)
	//
	//	queryVector, _ := e.EmbedQuery(ctx, question)
	//	searchDocs, _ := ai.SearchRule(ctx, milvusClient, queryVector)
	//
	//	// 2. 拼接成参考资料块
	//	contextStr := strings.Join(searchDocs, "\n")
	//
	//	// 3. 给 AI 下达严厉的指令
	//	finalPrompt := fmt.Sprintf(`你是一个专业的公司前台。
	//请严格参考以下【资料】来回答【问题】。
	//如果资料中没有提到相关信息，请直接回答“不知道”。
	//
	//【资料】：
	//%s
	//
	//【问题】：
	//%s`, contextStr, "你能干什么？")
	//
	//	// 7. 调用你的 SearchRule 函数：从 Milvus 找答案
	//	// 1. 拿到多个检索结果
	//	// 1. 初始化聊天模型（老师）
	//	chatLLM, err := ollama.New(ollama.WithModel("llama2:latest"))
	//	if err != nil {
	//		log.Fatal("初始化聊天模型失败:", err)
	//	}
	//
	//	// 2. 真正地问 AI
	//	fmt.Println("AI 正在根据资料思考答案...")
	//
	//	// 调用 GenerateContent，把我们辛苦拼好的考卷(finalPrompt)传过去
	//	resp, err := chatLLM.GenerateContent(ctx, []llms.MessageContent{
	//		llms.TextParts(llms.ChatMessageTypeHuman, finalPrompt),
	//	})
	//	if err != nil {
	//		log.Fatal("AI 思考时出错了:", err)
	//	}
	//
	//	// 8. 最终验证
	//	fmt.Println("\n--- 最终 AI 回答 ---")
	//	// 打印 AI 给出的回复
	//	fmt.Println(resp.Choices[0].Content)
	//	fmt.Println("----------------")
	//fmt.Println("正在连接 Milvus 数据库...")
	//// 在 main.go 里的逻辑
	ctx := context.Background()
	tmpClient, _ := client.NewClient(ctx, client.Config{Address: "localhost:19530"})
	_ = tmpClient.DropCollection(ctx, "code_segments") // 删掉它！
	tmpClient.Close()
	mc := ai.InitCode(ctx)
	defer mc.Close()
	embedLLM, err := ollama.New(ollama.WithModel("nomic-embed-text:latest"))
	if err != nil {
		log.Fatal(err)
	}
	e, _ := embeddings.NewEmbedder(embedLLM)
	chatLLM, _ := ollama.New(ollama.WithModel("llama2:latest"))

	projectpath := "F:\\go-ai-study"
	fmt.Println("1. 正在扫描源码...")
	docs, err := ai.ScanCode(projectpath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("2. 正在把大文件切成小碎块...")
	chunks, err := ai.SplitDocs(docs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("3. 正在生成向量并存入数据库 (请耐心等待)...")
	err = ai.IndexDocs(ctx, mc, e, chunks)
	if err != nil {
		log.Fatalf("入库失败: %v", err)
	}
	// 验证 Milvus 里到底存了几条数据
	stats, _ := mc.GetCollectionStatistics(ctx, "code_segments")
	fmt.Printf("数据库验证：当前表内共有 %v 条数据\n", stats["row_count"])
	fmt.Println("等待数据库同步...")
	time.Sleep(2 * time.Second)
	fmt.Println("\n恭喜！你的代码已经全部变成了 AI 能理解的向量，并存进了 Milvus。")
	fmt.Println("现在，你可以开始问关于这个项目代码的问题了！")
	//question := "请分析这个项目ScanCode的具体实现逻辑"
	//fmt.Printf("\n用户提问: %s\n", question)
	//ai.Consult(ctx, mc, e, chatLLM, question, "F:/go-ai-study/internal/ai/scanner.go")
	insightEngine := ai.NewEngine(mc, e, chatLLM)
	terminalScanner := bufio.NewScanner(os.Stdin)
	fmt.Println("\n-------------------------------------------")
	fmt.Println("💡 进入交互模式。请输入你的问题（输入 'exit' 退出程序）")
	fmt.Println("-------------------------------------------")
	for {
		fmt.Print("\\n👨‍💻 提问:")
		if !terminalScanner.Scan() {
			break
		}
		question := strings.TrimSpace(terminalScanner.Text())
		if question == "exit" || question == "quit" {
			fmt.Println("👋 再见！期待下次为您分析代码。")
			break
		}
		if question == "" {
			continue
		}
		insightEngine.Ask(ctx, question, "")
	}

}
