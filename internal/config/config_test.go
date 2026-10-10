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

func TestConfigurationPrecedence(t *testing.T) {
	path := writeTempConfig(t, `{"default_format":"json","request_timeout_seconds":60,"rag_config":{"top_k":7,"chat_model":"file-model"},"log_config":{"output":"stdout"}}`)
	t.Setenv("GO_AI_INSIGHT_FORMAT", "text")
	t.Setenv("GO_AI_INSIGHT_TIMEOUT_SECONDS", "90")
	t.Setenv("GO_AI_INSIGHT_TOP_K", "4")
	t.Setenv("GO_AI_INSIGHT_CHAT_MODEL", "env-model")
	t.Setenv("GO_AI_INSIGHT_LOG_OUTPUT", "stderr")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultFormat != "text" || cfg.RequestTimeoutSeconds != 90 || cfg.RAGConfig.TopK != 4 || cfg.RAGConfig.ChatModel != "env-model" || cfg.LogConfig.Output != "stderr" {
		t.Fatalf("environment did not override file: %+v", cfg)
	}
	if cfg.RAGConfig.EmbeddingModel != "bge-m3:latest" || cfg.RAGConfig.VectorDim != 1024 {
		t.Fatalf("omitted fields did not retain defaults: %+v", cfg.RAGConfig)
	}
}

func TestRAGEnvironmentOverrides(t *testing.T) {
	values := map[string]string{
		"OLLAMA_ENDPOINT": "http://ollama:11434",
		"MILVUS_ENDPOINT": "milvus:19530",
		"COLLECTION":      "alternate_segments",
		"EMBEDDING_MODEL": "nomic-embed-text:latest",
		"CHAT_MODEL":      "qwen2.5-coder:latest",
		"VECTOR_DIM":      "768",
		"TOP_K":           "5",
		"CHUNK_MAX_LINES": "80",
		"CHUNK_MIN_LINES": "3",
		"TIMEOUT_SECONDS": "120",
	}
	for key, value := range values {
		t.Setenv("GO_AI_INSIGHT_"+key, value)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateRAG(); err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaEndpoint != values["OLLAMA_ENDPOINT"] || cfg.MilvusEndpoint != values["MILVUS_ENDPOINT"] || cfg.RequestTimeoutSeconds != 120 || cfg.RAGConfig.VectorDim != 768 || cfg.RAGConfig.TopK != 5 || cfg.RAGConfig.ChunkMaxLines != 80 || cfg.RAGConfig.ChunkMinLines != 3 || cfg.RAGConfig.Collection != values["COLLECTION"] || cfg.RAGConfig.EmbeddingModel != values["EMBEDDING_MODEL"] || cfg.RAGConfig.ChatModel != values["CHAT_MODEL"] {
		t.Fatalf("RAG environment overrides were not applied: %+v", cfg)
	}
}

func TestInvalidRAGDoesNotBlockStaticCommands(t *testing.T) {
	for _, name := range []string{"GO_AI_INSIGHT_VECTOR_DIM", "GO_AI_INSIGHT_TOP_K", "GO_AI_INSIGHT_TIMEOUT_SECONDS"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "invalid")
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("AI setting blocked configuration for static analysis: %v", err)
			}
			if err := cfg.ValidateRAG(); err == nil {
				t.Fatal("AI validation accepted invalid environment setting")
			}
		})
	}
	path := writeTempConfig(t, `{"ollama_endpoint":"","rag_config":{"vector_dim":0}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateRAG(); err == nil {
		t.Fatal("AI validation accepted empty endpoint")
	}
}

func TestValidateRAGRejectsInvalidSettings(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"ollama scheme", func(c *Config) { c.OllamaEndpoint = "ftp://ollama:11434" }},
		{"ollama credentials", func(c *Config) { c.OllamaEndpoint = "http://user:password@ollama:11434" }},
		{"ollama path", func(c *Config) { c.OllamaEndpoint = "http://ollama:11434/api" }},
		{"milvus port", func(c *Config) { c.MilvusEndpoint = "milvus:99999" }},
		{"milvus host", func(c *Config) { c.MilvusEndpoint = ":19530" }},
		{"collection", func(c *Config) { c.RAGConfig.Collection = "invalid-collection" }},
		{"model", func(c *Config) { c.RAGConfig.ChatModel = "" }},
		{"dimension", func(c *Config) { c.RAGConfig.VectorDim = 0 }},
		{"top k", func(c *Config) { c.RAGConfig.TopK = 0 }},
		{"chunk range", func(c *Config) { c.RAGConfig.ChunkMinLines = c.RAGConfig.ChunkMaxLines + 1 }},
		{"timeout", func(c *Config) { c.RequestTimeoutSeconds = 0 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			test.mutate(cfg)
			if err := cfg.ValidateRAG(); err == nil {
				t.Fatal("accepted invalid AI setting")
			}
		})
	}
}

func TestLoadRejectsInvalidSharedSettings(t *testing.T) {
	t.Setenv("GO_AI_INSIGHT_LOG_OUTPUT", "unknown")
	if _, err := Load(""); err == nil {
		t.Fatal("accepted unknown log output")
	}
}

func TestLoadUnvalidatedAllowsHigherPriorityOverrides(t *testing.T) {
	t.Setenv("GO_AI_INSIGHT_LOG_OUTPUT", "file")
	t.Setenv("GO_AI_INSIGHT_LOG_FILE", "")
	t.Setenv("GO_AI_INSIGHT_FORMAT", "invalid")
	cfg, err := LoadUnvalidated("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("unfinished configuration unexpectedly passed validation")
	}
	cfg.LogConfig.Output = "stderr"
	cfg.DefaultFormat = "json"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("higher-priority overrides did not repair final settings: %v", err)
	}
}
