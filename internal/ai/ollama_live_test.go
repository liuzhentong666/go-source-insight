package ai

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestOllamaLiveRAG is opt-in because it loads real local models. Only fictional
// source is sent to Ollama, and vectors are held in memory rather than Milvus.
func TestOllamaLiveRAG(t *testing.T) {
	if os.Getenv("GO_AI_INSIGHT_LIVE_TEST") != "1" {
		t.Skip("set GO_AI_INSIGHT_LIVE_TEST=1 to run the local Ollama smoke test")
	}
	endpoint := strings.TrimSpace(os.Getenv("GO_AI_INSIGHT_OLLAMA_ENDPOINT"))
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	const sample = "package sample\n\n// Multiply returns the product of two integers.\nfunc Multiply(a, b int) int {\n\treturn a * b\n}\n"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	opts := FactoryOptions{
		OllamaEndpoint: endpoint,
		MilvusEndpoint: "localhost:19530", // validated but never connected in this test
		Timeout:        5 * time.Minute,
		RAG: RAGOptions{
			Collection: "live_smoke", EmbeddingModel: "bge-m3:latest", ChatModel: "llama3:latest",
			VectorDim: 1024, TopK: 2, ChunkMaxLines: 120, ChunkMinLines: 5,
		},
	}
	started := time.Now()
	store := &liveMemoryStore{dimension: opts.RAG.VectorDim}
	svc, err := newOllamaRAGService(ctx, opts, func(context.Context, string, string, int) (VectorStore, error) {
		return store, nil
	})
	if err != nil {
		t.Fatalf("initialize real Ollama: %v", err)
	}
	defer svc.Store.Close()
	summary, err := svc.Index(ctx, root)
	if err != nil {
		t.Fatalf("index fictional source: %v", err)
	}
	if summary.FilesScanned != 1 || summary.ChunksCreated == 0 || summary.ChunksIndexed != summary.ChunksCreated {
		t.Fatalf("index counts: files=%d chunks=%d indexed=%d", summary.FilesScanned, summary.ChunksCreated, summary.ChunksIndexed)
	}
	if len(store.vectors) != summary.ChunksIndexed {
		t.Fatal("stored vector count differs from indexed chunk count")
	}
	for _, vector := range store.vectors {
		if len(vector) != 1024 {
			t.Fatal("real embedding dimension differs from bge-m3's configured 1024 dimensions")
		}
	}
	answer, err := svc.Ask(ctx, "What does the Multiply function return? Answer in one short sentence.", "sample.go")
	if err != nil {
		t.Fatalf("ask real Ollama: %v", err)
	}
	if strings.TrimSpace(answer.Answer) == "" {
		t.Fatal("real chat model returned an empty answer")
	}
	if len(answer.Sources) == 0 {
		t.Fatal("answer has no source citations")
	}
	foundFunction := false
	for _, source := range answer.Sources {
		if source.Source != "sample.go" || filepath.IsAbs(source.Source) || source.StartLine < 1 || source.EndLine > 6 || source.StartLine > source.EndLine {
			t.Fatalf("invalid source citation: source=%q lines=%d-%d type=%q", source.Source, source.StartLine, source.EndLine, source.ChunkType)
		}
		if source.ChunkType == "function" && source.StartLine == 3 && source.EndLine == 6 && strings.Contains(source.Content, "return a * b") {
			foundFunction = true
		}
	}
	if !foundFunction {
		t.Fatal("retrieved sources do not include the fictional function and its exact source lines")
	}
	t.Logf("real Ollama smoke passed: 1 file, %d chunks, 1024 dimensions, nonempty answer, sample.go:3-6; elapsed=%s", summary.ChunksIndexed, time.Since(started).Round(time.Millisecond))
}

// liveMemoryStore performs actual cosine ranking so the smoke test exercises the
// vector returned by Ollama while remaining independent of a running database.
type liveMemoryStore struct {
	dimension int
	chunks    []CodeChunk
	vectors   [][]float32
}

func (s *liveMemoryStore) EnsureCollection(ctx context.Context) error { return ctx.Err() }

func (s *liveMemoryStore) Replace(ctx context.Context, chunks []CodeChunk, vectors [][]float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(chunks) != len(vectors) {
		return fmt.Errorf("memory store: mismatched chunk and vector counts")
	}
	for _, vector := range vectors {
		if err := validateVector(vector, s.dimension); err != nil {
			return err
		}
	}
	s.chunks = append([]CodeChunk(nil), chunks...)
	s.vectors = make([][]float32, len(vectors))
	for i, vector := range vectors {
		s.vectors[i] = append([]float32(nil), vector...)
	}
	return nil
}

func (s *liveMemoryStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateVector(query, s.dimension); err != nil {
		return nil, err
	}
	var results []SearchResult
	for i, chunk := range s.chunks {
		if opts.FileFilter != "" && chunk.Source != opts.FileFilter {
			continue
		}
		var dot, queryNorm, vectorNorm float64
		for j, value := range query {
			q, v := float64(value), float64(s.vectors[i][j])
			dot += q * v
			queryNorm += q * q
			vectorNorm += v * v
		}
		score := float32(dot / math.Sqrt(queryNorm*vectorNorm))
		results = append(results, SearchResult{
			ID: chunk.ID, Source: chunk.Source, Content: chunk.Content, Score: score,
			StartLine: chunk.StartLine, EndLine: chunk.EndLine, ChunkType: chunk.ChunkType,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}
	return results, nil
}

func (s *liveMemoryStore) Close() error { return nil }

var _ VectorStore = (*liveMemoryStore)(nil)
