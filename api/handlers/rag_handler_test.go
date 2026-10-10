package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go-ai-study/api/models"
	"go-ai-study/api/service"
	"go-ai-study/internal/ai"
	coreconfig "go-ai-study/internal/config"
)

// These tests use the production RAGService, confined scanner, and splitter.
// Only the external model/vector transports and HTTP repository are substituted.
type ragHarness struct {
	mu                             sync.Mutex
	collections                    map[string][]ai.CodeChunk
	opts                           []ai.FactoryOptions
	closed, modelCalls             int
	indexErr, modelErr, factoryErr bool
	indexBlock, askBlock           <-chan struct{}
	indexStarted, askStarted       chan string
	timeout                        int
	afterPublish                   <-chan struct{}
	published                      chan string
	closeErr                       bool
}

func newHarness() *ragHarness {
	return &ragHarness{collections: map[string][]ai.CodeChunk{}, indexStarted: make(chan string, 8), askStarted: make(chan string, 8), published: make(chan string, 16), timeout: 3}
}
func (h *ragHarness) settings() service.RAGSettings {
	return service.RAGSettings{LoadConfig: func() (*coreconfig.Config, error) {
		cfg := coreconfig.DefaultConfig()
		h.mu.Lock()
		cfg.RequestTimeoutSeconds = h.timeout
		h.mu.Unlock()
		cfg.RAGConfig.VectorDim = 2
		cfg.RAGConfig.TopK = 50
		cfg.RAGConfig.ChunkMinLines = 1
		return cfg, nil
	}, Factory: h.factory}
}
func (h *ragHarness) factory(_ context.Context, opts ai.FactoryOptions) (service.RAGClient, error) {
	h.mu.Lock()
	h.opts = append(h.opts, opts)
	fail := h.factoryErr
	h.mu.Unlock()
	splitter := ai.NewCodeSplitter()
	splitter.MinLines = opts.RAG.ChunkMinLines
	splitter.MaxLines = opts.RAG.ChunkMaxLines
	client := ai.NewRAGService(opts.Scanner, splitter, &ragEmbedding{}, &ragVector{h: h, collection: opts.RAG.Collection}, &ragModel{h: h, collection: opts.RAG.Collection}, opts.RAG)
	if fail {
		return client, errors.New("internal provider connection details")
	}
	return client, nil
}

type ragEmbedding struct{}

func (*ragEmbedding) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vectors := make([][]float32, len(texts))
	for i := range vectors {
		vectors[i] = []float32{1, 0}
	}
	return vectors, nil
}
func (*ragEmbedding) EmbedQuery(ctx context.Context, _ string) ([]float32, error) {
	return []float32{1, 0}, ctx.Err()
}

type ragVector struct {
	h          *ragHarness
	collection string
}

func (*ragVector) EnsureCollection(context.Context) error { return nil }
func (v *ragVector) Replace(ctx context.Context, chunks []ai.CodeChunk, _ [][]float32) error {
	v.h.mu.Lock()
	block, fail := v.h.indexBlock, v.h.indexErr
	v.h.mu.Unlock()
	v.h.indexStarted <- v.collection
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail {
		return errors.New("internal vector replacement details")
	}
	v.h.mu.Lock()
	v.h.collections[v.collection] = append([]ai.CodeChunk(nil), chunks...)
	after := v.h.afterPublish
	v.h.mu.Unlock()
	v.h.published <- v.collection
	if after != nil {
		<-after
	}
	return nil
}
func (v *ragVector) Search(ctx context.Context, _ []float32, opts ai.SearchOptions) ([]ai.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v.h.mu.Lock()
	defer v.h.mu.Unlock()
	var results []ai.SearchResult
	for _, chunk := range v.h.collections[v.collection] {
		if opts.FileFilter != "" && opts.FileFilter != chunk.Source {
			continue
		}
		results = append(results, ai.SearchResult{ID: chunk.ID, Source: chunk.Source, Content: chunk.Content, StartLine: chunk.StartLine, EndLine: chunk.EndLine, ChunkType: chunk.ChunkType, Score: 1})
	}
	return results, nil
}
func (v *ragVector) Close() error {
	v.h.mu.Lock()
	defer v.h.mu.Unlock()
	v.h.closed++
	if v.h.closeErr {
		return errors.New("internal closing error")
	}
	return nil
}

type ragModel struct {
	h          *ragHarness
	collection string
}

func (m *ragModel) GenerateContent(ctx context.Context, _ []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	m.h.mu.Lock()
	m.h.modelCalls++
	block, fail := m.h.askBlock, m.h.modelErr
	m.h.mu.Unlock()
	m.h.askStarted <- m.collection
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if fail {
		return nil, errors.New("internal model connection details")
	}
	return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: "answer from " + m.collection}}}, nil
}
func (m *ragModel) Call(ctx context.Context, _ string, _ ...llms.CallOption) (string, error) {
	response, err := m.GenerateContent(ctx, nil)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Content, nil
}

func submitIndex(t *testing.T, f *httpFixture, owner uint, project, target string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"target": target})
	w := f.request(t, http.MethodPost, "/api/v1/projects/"+project+"/index", string(body), owner)
	if w.Code != http.StatusAccepted {
		t.Fatalf("index=%d %s", w.Code, w.Body.String())
	}
	var job models.Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || job.Status != models.JobQueued {
		t.Fatalf("invalid index acceptance: %s", w.Body.String())
	}
	return job.ID
}
func ask(t *testing.T, f *httpFixture, owner uint, project, question, file string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"question": question, "file": file})
	for deadline := time.Now().Add(2 * time.Second); ; {
		w := f.request(t, http.MethodPost, "/api/v1/projects/"+project+"/ask", string(body), owner)
		if w.Code != http.StatusConflict || time.Now().After(deadline) {
			return w
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRAGHTTPIndexAskSourcesAndIsolation(t *testing.T) {
	h := newHarness()
	f := newHTTPFixture(t, h.settings())
	code := "package sample\n//line virtual.go:900\nfunc Helper() {\n println(\"example\")\n}\n"
	if err := os.WriteFile(filepath.Join(f.root, "7", "1", "pkg", "helper.go"), []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	job := f.await(t, submitIndex(t, f, 7, "1", "pkg"))
	if job.Status != models.JobCompleted || job.Kind != models.JobIndex {
		t.Fatalf("index job failed: %+v", job)
	}
	var summary ai.IndexSummary
	if err := json.Unmarshal(job.Result, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Collection != "api_project_1" || summary.TargetPath != "pkg" || summary.FilesScanned != 1 || summary.ChunksIndexed < 1 {
		t.Fatalf("unexpected index: %+v", summary)
	}
	w := ask(t, f, 7, "1", "Explain Helper", "pkg/helper.go")
	if w.Code != http.StatusOK {
		t.Fatalf("ask=%d %s", w.Code, w.Body.String())
	}
	var answer ai.AnswerResult
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, citation := range answer.Sources {
		if citation.Source != "pkg/helper.go" {
			t.Fatalf("source not project-relative: %+v", citation)
		}
		if strings.Contains(citation.Content, "func Helper") {
			found = true
			if citation.StartLine != 2 || citation.EndLine != 5 || !strings.Contains(code, citation.Content) {
				t.Fatalf("physical source changed: %+v", citation)
			}
		}
	}
	if !found || !strings.Contains(answer.Answer, "api_project_1") {
		t.Fatalf("missing answer citation: %+v", answer)
	}
	// Ask works when the source binding has been removed; it uses published data.
	if err := os.Rename(filepath.Join(f.root, "7", "1"), filepath.Join(f.root, "7", "archived")); err != nil {
		t.Fatal(err)
	}
	w = ask(t, f, 7, "1", "Explain Helper", "pkg/helper.go")
	if w.Code != http.StatusOK {
		t.Fatalf("ask required source tree: %d %s", w.Code, w.Body.String())
	}
	w = ask(t, f, 7, "1", "Explain Helper", "missing.go")
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Answer != "" || len(answer.Sources) != 0 {
		t.Fatalf("empty retrieval invented result: %+v", answer)
	}
	w = ask(t, f, 8, "2", "Explain Helper", "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "func Helper") {
		t.Fatalf("project data mixed: %d %s", w.Code, w.Body.String())
	}
	secondRoot := filepath.Join(f.root, "8", "2", "pkg")
	if err := os.MkdirAll(secondRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondRoot, "helper.go"), []byte("package second\nfunc OnlySecond() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	secondJob := f.awaitOwner(t, submitIndex(t, f, 8, "2", "pkg"), 8)
	if secondJob.Status != models.JobCompleted {
		t.Fatalf("second project index: %+v", secondJob)
	}
	w = ask(t, f, 8, "2", "Explain OnlySecond", "pkg/helper.go")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "OnlySecond") || strings.Contains(w.Body.String(), "func Helper") || !strings.Contains(w.Body.String(), "api_project_2") {
		t.Fatalf("independently indexed data mixed: %d %s", w.Code, w.Body.String())
	}
	w = ask(t, f, 7, "1", "Explain Helper", "pkg/helper.go")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "OnlySecond") {
		t.Fatalf("second project overwrote first: %d %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	calls, closed := len(h.opts), h.closed
	opts := append([]ai.FactoryOptions(nil), h.opts...)
	h.mu.Unlock()
	if calls != closed {
		t.Fatalf("RAG clients leaked: calls=%d closed=%d", calls, closed)
	}
	for _, option := range opts {
		if option.RAG.Collection == "code_segments" {
			t.Fatal("default CLI collection used")
		}
	}
	// The indexing scanner must no longer have an open project handle.
	if _, err := opts[0].Scanner.Scan(context.Background(), "."); err == nil {
		t.Fatal("indexing root was not closed")
	}
}

func TestRAGHTTPParametersLazyInitializationAndFailures(t *testing.T) {
	h := newHarness()
	f := newHTTPFixture(t, h.settings())
	for _, route := range []string{"index", "ask"} {
		if w := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, `{"question":"test"}`, 0); w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s=%d", route, w.Code)
		}
		body := `{}`
		if route == "ask" {
			body = `{"question":"test"}`
		}
		if w := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, body, 8); w.Code != http.StatusNotFound {
			t.Fatalf("wrong owner %s=%d", route, w.Code)
		}
	}
	for route, bodies := range map[string][]string{
		"index": {`{"collection":"chosen"}`, `{"ollama_endpoint":"chosen"}`, `{"target":"../other"}`},
		"ask":   {`{}`, `null`, `{"question":" "}`, `{"question":"test","file":"../other"}`, `{"question":"test","collection":"chosen"}`, `{"question":"test","chat_model":"chosen"}`, `{"question":"` + strings.Repeat("q", 4097) + `"}`, `{"question":"` + strings.Repeat("q", 17<<10) + `"}`},
	} {
		for _, body := range bodies {
			if w := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, body, 7); w.Code != http.StatusBadRequest {
				t.Fatalf("invalid %s accepted: %d %s", route, w.Code, w.Body.String())
			}
		}
	}
	if w := f.request(t, http.MethodPost, "/api/v1/projects/1/scan", `{}`, 7); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	f.await(t, f.submit(t, "."))
	h.mu.Lock()
	calls := len(h.opts)
	h.mu.Unlock()
	if calls != 0 {
		t.Fatal("static routes initialized model clients")
	}
	completed := f.await(t, submitIndex(t, f, 7, "1", "pkg"))
	if completed.Status != models.JobCompleted {
		t.Fatal(completed.FailureReason)
	}
	h.mu.Lock()
	h.indexErr = true
	h.mu.Unlock()
	failed := f.await(t, submitIndex(t, f, 7, "1", "pkg"))
	if failed.Status != models.JobFailed || failed.FailureCode != "index_failed" || string(failed.Result) != "null" {
		t.Fatalf("replacement failure misreported: %+v", failed)
	}
	w := ask(t, f, 7, "1", "Explain Helper", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Helper") {
		t.Fatalf("failed replacement lost old data: %d %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	h.modelErr = true
	h.mu.Unlock()
	w = ask(t, f, 7, "1", "Explain Helper", "")
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "ask_failed") || strings.Contains(w.Body.String(), "internal") {
		t.Fatalf("model failure leaked: %d %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	h.modelErr = false
	h.factoryErr = true
	h.mu.Unlock()
	w = ask(t, f, 7, "1", "Explain Helper", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("factory error=%d %s", w.Code, w.Body.String())
	}
	h.mu.Lock()
	calls, closed := len(h.opts), h.closed
	h.mu.Unlock()
	if calls != closed {
		t.Fatalf("failed clients not closed: calls=%d closed=%d", calls, closed)
	}
}

func TestRAGHTTPProjectCoordinationAndCancellation(t *testing.T) {
	h := newHarness()
	release := make(chan struct{})
	h.indexBlock = release
	f := newHTTPFixture(t, h.settings())
	jobID := submitIndex(t, f, 7, "1", "pkg")
	select {
	case <-h.indexStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("index did not start")
	}
	for route, body := range map[string]string{"index": `{}`, "ask": `{"question":"Explain Helper"}`} {
		w := f.request(t, http.MethodPost, "/api/v1/projects/1/"+route, body, 7)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "project_busy") {
			t.Fatalf("same-project conflict %s=%d %s", route, w.Code, w.Body.String())
		}
	}
	w := ask(t, f, 8, "2", "Explain Helper", "")
	if w.Code != http.StatusOK {
		t.Fatalf("different project blocked=%d", w.Code)
	}
	close(release)
	if j := f.await(t, jobID); j.Status != models.JobCompleted {
		t.Fatal(j.FailureReason)
	}
	h.mu.Lock()
	h.indexBlock = nil
	askRelease := make(chan struct{})
	h.askBlock = askRelease
	h.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/1/ask", strings.NewReader(`{"question":"Explain Helper"}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+f.tokens[7])
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { f.router.ServeHTTP(response, req); close(done) }()
	select {
	case <-h.askStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("ask did not start")
	}
	if w := f.request(t, http.MethodPost, "/api/v1/projects/1/index", `{}`, 7); w.Code != http.StatusConflict {
		t.Fatalf("index overlapped ask=%d", w.Code)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP cancellation did not reach RAG")
	}
	if response.Code != http.StatusRequestTimeout || !strings.Contains(response.Body.String(), "canceled") {
		t.Fatalf("canceled ask=%d %s", response.Code, response.Body.String())
	}
	h.mu.Lock()
	h.timeout = 1
	h.mu.Unlock()
	w = ask(t, f, 7, "1", "Explain Helper", "")
	if w.Code != http.StatusGatewayTimeout || !strings.Contains(w.Body.String(), "timeout") {
		t.Fatalf("ask deadline=%d %s", w.Code, w.Body.String())
	}
	close(askRelease)
	h.mu.Lock()
	h.askBlock = nil
	calls, closed := len(h.opts), h.closed
	h.mu.Unlock()
	if calls != closed {
		t.Fatalf("canceled clients leaked: %d/%d", closed, calls)
	}
	if w := ask(t, f, 7, "1", "Explain Helper", ""); w.Code != http.StatusOK {
		t.Fatalf("project gate not released=%d", w.Code)
	}
}
