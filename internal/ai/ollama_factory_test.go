package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
)

func factoryTestOptions(endpoint string) FactoryOptions {
	return FactoryOptions{
		OllamaEndpoint: endpoint,
		MilvusEndpoint: "http://milvus:19530/",
		Timeout:        time.Second,
		RAG: RAGOptions{
			Collection: "code_segments", EmbeddingModel: "embed", ChatModel: "chat:latest",
			VectorDim: 3, TopK: 2, ChunkMaxLines: 40, ChunkMinLines: 4,
		},
	}
}

func TestFactoryUsesConfiguredModelsAndSplitter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"embed:latest"},{"model":"chat:latest"}]}`)
		case "/api/embed":
			var request struct {
				Model string `json:"model"`
				Input string `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Model != "embed" || request.Input == "" {
				t.Errorf("invalid embedding request: %+v", request)
			}
			fmt.Fprint(w, `{"embeddings":[[0.1,0.2,0.3]]}`)
		case "/api/chat":
			var request struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Model != "chat:latest" {
				t.Errorf("chat model = %q", request.Model)
			}
			fmt.Fprint(w, `{"message":{"role":"assistant","content":"answer"},"done":true}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	opts := factoryTestOptions(server.URL)
	injectedScanner := &fakeScanner{}
	opts.Scanner = injectedScanner
	store := &fakeStore{}
	svc, err := newOllamaRAGService(context.Background(), opts, func(ctx context.Context, address, collection string, dim int) (VectorStore, error) {
		if address != "milvus:19530" || collection != opts.RAG.Collection || dim != opts.RAG.VectorDim {
			t.Errorf("store options: %s %s %d", address, collection, dim)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("store connection has no timeout")
		}
		return store, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Store.Close()
	if svc.Scanner != injectedScanner {
		t.Fatal("configured source scanner was replaced")
	}
	splitter, ok := svc.Splitter.(*CodeSplitter)
	if !ok || splitter.MaxLines != 40 || splitter.MinLines != 4 {
		t.Fatalf("configured splitter = %+v", svc.Splitter)
	}
	response, err := svc.ChatModel.GenerateContent(context.Background(), []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "test")})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Choices) != 1 || response.Choices[0].Content != "answer" {
		t.Fatalf("response = %+v", response)
	}
}

func TestFactoryRejectsMissingModelBeforeMilvus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"models":[{"name":"embed:latest"}]}`)
	}))
	defer server.Close()
	_, err := newOllamaRAGService(context.Background(), factoryTestOptions(server.URL), func(context.Context, string, string, int) (VectorStore, error) {
		t.Fatal("connected to Milvus before checking missing chat model")
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), `"chat:latest" is not installed`) {
		t.Fatalf("missing model error = %v", err)
	}
}

func TestFactoryRejectsDimensionMismatchBeforeMilvus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			fmt.Fprint(w, `{"models":[{"name":"embed:latest"},{"name":"chat:latest"}]}`)
		case "/api/embed":
			fmt.Fprint(w, `{"embeddings":[[1,2]]}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	_, err := newOllamaRAGService(context.Background(), factoryTestOptions(server.URL), func(context.Context, string, string, int) (VectorStore, error) {
		t.Fatal("connected to Milvus with incompatible embedding dimension")
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "produces 2 dimensions; vector_dim is 3") {
		t.Fatalf("dimension mismatch error = %v", err)
	}
}

func TestFactoryHealthCheckRespectsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	opts := factoryTestOptions(server.URL)
	opts.Timeout = 30 * time.Millisecond
	started := time.Now()
	_, err := NewOllamaRAGService(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "Ollama is unavailable") {
		t.Fatalf("timeout error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("health check exceeded configured timeout")
	}
}

func TestFactoryRejectsInvalidEndpointWithoutNetwork(t *testing.T) {
	opts := factoryTestOptions("http://%invalid")
	if _, err := NewOllamaRAGService(context.Background(), opts); err == nil {
		t.Fatal("invalid endpoint accepted")
	}
}
