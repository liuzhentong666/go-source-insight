package ai

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

// ---- fakes ----

type fakeScanner struct {
	docs []schema.Document
	err  error
}

func (f *fakeScanner) Scan(_ context.Context, _ string) ([]schema.Document, error) {
	return f.docs, f.err
}

type fakeSplitter struct {
	chunks []schema.Document
	err    error
}

func (f *fakeSplitter) SplitDocuments(_ []schema.Document) ([]schema.Document, error) {
	return f.chunks, f.err
}

type fakeEmbedder struct {
	docVectors  [][]float32
	docErr      error
	queryVector []float32
	queryErr    error
}

func (f *fakeEmbedder) EmbedDocuments(_ context.Context, texts []string) ([][]float32, error) {
	if f.docErr != nil {
		return nil, f.docErr
	}
	return f.docVectors, nil
}

func (f *fakeEmbedder) EmbedQuery(_ context.Context, _ string) ([]float32, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryVector, nil
}

var _ embeddings.Embedder = (*fakeEmbedder)(nil)

type fakeStore struct {
	ensureErr    error
	insertErr    error
	searchResult []SearchResult
	searchErr    error

	insertedChunks  []CodeChunk
	insertedVectors [][]float32
	closed          bool
	ensureCalls     int
	replaceCalls    int
	searchOptions   SearchOptions
}

func (f *fakeStore) EnsureCollection(_ context.Context) error {
	f.ensureCalls++
	return f.ensureErr
}

func (f *fakeStore) Replace(_ context.Context, chunks []CodeChunk, vectors [][]float32) error {
	f.replaceCalls++
	if f.insertErr != nil {
		return f.insertErr
	}
	f.insertedChunks = chunks
	f.insertedVectors = vectors
	return nil
}

func (f *fakeStore) Search(_ context.Context, _ []float32, options SearchOptions) ([]SearchResult, error) {
	f.searchOptions = options
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchResult, nil
}

func (f *fakeStore) Close() error {
	f.closed = true
	return nil
}

var _ VectorStore = (*fakeStore)(nil)

type fakeModel struct {
	content      string
	err          error
	lastMessages []llms.MessageContent
}

func (f *fakeModel) GenerateContent(_ context.Context, messages []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	f.lastMessages = messages
	if f.err != nil {
		return nil, f.err
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: f.content},
		},
	}, nil
}

func (f *fakeModel) Call(_ context.Context, _ string, _ ...llms.CallOption) (string, error) {
	return f.content, f.err
}

var _ llms.Model = (*fakeModel)(nil)

// ---- helpers ----

func newTestRAGService(scanner CodeScanner, splitter DocumentSplitter, embedder embeddings.Embedder, store VectorStore, model llms.Model) *RAGService {
	return NewRAGService(scanner, splitter, embedder, store, model, RAGOptions{
		Collection: "test_collection",
		VectorDim:  2,
		TopK:       3,
	})
}

// ---- Index tests ----

func TestRAGService_Index_EmptyPath(t *testing.T) {
	svc := newTestRAGService(&fakeScanner{}, &fakeSplitter{}, &fakeEmbedder{}, &fakeStore{}, &fakeModel{})

	_, err := svc.Index(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty targetPath, got nil")
	}
}

func TestRAGService_Index_NoDocs(t *testing.T) {
	store := &fakeStore{insertedChunks: []CodeChunk{{Source: "deleted.go", Content: "func Old() {}"}}}
	svc := newTestRAGService(&fakeScanner{docs: nil}, &fakeSplitter{}, &fakeEmbedder{}, store, &fakeModel{})

	summary, err := svc.Index(context.Background(), "./somepath")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.FilesScanned != 0 {
		t.Fatalf("FilesScanned = %d, want 0", summary.FilesScanned)
	}
	if summary.ChunksCreated != 0 || summary.ChunksIndexed != 0 {
		t.Fatalf("expected zero chunks, got created=%d indexed=%d", summary.ChunksCreated, summary.ChunksIndexed)
	}
	if store.replaceCalls != 1 || len(store.insertedChunks) != 0 {
		t.Fatal("empty repository did not clear the stale index")
	}
}

func TestRAGService_Index_EmbeddingFailure(t *testing.T) {
	docs := []schema.Document{{PageContent: "package main", Metadata: map[string]any{"source": "main.go"}}}
	chunks := []schema.Document{{PageContent: "package main", Metadata: map[string]any{"source": "main.go", "chunk_type": "text", "start_line": 1, "end_line": 1}}}

	svc := newTestRAGService(
		&fakeScanner{docs: docs},
		&fakeSplitter{chunks: chunks},
		&fakeEmbedder{docErr: errors.New("embedding backend down")},
		&fakeStore{},
		&fakeModel{},
	)

	_, err := svc.Index(context.Background(), "./somepath")
	if err == nil {
		t.Fatal("expected error when embedding fails, got nil")
	}
}

func TestRAGService_Index_Success(t *testing.T) {
	docs := []schema.Document{{PageContent: "package main", Metadata: map[string]any{"source": "main.go"}}}
	chunks := []schema.Document{
		{PageContent: "func A() {}", Metadata: map[string]any{"source": "main.go", "chunk_type": "function", "start_line": 1, "end_line": 3}},
		{PageContent: "func B() {}", Metadata: map[string]any{"source": "main.go", "chunk_type": "function", "start_line": 4, "end_line": 6}},
	}
	vectors := [][]float32{{0.1, 0.2}, {0.3, 0.4}}

	store := &fakeStore{}
	svc := newTestRAGService(
		&fakeScanner{docs: docs},
		&fakeSplitter{chunks: chunks},
		&fakeEmbedder{docVectors: vectors},
		store,
		&fakeModel{},
	)

	summary, err := svc.Index(context.Background(), "./somepath")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if summary.FilesScanned != 1 {
		t.Fatalf("FilesScanned = %d, want 1", summary.FilesScanned)
	}
	if summary.ChunksCreated != 2 {
		t.Fatalf("ChunksCreated = %d, want 2", summary.ChunksCreated)
	}
	if summary.ChunksIndexed != 2 {
		t.Fatalf("ChunksIndexed = %d, want 2", summary.ChunksIndexed)
	}
	if summary.Collection != "test_collection" {
		t.Fatalf("Collection = %q, want %q", summary.Collection, "test_collection")
	}
	if len(store.insertedChunks) != 2 {
		t.Fatalf("store received %d chunks, want 2", len(store.insertedChunks))
	}
	if store.insertedChunks[0].Source != "main.go" {
		t.Fatalf("chunk[0].Source = %q, want %q", store.insertedChunks[0].Source, "main.go")
	}
	if store.insertedChunks[0].ChunkType != "function" {
		t.Fatalf("chunk[0].ChunkType = %q, want %q", store.insertedChunks[0].ChunkType, "function")
	}
	if len(store.insertedVectors) != 2 {
		t.Fatalf("store received %d vectors, want 2", len(store.insertedVectors))
	}
}

// ---- Ask tests ----

func TestRAGService_Ask_EmptyQuestion(t *testing.T) {
	svc := newTestRAGService(&fakeScanner{}, &fakeSplitter{}, &fakeEmbedder{}, &fakeStore{}, &fakeModel{})

	_, err := svc.Ask(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for empty question, got nil")
	}
}

func TestRAGService_Ask_SearchNoResults(t *testing.T) {
	model := &fakeModel{content: "should not be called"}
	svc := newTestRAGService(
		&fakeScanner{},
		&fakeSplitter{},
		&fakeEmbedder{queryVector: []float32{0.1, 0.2}},
		&fakeStore{searchResult: nil},
		model,
	)

	result, err := svc.Ask(context.Background(), "什么是 ToolManager?", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Answer != "" {
		t.Fatalf("Answer = %q, want empty", result.Answer)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("Sources = %v, want empty", result.Sources)
	}
	if len(model.lastMessages) != 0 {
		t.Fatal("ChatModel was called but should not be for empty search results")
	}
}

func TestRAGService_Ask_Success(t *testing.T) {
	searchResults := []SearchResult{
		{
			ID:        "tool_manager.go:45-78#0",
			Source:    "tool_manager.go",
			Content:   "func (tm *ToolManager) Run(...) {...}",
			Score:     0.9,
			StartLine: 45,
			EndLine:   78,
			ChunkType: "function",
		},
	}

	model := &fakeModel{content: "ToolManager.Run 使用双通道错误模型。"}
	svc := newTestRAGService(
		&fakeScanner{},
		&fakeSplitter{},
		&fakeEmbedder{queryVector: []float32{0.1, 0.2}},
		&fakeStore{searchResult: searchResults},
		model,
	)

	result, err := svc.Ask(context.Background(), "ToolManager.Run 的错误模型是什么？", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Answer != "ToolManager.Run 使用双通道错误模型。" {
		t.Fatalf("Answer = %q, unexpected", result.Answer)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("Sources length = %d, want 1", len(result.Sources))
	}
	if result.Sources[0].Source != "tool_manager.go" {
		t.Fatalf("Sources[0].Source = %q, want %q", result.Sources[0].Source, "tool_manager.go")
	}
	if result.Sources[0].StartLine != 45 || result.Sources[0].EndLine != 78 {
		t.Fatalf("Sources[0] line range = %d-%d, want 45-78", result.Sources[0].StartLine, result.Sources[0].EndLine)
	}
	if result.Sources[0].ChunkType != "function" {
		t.Fatalf("Sources[0].ChunkType = %q, want %q", result.Sources[0].ChunkType, "function")
	}
	if len(model.lastMessages) != 1 || len(model.lastMessages[0].Parts) != 1 {
		t.Fatalf("model received unexpected messages: %#v", model.lastMessages)
	}
	promptPart, ok := model.lastMessages[0].Parts[0].(llms.TextContent)
	if !ok {
		t.Fatalf("model prompt part type = %T, want llms.TextContent", model.lastMessages[0].Parts[0])
	}
	if !strings.Contains(promptPart.Text, "行号: 45-78") {
		t.Fatalf("prompt does not contain line reference: %q", promptPart.Text)
	}
}

func TestRAGService_Index_FailurePreservesExistingIndex(t *testing.T) {
	chunk := schema.Document{PageContent: "func Current() {}", Metadata: map[string]any{"source": "main.go", "chunk_type": "function", "start_line": 1, "end_line": 1}}
	tests := []struct {
		name    string
		vectors [][]float32
		err     error
	}{
		{name: "backend failure", err: errors.New("embedding unavailable")},
		{name: "count mismatch", vectors: nil},
		{name: "dimension mismatch", vectors: [][]float32{{1}}},
		{name: "zero vector", vectors: [][]float32{{0, 0}}},
		{name: "NaN vector", vectors: [][]float32{{float32(math.NaN()), 1}}},
		{name: "infinite vector", vectors: [][]float32{{1, float32(math.Inf(1))}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{insertedChunks: []CodeChunk{{Source: "old.go"}}}
			svc := newTestRAGService(&fakeScanner{docs: []schema.Document{chunk}}, &fakeSplitter{chunks: []schema.Document{chunk}}, &fakeEmbedder{docVectors: tt.vectors, docErr: tt.err}, store, nil)
			if _, err := svc.Index(context.Background(), "project"); err == nil {
				t.Fatal("expected index failure")
			}
			if store.ensureCalls != 0 || store.replaceCalls != 0 || store.insertedChunks[0].Source != "old.go" {
				t.Fatal("embedding failure changed the existing index")
			}
		})
	}
}

func TestRAGService_Index_ReplaceRemovesDeletedFiles(t *testing.T) {
	first := schema.Document{PageContent: "func First() {}", Metadata: map[string]any{"source": "first.go", "chunk_type": "function", "start_line": 1, "end_line": 1}}
	second := schema.Document{PageContent: "func Second() {}", Metadata: map[string]any{"source": "second.go", "chunk_type": "function", "start_line": 1, "end_line": 1}}
	store := &fakeStore{}
	scanner, splitter := &fakeScanner{docs: []schema.Document{first, second}}, &fakeSplitter{chunks: []schema.Document{first, second}}
	embedder := &fakeEmbedder{docVectors: [][]float32{{1, 0}, {0, 1}}}
	svc := newTestRAGService(scanner, splitter, embedder, store, nil)
	if _, err := svc.Index(context.Background(), "project"); err != nil {
		t.Fatal(err)
	}
	scanner.docs, splitter.chunks, embedder.docVectors = []schema.Document{second}, []schema.Document{second}, [][]float32{{0, 1}}
	for i := 0; i < 2; i++ {
		if _, err := svc.Index(context.Background(), "project"); err != nil {
			t.Fatal(err)
		}
		if len(store.insertedChunks) != 1 || store.insertedChunks[0].Source != "second.go" {
			t.Fatalf("deleted or duplicate chunks remain: %v", store.insertedChunks)
		}
	}
}

func TestRAGService_Ask_EmptyContentDoesNotCallModelOrCreateIndex(t *testing.T) {
	store := &fakeStore{searchResult: []SearchResult{{Content: " \n\t"}}}
	model := &fakeModel{content: "invented answer"}
	svc := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, model)
	answer, err := svc.Ask(context.Background(), "what is available?", "")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Answer != "" || len(answer.Sources) != 0 || len(model.lastMessages) != 0 {
		t.Fatalf("empty retrieved content produced an answer: %+v", answer)
	}
	if store.ensureCalls != 0 || store.replaceCalls != 0 {
		t.Fatal("ask attempted to create or rebuild an index")
	}
}

func TestRAGService_Ask_EmptyModelAnswerIsAnError(t *testing.T) {
	svc := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, &fakeStore{searchResult: []SearchResult{{Content: "package demo"}}}, &fakeModel{})
	if _, err := svc.Ask(context.Background(), "what package?", ""); err == nil {
		t.Fatal("expected an error for an empty model answer")
	}
}

func TestRAGService_CloseAndCanceledContext(t *testing.T) {
	store := &fakeStore{}
	svc := newTestRAGService(&fakeScanner{}, &fakeSplitter{}, &fakeEmbedder{}, store, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Index(ctx, "project"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled index error = %v", err)
	}
	if _, err := svc.Ask(ctx, "question", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled ask error = %v", err)
	}
	if err := svc.Close(); err != nil || !store.closed {
		t.Fatalf("Close did not release the store: %v", err)
	}
	if err := (*RAGService)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRAGService_AskRecallsAndRanksExplicitFunction(t *testing.T) {
	store := &fakeStore{searchResult: []SearchResult{
		{ID: "package", Content: "package source", Score: 0.95, Source: "scanner.go"},
		{ID: "import", Content: "import (\"os\"; \"io/fs\")", Score: 0.9, Source: "scanner.go"},
		{ID: "type", Content: "type ScanResult struct { Files []string }", Score: 0.85, Source: "scanner.go"},
		{ID: "function", Content: "func Scan(ctx context.Context, target string) (ScanResult, error) {\n info, err := os.Lstat(target)\n if info.Mode()&os.ModeSymlink != 0 { return ScanResult{}, err }\n return ScanResult{}, nil\n}", Score: 0.8, Source: "scanner.go", StartLine: 3, EndLine: 7, ChunkType: "function"},
	}}
	model := &fakeModel{content: "Lstat inspects the link itself; ModeSymlink detects symbolic links."}
	service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, model)
	answer, err := service.Ask(context.Background(), "Scan函数中os.Lstat和os.ModeSymlink作用", "scanner.go")
	if err != nil {
		t.Fatal(err)
	}
	if store.searchOptions.TopK != 12 || store.searchOptions.FileFilter != "scanner.go" {
		t.Fatalf("expanded recall lost candidate limit/filter: %+v", store.searchOptions)
	}
	if len(answer.Sources) != 3 || answer.Sources[0].ID != "function" {
		t.Fatalf("function at vector rank 4 was not included in final top 3: %+v", answer.Sources)
	}
	if answer.Sources[0].StartLine != 3 || answer.Sources[0].EndLine != 7 {
		t.Fatal("reranking lost source line references")
	}
	part := model.lastMessages[0].Parts[0].(llms.TextContent)
	if !strings.Contains(part.Text, "func Scan(") || !strings.Contains(part.Text, "行号: 3-7") {
		t.Fatalf("model did not receive the requested function and references: %q", part.Text)
	}
}

func TestRAGService_AskSemanticQuestionsPreserveVectorOrder(t *testing.T) {
	for _, question := range []string{"扫描逻辑如何工作？", "What does the function do?", "Explain how Go source files are scanned"} {
		t.Run(question, func(t *testing.T) {
			store := &fakeStore{searchResult: []SearchResult{
				{ID: "first", Content: "package source", Score: 0.9},
				{ID: "second", Content: "func Scan() {}", Score: 0.8},
				{ID: "third", Content: "type ScanResult struct {}", Score: 0.7},
				{ID: "fourth", Content: "var Current = 1", Score: 0.6},
			}}
			service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, &fakeModel{content: "answer"})
			answer, err := service.Ask(context.Background(), question, "")
			if err != nil {
				t.Fatal(err)
			}
			if store.searchOptions.TopK != 3 {
				t.Fatalf("ordinary semantic question expanded candidates: %+v", store.searchOptions)
			}
			if len(answer.Sources) != 3 || answer.Sources[0].ID != "first" || answer.Sources[1].ID != "second" || answer.Sources[2].ID != "third" {
				t.Fatalf("semantic ranking or source limit changed: %+v", answer.Sources)
			}
		})
	}
}

func TestCodeSymbolRankingUsesIdentifierBoundariesAndDefinitions(t *testing.T) {
	for _, content := range []string{"ScanResult", "SomeScan", "_Scan", "Scan_", "Scan中文", "中文Scan"} {
		if containsCodeSymbol(content, "Scan") {
			t.Errorf("Scan incorrectly matched %q", content)
		}
	}
	for _, content := range []string{"func Scan() {}", "source.Scan()", "// Scan performs a scan", "(Scan)"} {
		if !containsCodeSymbol(content, "Scan") {
			t.Errorf("Scan did not match %q", content)
		}
	}
	if containsCodeSymbol("os.LstatExtra(target)", "os.Lstat") || !containsCodeSymbol("os.Lstat(target)", "os.Lstat") {
		t.Fatal("qualified identifier boundary matching is incorrect")
	}
	results := []SearchResult{
		{ID: "prefix", Content: "func ScanResult() {}"},
		{ID: "unicode prefix", Content: "func Scan中文() {}"},
		{ID: "call", Content: "func Use() { Scan() }"},
		{ID: "definition", Content: "func (s *Scanner) Scan() {}"},
	}
	ranked := rankCodeCandidates(results, []string{"Scan"}, 3)
	if ranked[0].ID != "definition" || ranked[1].ID != "call" || ranked[2].ID != "prefix" {
		t.Fatalf("definition/boundary/stable tie ranking incorrect: %+v", ranked)
	}
}

func TestQuestionCodeSymbolsAndRecallLimit(t *testing.T) {
	for _, test := range []struct {
		question string
		want     []string
	}{
		{question: "How does RAGService.Ask work?", want: []string{"RAGService.Ask"}},
		{question: "Explain `scan` and main()", want: []string{"scan", "main"}},
		{question: "What is the purpose of this Go function?", want: nil},
		{question: "Scan如何使用ScanResult？", want: []string{"Scan", "ScanResult"}},
	} {
		got := questionCodeSymbols(test.question)
		if strings.Join(got, ",") != strings.Join(test.want, ",") {
			t.Errorf("symbols(%q)=%v, want %v", test.question, got, test.want)
		}
	}
	store := &fakeStore{}
	service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, nil)
	service.Config.TopK = 5000
	if _, err := service.Ask(context.Background(), "Scan function", ""); err != nil {
		t.Fatal(err)
	}
	if store.searchOptions.TopK != 16384 {
		t.Fatalf("expanded candidates exceeded the Milvus limit: %+v", store.searchOptions)
	}
}

func TestRAGService_AskSymbolSearchFailureRemainsAnError(t *testing.T) {
	want := errors.New("search unavailable")
	store := &fakeStore{searchErr: want}
	model := &fakeModel{content: "must not be called"}
	service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, model)
	if _, err := service.Ask(context.Background(), "Scan函数如何工作？", ""); !errors.Is(err, want) {
		t.Fatalf("search error was lost during reranking: %v", err)
	}
	if len(model.lastMessages) != 0 {
		t.Fatal("model was called after a failed search")
	}
}

func TestRAGService_AskDefaultsNonpositiveTopK(t *testing.T) {
	for _, topK := range []int{0, -1} {
		store := &fakeStore{searchResult: []SearchResult{
			{ID: "one", Content: "func Scan() {}"},
			{ID: "two", Content: "package source"},
			{ID: "three", Content: "var Value = 1"},
			{ID: "four", Content: "type Result struct {}"},
		}}
		service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, &fakeModel{content: "answer"})
		service.Config.TopK = topK
		answer, err := service.Ask(context.Background(), "Scan函数如何工作？", "")
		if err != nil || len(answer.Sources) != defaultTopK || store.searchOptions.TopK != defaultTopK*4 {
			t.Fatalf("TopK=%d: answer=%+v options=%+v err=%v", topK, answer, store.searchOptions, err)
		}
		if len(rankCodeCandidates(store.searchResult, nil, topK)) != defaultTopK {
			t.Fatalf("rank helper did not default nonpositive TopK=%d", topK)
		}
	}
}
