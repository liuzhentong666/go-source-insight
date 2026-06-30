package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultConfigIncludesRAGConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.RAGConfig.Collection != "code_segments" {
		t.Fatalf("Collection = %q, want %q", cfg.RAGConfig.Collection, "code_segments")
	}
	if cfg.RAGConfig.EmbeddingModel != "bge-m3:latest" {
		t.Fatalf("EmbeddingModel = %q, want %q", cfg.RAGConfig.EmbeddingModel, "bge-m3:latest")
	}
	if cfg.RAGConfig.ChatModel != "llama3:latest" {
		t.Fatalf("ChatModel = %q, want %q", cfg.RAGConfig.ChatModel, "llama3:latest")
	}
	if cfg.RAGConfig.VectorDim != 1024 {
		t.Fatalf("VectorDim = %d, want %d", cfg.RAGConfig.VectorDim, 1024)
	}
	if cfg.RAGConfig.TopK != 3 {
		t.Fatalf("TopK = %d, want %d", cfg.RAGConfig.TopK, 3)
	}
	if cfg.RAGConfig.ChunkMaxLines != 120 {
		t.Fatalf("ChunkMaxLines = %d, want %d", cfg.RAGConfig.ChunkMaxLines, 120)
	}
	if cfg.RAGConfig.ChunkMinLines != 5 {
		t.Fatalf("ChunkMinLines = %d, want %d", cfg.RAGConfig.ChunkMinLines, 5)
	}
}

func TestLoadConfigOverridesRAGConfig(t *testing.T) {
	configPath := writeTempConfig(t, `{
  "rag_config": {
    "collection": "custom_segments",
    "embedding_model": "nomic-embed-text:latest",
    "chat_model": "qwen2.5-coder:latest",
    "vector_dim": 768,
    "top_k": 5,
    "chunk_max_lines": 80,
    "chunk_min_lines": 3
  }
}`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.RAGConfig.Collection != "custom_segments" {
		t.Fatalf("Collection = %q, want %q", cfg.RAGConfig.Collection, "custom_segments")
	}
	if cfg.RAGConfig.EmbeddingModel != "nomic-embed-text:latest" {
		t.Fatalf("EmbeddingModel = %q, want %q", cfg.RAGConfig.EmbeddingModel, "nomic-embed-text:latest")
	}
	if cfg.RAGConfig.ChatModel != "qwen2.5-coder:latest" {
		t.Fatalf("ChatModel = %q, want %q", cfg.RAGConfig.ChatModel, "qwen2.5-coder:latest")
	}
	if cfg.RAGConfig.VectorDim != 768 {
		t.Fatalf("VectorDim = %d, want %d", cfg.RAGConfig.VectorDim, 768)
	}
	if cfg.RAGConfig.TopK != 5 {
		t.Fatalf("TopK = %d, want %d", cfg.RAGConfig.TopK, 5)
	}
	if cfg.RAGConfig.ChunkMaxLines != 80 {
		t.Fatalf("ChunkMaxLines = %d, want %d", cfg.RAGConfig.ChunkMaxLines, 80)
	}
	if cfg.RAGConfig.ChunkMinLines != 3 {
		t.Fatalf("ChunkMinLines = %d, want %d", cfg.RAGConfig.ChunkMinLines, 3)
	}
}

func TestLoadLegacyConfigKeepsDefaultRAGConfig(t *testing.T) {
	configPath := writeTempConfig(t, `{
  "default_output": "stdout",
  "default_format": "json",
  "verbose": true,
  "ollama_endpoint": "http://localhost:11434",
  "milvus_endpoint": "http://localhost:19530"
}`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DefaultFormat != "json" {
		t.Fatalf("DefaultFormat = %q, want %q", cfg.DefaultFormat, "json")
	}
	if cfg.RAGConfig.Collection != "code_segments" {
		t.Fatalf("Collection = %q, want default %q", cfg.RAGConfig.Collection, "code_segments")
	}
	if cfg.RAGConfig.VectorDim != 1024 {
		t.Fatalf("VectorDim = %d, want default %d", cfg.RAGConfig.VectorDim, 1024)
	}
	if cfg.RAGConfig.TopK != 3 {
		t.Fatalf("TopK = %d, want default %d", cfg.RAGConfig.TopK, 3)
	}
}

func TestNormalizeMilvusEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "http URL", endpoint: "http://localhost:19530", want: "localhost:19530"},
		{name: "https URL", endpoint: "https://localhost:19530", want: "localhost:19530"},
		{name: "host port", endpoint: "localhost:19530", want: "localhost:19530"},
		{name: "trailing slash", endpoint: "http://localhost:19530/", want: "localhost:19530"},
		{name: "spaces", endpoint: "  http://localhost:19530  ", want: "localhost:19530"},
		{name: "empty", endpoint: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMilvusEndpoint(tt.endpoint)
			if got != tt.want {
				t.Fatalf("NormalizeMilvusEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
