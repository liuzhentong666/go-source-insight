package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type failSecondLiveInsert struct {
	client.Client
	calls int
}

func TestOllamaMilvusSnapshotLive(t *testing.T) {
	if os.Getenv("GO_AI_INSIGHT_SNAPSHOT_LIVE") != "1" {
		t.Skip("opt-in real Ollama/Milvus snapshot smoke")
	}
	logical := fmt.Sprintf("review_ollama_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	ollamaEndpoint := os.Getenv("GO_AI_INSIGHT_OLLAMA_ENDPOINT")
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://ollama:11434"
	}
	milvusEndpoint := os.Getenv("GO_AI_INSIGHT_MILVUS_ENDPOINT")
	if milvusEndpoint == "" {
		milvusEndpoint = "milvus:19530"
	}
	opts := FactoryOptions{OllamaEndpoint: ollamaEndpoint, MilvusEndpoint: milvusEndpoint, Timeout: 180 * time.Second, RAG: RAGOptions{Collection: logical, EmbeddingModel: "bge-m3:latest", ChatModel: "llama3:latest", VectorDim: 1024, TopK: 2, ChunkMaxLines: 120, ChunkMinLines: 5}}
	svc, err := NewOllamaRAGService(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	store := svc.Store.(*MilvusStore)
	defer func() {
		clean, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = store.client.DropAlias(clean, snapshotAlias(logical))
		collections, e := store.client.ListCollections(clean)
		if e != nil {
			t.Error(e)
			return
		}
		for _, c := range collections {
			if strings.HasPrefix(c.Name, snapshotPrefix(logical)) {
				if e = store.client.DropCollection(clean, c.Name); e != nil {
					t.Error(e)
				}
			}
		}
	}()
	root := t.TempDir()
	file := filepath.Join(root, "sample.go")
	if err = os.WriteFile(file, []byte("package sample\n//line virtual.go:100\nfunc Multiply(a,b int) int { return a*b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Index(ctx, root)
	if err != nil || summary.ChunksIndexed == 0 {
		t.Fatalf("index: %v %v", summary, err)
	}
	fresh, err := NewOllamaRAGService(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	answer, err := fresh.Ask(ctx, "What does Multiply return? Answer briefly.", "sample.go")
	if err != nil || answer == nil || strings.TrimSpace(answer.Answer) == "" || len(answer.Sources) == 0 {
		t.Fatalf("ask: %v %v", answer, err)
	}
	for _, source := range answer.Sources {
		if source.StartLine < 1 || source.EndLine > 4 {
			t.Fatalf("nonphysical citation: %+v", source)
		}
	}
	t.Logf("fresh Ollama answer=%s citations=%d", answer.Answer, len(answer.Sources))
	if _, err = svc.Index(ctx, root); err != nil {
		t.Fatal(err)
	}
	if answer, err = fresh.Ask(ctx, "What does Multiply return?", "missing.go"); err != nil || len(answer.Sources) != 0 {
		t.Fatalf("empty filter: %v %v", answer, err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Index(ctx, root); err != nil {
		t.Fatal(err)
	}
	if answer, err = fresh.Ask(ctx, "What does Multiply return?", ""); err != nil || len(answer.Sources) != 0 {
		t.Fatalf("empty index: %v %v", answer, err)
	}
}

func (f *failSecondLiveInsert) Insert(ctx context.Context, n, p string, c ...entity.Column) (entity.Column, error) {
	f.calls++
	if f.calls == 2 {
		return nil, fmt.Errorf("review injected second-batch failure")
	}
	return f.Client.Insert(ctx, n, p, c...)
}

// Opt-in: synthetic vectors only; never touches existing user collections.
func TestMilvusSnapshotLive(t *testing.T) {
	if os.Getenv("GO_AI_INSIGHT_SNAPSHOT_LIVE") != "1" {
		t.Skip("opt-in real Milvus snapshot test")
	}
	address := os.Getenv("GO_AI_INSIGHT_MILVUS_ENDPOINT")
	if address == "" {
		address = "milvus:19530"
	}
	logical := fmt.Sprintf("review_snapshot_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	s, e := NewMilvusStore(ctx, address, logical, 2)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	defer func() {
		clean, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.client.DropAlias(clean, snapshotAlias(logical))
		collections, e := s.client.ListCollections(clean)
		if e != nil {
			t.Error(e)
			return
		}
		for _, c := range collections {
			n := c.Name
			if n == logical || strings.HasPrefix(n, snapshotPrefix(logical)) {
				if e = s.client.DropCollection(clean, n); e != nil {
					t.Error(e)
				}
			}
		}
	}()
	legacy := &MilvusStore{client: s.client, collection: logical, vectorDim: 2}
	if e = legacy.EnsureCollection(ctx); e != nil {
		t.Fatal(e)
	}
	c, v := snapshotChunks(1, 20)
	if e = legacy.Insert(ctx, c, v); e != nil {
		t.Fatal(e)
	}
	if rows, e := s.Search(ctx, []float32{1, 0}, SearchOptions{}); e != nil || len(rows) != 1 {
		t.Fatalf("legacy query rows=%d err=%v", len(rows), e)
	}
	c, v = snapshotChunks(1200, 60000)
	if e = s.Replace(ctx, c, v); e != nil {
		t.Fatal(e)
	}
	has, e := s.client.HasCollection(ctx, snapshotAlias(logical))
	t.Logf("HasCollection(alias)=%v err=%v", has, e)
	if e != nil || !has {
		t.Fatal("alias discovery unsupported")
	}
	fresh, e := NewMilvusStore(ctx, address, logical, 2)
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close()
	physical, _, e := fresh.existingSnapshot(ctx)
	if e != nil {
		t.Fatal(e)
	}
	reader := &MilvusStore{client: fresh.client, collection: physical, vectorDim: 2}
	if count, e := reader.rowCount(ctx); e != nil || count != 1200 {
		t.Fatalf("72MB count=%d err=%v", count, e)
	}
	if rows, e := fresh.Search(ctx, []float32{1, 0}, SearchOptions{TopK: 1}); e != nil || len(rows) != 1 {
		t.Fatalf("fresh query rows=%d err=%v", len(rows), e)
	}
	t.Log("72MB replacement and fresh alias reader passed")
	oldClient := s.client
	s.client = &failSecondLiveInsert{Client: oldClient}
	e = s.Replace(ctx, c, v)
	s.client = oldClient
	if e == nil {
		t.Fatal("fault injection did not fail")
	}
	current, _, e := fresh.existingSnapshot(ctx)
	if e != nil || current != physical {
		t.Fatalf("failure switched snapshot: %s %v", current, e)
	}
	if count, e := reader.rowCount(ctx); e != nil || count != 1200 {
		t.Fatalf("old snapshot lost: %d %v", count, e)
	}
	t.Log("second-batch failure retained 1200 old rows")
	if has, e := s.client.HasCollection(ctx, logical); e != nil || !has {
		t.Fatal("legacy physical collection removed")
	}
	if e = s.Replace(ctx, nil, nil); e != nil {
		t.Fatal(e)
	}
	if rows, e := fresh.Search(ctx, []float32{1, 0}, SearchOptions{}); e != nil || len(rows) != 0 {
		t.Fatalf("empty snapshot query: %d %v", len(rows), e)
	}
}
