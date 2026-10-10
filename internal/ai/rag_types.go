package ai

import (
	"context"

	"github.com/tmc/langchaingo/schema"
)

// RAGOptions RAGService 的运行时配置。
// 独立于 internal/config.RAGConfig，internal/ai 不依赖 internal/config，
// 由 index/ask 命令负责做 config.RAGConfig -> RAGOptions 的转换。
type RAGOptions struct {
	Collection     string
	EmbeddingModel string
	ChatModel      string
	VectorDim      int
	TopK           int
	ChunkMaxLines  int
	ChunkMinLines  int
}

// CodeChunk 写入向量库的最小数据单元。
type CodeChunk struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	ChunkType string `json:"chunk_type"`
}

// IndexSummary Index 方法的结果摘要。
type IndexSummary struct {
	TargetPath    string `json:"target_path"`
	FilesScanned  int    `json:"files_scanned"`
	ChunksCreated int    `json:"chunks_created"`
	ChunksIndexed int    `json:"chunks_indexed"`
	Collection    string `json:"collection"`
}

// SearchOptions 检索参数。
type SearchOptions struct {
	TopK       int
	FileFilter string
}

// SearchResult 单条检索结果。
// 保留 chunk 的来源、行号和类型，供回答引用。
type SearchResult struct {
	ID        string  `json:"id"`
	Source    string  `json:"source"`
	Content   string  `json:"content"`
	Score     float32 `json:"score"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	ChunkType string  `json:"chunk_type"`
}

// AnswerResult Ask 方法的结果。
type AnswerResult struct {
	Question string         `json:"question"`
	Answer   string         `json:"answer"`
	Sources  []SearchResult `json:"sources"`
}

// CodeScanner 扫描源码并产出待切块的文档。
type CodeScanner interface {
	Scan(ctx context.Context, rootPath string) ([]schema.Document, error)
}

// DocumentSplitter 将文档切分为带 metadata 的 chunk。
type DocumentSplitter interface {
	SplitDocuments(docs []schema.Document) ([]schema.Document, error)
}

// VectorStore 向量库的最小抽象，便于用 fake 替换真实 Milvus。
type VectorStore interface {
	EnsureCollection(ctx context.Context) error
	// Replace replaces all chunks in this collection, including removing deleted files.
	// Implementations must validate all input before altering existing data.
	Replace(ctx context.Context, chunks []CodeChunk, vectors [][]float32) error
	Search(ctx context.Context, queryVector []float32, opts SearchOptions) ([]SearchResult, error)
	Close() error
}
