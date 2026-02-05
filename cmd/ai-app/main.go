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
	ctx := context.Background()
	tmpClient, _ := client.NewClient(ctx, client.Config{Address: "localhost:19530"})
	_ = tmpClient.DropCollection(ctx, "code_segments") // 删掉它！
	tmpClient.Close()
	mc := ai.InitCode(ctx)
	defer mc.Close()
	embedLLM, err := ollama.New(ollama.WithModel("bge-m3:latest"))
	if err != nil {
		log.Fatal(err)
	}
	e, _ := embeddings.NewEmbedder(embedLLM)
	chatLLM, _ := ollama.New(ollama.WithModel("llama3:latest"))

	projectpath := "F:\\go-ai-study"
	fmt.Println("1. 正在扫描源码...")
	docs, err := ai.ScanCode(projectpath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("2. 正在把大文件切成小碎块...")
	codeSplitter := ai.NewCodeSplitter()
	chunks, err := codeSplitter.SplitDocuments(docs)
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
