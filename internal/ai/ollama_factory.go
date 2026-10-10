package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

// FactoryOptions describes the runtime dependencies without depending on the CLI
// configuration package. Timeout applies to model requests; connection and model
// availability checks are limited to at most ten seconds.
type FactoryOptions struct {
	OllamaEndpoint string
	MilvusEndpoint string
	RAG            RAGOptions
	Timeout        time.Duration
	// Scanner optionally restricts indexing to an API-owned filesystem.
	Scanner CodeScanner
}

type vectorStoreFactory func(context.Context, string, string, int) (VectorStore, error)

// NewOllamaRAGService connects the local model provider and vector store. It checks
// installed models and the actual embedding dimension before opening Milvus, and
// never creates collections or indexes source code during initialization.
func NewOllamaRAGService(ctx context.Context, opts FactoryOptions) (*RAGService, error) {
	return newOllamaRAGService(ctx, opts, func(ctx context.Context, address, collection string, dim int) (VectorStore, error) {
		return NewMilvusStore(ctx, address, collection, dim)
	})
}

func newOllamaRAGService(ctx context.Context, opts FactoryOptions, createStore vectorStoreFactory) (*RAGService, error) {
	opts.OllamaEndpoint = strings.TrimSpace(opts.OllamaEndpoint)
	opts.MilvusEndpoint = strings.TrimSpace(opts.MilvusEndpoint)
	opts.RAG.EmbeddingModel = strings.TrimSpace(opts.RAG.EmbeddingModel)
	opts.RAG.ChatModel = strings.TrimSpace(opts.RAG.ChatModel)
	if err := validateFactoryOptions(opts); err != nil {
		return nil, err
	}
	embedder, chatModel, err := newOllamaModels(ctx, opts)
	if err != nil {
		return nil, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, connectionTimeout(opts.Timeout))
	defer cancel()
	address := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(opts.MilvusEndpoint), "http://"), "https://"), "/")
	store, err := createStore(connectCtx, address, opts.RAG.Collection, opts.RAG.VectorDim)
	if err != nil {
		return nil, fmt.Errorf("rag: connect to Milvus: %w", err)
	}
	splitter := NewCodeSplitter()
	splitter.MaxLines = opts.RAG.ChunkMaxLines
	splitter.MinLines = opts.RAG.ChunkMinLines
	scanner := opts.Scanner
	if scanner == nil {
		scanner = &ScannerAdapter{}
	}
	return NewRAGService(scanner, splitter, embedder, store, chatModel, opts.RAG), nil
}

func newOllamaModels(ctx context.Context, opts FactoryOptions) (embeddings.Embedder, llms.Model, error) {
	httpClient := &http.Client{Timeout: opts.Timeout}
	if err := checkOllamaModels(ctx, httpClient, opts); err != nil {
		return nil, nil, err
	}
	embedLLM, err := ollama.New(
		ollama.WithServerURL(strings.TrimRight(opts.OllamaEndpoint, "/")),
		ollama.WithHTTPClient(httpClient),
		ollama.WithModel(opts.RAG.EmbeddingModel),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("rag: create Ollama embedding model: %w", err)
	}
	embedder, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, nil, fmt.Errorf("rag: create embedder: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	vector, err := embedder.EmbedQuery(probeCtx, "Go source code")
	if err != nil {
		return nil, nil, fmt.Errorf("rag: check embedding model %q: %w", opts.RAG.EmbeddingModel, err)
	}
	if len(vector) != opts.RAG.VectorDim {
		return nil, nil, fmt.Errorf("rag: embedding model %q produces %d dimensions; vector_dim is %d", opts.RAG.EmbeddingModel, len(vector), opts.RAG.VectorDim)
	}
	chatModel, err := ollama.New(
		ollama.WithServerURL(strings.TrimRight(opts.OllamaEndpoint, "/")),
		ollama.WithHTTPClient(httpClient),
		ollama.WithModel(opts.RAG.ChatModel),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("rag: create Ollama chat model: %w", err)
	}
	return embedder, chatModel, nil
}

func checkOllamaModels(ctx context.Context, client *http.Client, opts FactoryOptions) error {
	checkCtx, cancel := context.WithTimeout(ctx, connectionTimeout(opts.Timeout))
	defer cancel()
	request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, strings.TrimRight(opts.OllamaEndpoint, "/")+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("rag: create Ollama health request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("rag: Ollama is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("rag: Ollama model list returned HTTP %d", response.StatusCode)
	}
	var tags struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&tags); err != nil {
		return fmt.Errorf("rag: invalid Ollama model list: %w", err)
	}
	available := make(map[string]bool, len(tags.Models))
	for _, model := range tags.Models {
		available[canonicalModelName(model.Name)] = true
		available[canonicalModelName(model.Model)] = true
	}
	for _, model := range []string{opts.RAG.EmbeddingModel, opts.RAG.ChatModel} {
		if !available[canonicalModelName(model)] {
			return fmt.Errorf("rag: Ollama model %q is not installed; run ollama pull %s or the deployment setup", model, model)
		}
	}
	return nil
}

func canonicalModelName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if !strings.Contains(name[strings.LastIndex(name, "/")+1:], ":") {
		return name + ":latest"
	}
	return name
}

func connectionTimeout(requestTimeout time.Duration) time.Duration {
	if requestTimeout < 10*time.Second {
		return requestTimeout
	}
	return 10 * time.Second
}

func validateFactoryOptions(opts FactoryOptions) error {
	u, err := url.Parse(strings.TrimSpace(opts.OllamaEndpoint))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("rag: Ollama endpoint must be an http(s) URL without credentials, path or query")
	}
	address := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(opts.MilvusEndpoint), "http://"), "https://"), "/")
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return fmt.Errorf("rag: Milvus endpoint must be host:port")
	}
	if n, err := strconv.Atoi(port); err != nil || n <= 0 || n > 65535 {
		return fmt.Errorf("rag: Milvus port must be between 1 and 65535")
	}
	if opts.Timeout <= 0 {
		return fmt.Errorf("rag: timeout must be positive")
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(opts.RAG.Collection) {
		return fmt.Errorf("rag: collection must be a valid Milvus collection name")
	}
	if strings.TrimSpace(opts.RAG.EmbeddingModel) == "" || strings.TrimSpace(opts.RAG.ChatModel) == "" {
		return fmt.Errorf("rag: embedding and chat model names are required")
	}
	if opts.RAG.VectorDim <= 0 || opts.RAG.TopK <= 0 || opts.RAG.TopK > 16384 {
		return fmt.Errorf("rag: vector dimension must be positive and top_k must be between 1 and 16384")
	}
	if opts.RAG.ChunkMinLines <= 0 || opts.RAG.ChunkMaxLines < opts.RAG.ChunkMinLines {
		return fmt.Errorf("rag: chunk lines must satisfy 0 < min <= max")
	}
	return nil
}
