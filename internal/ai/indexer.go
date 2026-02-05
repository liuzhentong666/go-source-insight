package ai

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
)

func IndexDocs(ctx context.Context, mc client.Client, e embeddings.Embedder, chunks []schema.Document) error {
	var contents []string
	var sources []string
	for _, chunk := range chunks {
		contents = append(contents, chunk.PageContent)
		sources = append(sources, chunk.Metadata["source"].(string))
	}
	fmt.Printf("正在为 %d 个碎块生成向量数字...\n", len(contents))
	vectors, err := e.EmbedDocuments(ctx, contents)
	if err != nil {
		return fmt.Errorf("生成向量失败: %v", err)
	}

	// 调试：检查向量维度
	if len(vectors) > 0 {
		fmt.Printf("✓ 向量生成成功，维度: %d\n", len(vectors[0]))
	} else {
		fmt.Println("✗ 警告：没有生成任何向量！")
	}

	fmt.Println("正在将数据存入 Milvus 数据库...")
	err = InsertCodeChunks(ctx, mc, sources, contents, vectors)
	if err != nil {
		return fmt.Errorf("插入数据失败: %v", err)
	}
	fmt.Println("索引创建完成！AI 现在已经记住你的代码了。")
	return nil
}
