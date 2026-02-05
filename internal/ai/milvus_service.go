package ai

import (
	"context"
	"fmt"
	"github.com/milvus-io/milvus-sdk-go/v2/client" // 引入 Milvus SDK
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"log"
)

//	func InitMilvus(ctx context.Context) client.Client {
//		// 1. 连接 Milvus
//		c, err := client.NewClient(ctx, client.Config{
//			Address: "localhost:19530",
//		})
//		if err != nil {
//			log.Fatal("连接 Milvus 失败:", err)
//		}
//
//		// 2. 定义表结构 (Schema)
//		collectionName := "company_rules"
//		schema := &entity.Schema{
//			CollectionName: collectionName,
//			Description:    "公司规章制度表",
//			Fields: []*entity.Field{
//				// 主键 ID
//				entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true),
//				// 规章内容
//				entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(500),
//				// 向量字段 (注意：768 是 nomic-embed-text 的维度)
//				entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(768),
//			},
//		}
//
//		// 3. 创建 Collection
//		err = c.CreateCollection(ctx, schema, entity.DefaultShardNumber)
//		if err != nil {
//			fmt.Println("表可能已存在:", err)
//		}
//
//		fmt.Println("Milvus 初始化成功，表 company_rules 已就绪")
//		return c
//	}
//
// // InsertRules 将规章制度存入 Milvus
//
//	func InsertRules(ctx context.Context, c client.Client, rules []string, vectors [][]float32) error {
//		collectionName := "company_rules"
//
//		// 1. 准备数据列
//		contentColumn := entity.NewColumnVarChar("content", rules)
//		vectorColumn := entity.NewColumnFloatVector("vector", 768, vectors)
//
//		// 2. 插入数据
//		_, err := c.Insert(ctx, collectionName, "", contentColumn, vectorColumn)
//		if err != nil {
//			return fmt.Errorf("插入数据失败: %v", err)
//		}
//
//		// 3. 【关键步骤】创建索引 (让搜索变快)
//		idx, _ := entity.NewIndexHNSW(entity.COSINE, 16, 64) // 使用 L2 距离算法
//		err = c.CreateIndex(ctx, collectionName, "vector", idx, false)
//
//		// 4. 【关键步骤】加载到内存
//		err = c.LoadCollection(ctx, collectionName, false)
//
//		fmt.Println("数据已成功存入 Milvus 并建立索引")
//		return nil
//	}
//
// // SearchRule 在 Milvus 中搜索最相关的规章
//
//	func SearchRule(ctx context.Context, c client.Client, queryVector []float32) ([]string, error) {
//		collectionName := "company_rules"
//
//		// 搜索参数：搜索 "vector" 字段，找最像的 1 条 (Top-1)
//		searchParam, _ := entity.NewIndexHNSWSearchParam(64)
//
//		res, _ := c.Search(ctx, collectionName, []string{}, "", []string{"content"},
//			[]entity.Vector{entity.FloatVector(queryVector)}, "vector",
//			entity.COSINE, 2, searchParam)
//
//		var results []string
//		if len(res) > 0 {
//			col := res[0].Fields.GetColumn("content")
//			for i := 0; i < res[0].IDs.Len(); i++ {
//				val, _ := col.Get(i)
//				results = append(results, val.(string))
//			}
//		}
//
//		return results, nil
//	}
//
//	func InitUserKnowledge(ctx context.Context) client.Client {
//		m, err := client.NewClient(ctx, client.Config{
//			Address: "localhost:19530",
//		})
//		if err != nil {
//			log.Fatal("连接 Milvus 失败:", err)
//		}
//		fields := []*entity.Field{
//			entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true),
//			entity.NewField().WithName("category").WithDataType(entity.FieldTypeVarChar).WithMaxLength(100),
//			entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(500),
//			entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(768),
//		}
//		schema := &entity.Schema{
//			CollectionName: "user_knowledge",
//			Fields:         fields,
//			Description:    "用户知识库",
//		}
//		err = m.CreateCollection(ctx, schema, entity.DefaultShardNumber)
//		if err != nil {
//			fmt.Printf("表可能已经存在: %v\n", err)
//		}
//		idx, _ := entity.NewIndexHNSW(entity.COSINE, 16, 64)
//		_ = m.CreateIndex(ctx, "user_knowledge", "vector", idx, false)
//		_ = m.LoadCollection(ctx, "user_knowledge", false)
//		fmt.Println("user_knowledge 初始化成功")
//		return m
//	}
//
//	func InsertUserKnowledge(ctx context.Context, m client.Client, category string, content string, vec []float32) error {
//		// 1. 准备“分类”列
//		// 参数意义：("字段名", []数据切片)
//		categoryCol := entity.NewColumnVarChar("category", []string{category})
//
//		// 2. 准备“内容”列
//		contentCol := entity.NewColumnVarChar("content", []string{content})
//
//		// 3. 准备“向量”列
//		// 参数意义：("字段名", 维度, [][]数据切片)
//		// 注意：向量列要求是两层切片，因为你可以一次插入多行
//		vectorCol := entity.NewColumnFloatVector("vector", 768, [][]float32{vec})
//
//		// 4. 执行插入动作
//		// 参数意义：(上下文, "表名", "分区名(选填,通常留空)", 所有的列...)
//		_, err := m.Insert(ctx, "user_knowledge", "", categoryCol, contentCol, vectorCol)
//		if err != nil {
//			return fmt.Errorf("插入数据失败: %v", err)
//		}
//
//		// 5. 习惯性动作：如果是学习环境，可以手动执行一次刷盘（可选）
//		// m.Flush(ctx, "user_knowledge", false)
//
//		fmt.Println("成功往 Milvus 插入了一条知识")
//		return nil
//	}
//
//	func InsertUserKnowledge2(ctx context.Context, m client.Client, category string, content string, vec []float32) error {
//		categoryCol := entity.NewColumnVarChar("category", []string{category})
//		contentCol := entity.NewColumnVarChar("content", []string{content})
//		vectorCol := entity.NewColumnFloatVector("vector", 768, [][]float32{vec})
//		_, err := m.Insert(ctx, "user_knowledge", "", categoryCol, contentCol, vectorCol)
//		if err != nil {
//			return fmt.Errorf("插入数据失败: %v", err)
//		}
//		fmt.Println("成功往 Milvus 插入了一条知识")
//		return nil
//	}
//
//	func SearchUserKnowledge(ctx context.Context, m client.Client, category string, queryVec []float32) (string, error) {
//		searchParam, _ := entity.NewIndexHNSWSearchParam(64)
//		res, err := m.Search(
//			ctx,
//			"user_knowledge",    // 表名
//			[]string{},          // 分区名（留空）
//			"",                  // 过滤条件（比如只要某个 category 的，现在留空）
//			[]string{"content"}, // 【关键】你要数据库返回哪一列的原文字？
//			[]entity.Vector{entity.FloatVector(queryVec)}, // 你的问题向量
//			"vector",      // 数据库里向量列叫什么名字？
//			entity.COSINE, // 用什么算法比对？
//			1,             // 找最像的前几个？(Top-1)
//			searchParam,   // 刚才定义的参数
//		)
//		if err != nil {
//			return "", err
//		}
//		if len(res) > 0 && res[0].IDs.Len() > 0 {
//			contentCol := res[0].Fields.GetColumn("content")
//			val, _ := contentCol.Get(0)
//			return val.(string), nil
//		}
//		return "没找到", nil
//	}
func InitCode(ctx context.Context) client.Client {
	m, err := client.NewClient(ctx, client.Config{
		Address: "localhost:19530",
	})
	if err != nil {
		log.Fatal("连接 Milvus 失败:", err)
	}
	fields := []*entity.Field{
		entity.NewField().WithName("id").WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true),
		entity.NewField().WithName("source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(500),
		entity.NewField().WithName("content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(10000),
		entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(1024),
	}
	schema := &entity.Schema{
		CollectionName: "code_segments",
		Fields:         fields,
		Description:    "用户代码库",
	}
	err = m.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		fmt.Printf("表可能已经存在: %v\n", err)
	}
	idx, _ := entity.NewIndexHNSW(entity.COSINE, 16, 64)
	_ = m.CreateIndex(ctx, "code_segments", "vector", idx, false)
	_ = m.LoadCollection(ctx, "code_segments", false)
	fmt.Println("code_segments 初始化成功")
	return m
}
func InsertCodeChunks(ctx context.Context, m client.Client, sources []string, contents []string, vectors [][]float32) error {
	sourcesCol := entity.NewColumnVarChar("source", sources)
	contentsCol := entity.NewColumnVarChar("content", contents)
	vectorsCol := entity.NewColumnFloatVector("vector", 1024, vectors)
	_, err := m.Insert(ctx, "code_segments", "", sourcesCol, vectorsCol, contentsCol)
	if err != nil {
		return fmt.Errorf("插入数据失败: %v", err)
	}
	err = m.Flush(ctx, "code_segments", false)
	if err != nil {
		return fmt.Errorf("Flush 失败: %v", err)
	}
	return nil
}
