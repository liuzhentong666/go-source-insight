package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Config 应用配置
type Config struct {
	DefaultOutput         string    `json:"default_output"`
	DefaultFormat         string    `json:"default_format"`
	Verbose               bool      `json:"verbose"`
	OllamaEndpoint        string    `json:"ollama_endpoint"`
	MilvusEndpoint        string    `json:"milvus_endpoint"`
	RequestTimeoutSeconds int       `json:"request_timeout_seconds"`
	RAGConfig             RAGConfig `json:"rag_config"`
	LogConfig             LogConfig `json:"log_config"`
	ragEnvironmentError   error
}

// RAGConfig RAG 功能配置
type RAGConfig struct {
	Collection     string `json:"collection"`
	EmbeddingModel string `json:"embedding_model"`
	ChatModel      string `json:"chat_model"`
	VectorDim      int    `json:"vector_dim"`
	TopK           int    `json:"top_k"`
	ChunkMaxLines  int    `json:"chunk_max_lines"`
	ChunkMinLines  int    `json:"chunk_min_lines"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `json:"level"`     // debug, info, warn, error
	Format   string `json:"format"`    // text, json
	Output   string `json:"output"`    // stdout, stderr, file
	FilePath string `json:"file_path"` // 日志文件路径（当 output=file 时使用）
}

// Load 加载配置
func Load(configPath string) (*Config, error) {
	cfg, err := LoadUnvalidated(configPath)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadUnvalidated merges defaults, the explicit file and environment variables.
// CLI callers can apply their higher-priority flags before validating the final
// settings. Malformed files and shared environment values still return errors.
func LoadUnvalidated(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// 如果指定了配置文件，则加载
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if err := cfg.applyEnvironment(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// DefaultConfig 返回应用默认配置。
func DefaultConfig() *Config {
	return &Config{
		DefaultOutput:         "stdout",
		DefaultFormat:         "text",
		Verbose:               false,
		OllamaEndpoint:        "http://localhost:11434",
		MilvusEndpoint:        "localhost:19530",
		RequestTimeoutSeconds: 300,
		RAGConfig: RAGConfig{
			Collection:     "code_segments",
			EmbeddingModel: "bge-m3:latest",
			ChatModel:      "llama3:latest",
			VectorDim:      1024,
			TopK:           3,
			ChunkMaxLines:  120,
			ChunkMinLines:  5,
		},
		LogConfig: LogConfig{
			Level:    "info",
			Format:   "text",
			Output:   "stderr",
			FilePath: "",
		},
	}
}

// applyEnvironment applies only explicitly set variables after the file values.
func (cfg *Config) applyEnvironment() error {
	stringsToSet := map[string]*string{
		"OUTPUT":          &cfg.DefaultOutput,
		"FORMAT":          &cfg.DefaultFormat,
		"OLLAMA_ENDPOINT": &cfg.OllamaEndpoint,
		"MILVUS_ENDPOINT": &cfg.MilvusEndpoint,
		"COLLECTION":      &cfg.RAGConfig.Collection,
		"EMBEDDING_MODEL": &cfg.RAGConfig.EmbeddingModel,
		"CHAT_MODEL":      &cfg.RAGConfig.ChatModel,
		"LOG_LEVEL":       &cfg.LogConfig.Level,
		"LOG_FORMAT":      &cfg.LogConfig.Format,
		"LOG_OUTPUT":      &cfg.LogConfig.Output,
		"LOG_FILE":        &cfg.LogConfig.FilePath,
	}
	for key, target := range stringsToSet {
		if val, ok := os.LookupEnv("GO_AI_INSIGHT_" + key); ok {
			*target = strings.TrimSpace(val)
		}
	}
	intsToSet := map[string]*int{
		"TIMEOUT_SECONDS": &cfg.RequestTimeoutSeconds,
		"VECTOR_DIM":      &cfg.RAGConfig.VectorDim,
		"TOP_K":           &cfg.RAGConfig.TopK,
		"CHUNK_MAX_LINES": &cfg.RAGConfig.ChunkMaxLines,
		"CHUNK_MIN_LINES": &cfg.RAGConfig.ChunkMinLines,
	}
	for key, target := range intsToSet {
		name := "GO_AI_INSIGHT_" + key
		if val, ok := os.LookupEnv(name); ok {
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				cfg.ragEnvironmentError = fmt.Errorf("config: %s must be an integer", name)
				continue
			}
			*target = n
		}
	}
	if val, ok := os.LookupEnv("GO_AI_INSIGHT_VERBOSE"); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("config: GO_AI_INSIGHT_VERBOSE must be a boolean")
		}
		cfg.Verbose = parsed
	}
	return nil
}

// Validate checks shared CLI settings. AI-only settings are checked by ValidateRAG
// so source scanning and static analysis work without any AI services configured.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config: configuration is nil")
	}
	if !oneOf(cfg.DefaultFormat, "text", "json") {
		return fmt.Errorf("config: default_format must be text or json")
	}
	if !oneOf(cfg.LogConfig.Level, "debug", "info", "warn", "error") {
		return fmt.Errorf("config: log_config.level must be debug, info, warn or error")
	}
	if !oneOf(cfg.LogConfig.Format, "text", "json") {
		return fmt.Errorf("config: log_config.format must be text or json")
	}
	if !oneOf(cfg.LogConfig.Output, "stdout", "stderr", "file") {
		return fmt.Errorf("config: log_config.output must be stdout, stderr or file")
	}
	if cfg.LogConfig.Output == "file" && strings.TrimSpace(cfg.LogConfig.FilePath) == "" {
		return fmt.Errorf("config: log_config.file_path is required when output is file")
	}
	return nil
}

// ValidateRAG validates AI command settings without making network requests.
func (cfg *Config) ValidateRAG() error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.ragEnvironmentError != nil {
		return cfg.ragEnvironmentError
	}
	u, err := url.Parse(strings.TrimSpace(cfg.OllamaEndpoint))
	if err != nil || u.Host == "" || !oneOf(u.Scheme, "http", "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("config: ollama_endpoint must be an http(s) URL without credentials, path or query")
	}
	if host, port, err := net.SplitHostPort(NormalizeMilvusEndpoint(cfg.MilvusEndpoint)); err != nil || host == "" {
		return fmt.Errorf("config: milvus_endpoint must be host:port")
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("config: milvus_endpoint port must be between 1 and 65535")
	}
	if cfg.RequestTimeoutSeconds <= 0 || cfg.RequestTimeoutSeconds > 86400 {
		return fmt.Errorf("config: request_timeout_seconds must be between 1 and 86400")
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(cfg.RAGConfig.Collection) {
		return fmt.Errorf("config: rag_config.collection must contain only letters, digits and underscores and start with a letter or underscore")
	}
	if strings.TrimSpace(cfg.RAGConfig.EmbeddingModel) == "" || strings.TrimSpace(cfg.RAGConfig.ChatModel) == "" {
		return fmt.Errorf("config: embedding_model and chat_model are required")
	}
	if cfg.RAGConfig.VectorDim <= 0 {
		return fmt.Errorf("config: rag_config.vector_dim must be positive")
	}
	if cfg.RAGConfig.TopK <= 0 || cfg.RAGConfig.TopK > 16384 {
		return fmt.Errorf("config: rag_config.top_k must be between 1 and 16384")
	}
	if cfg.RAGConfig.ChunkMinLines <= 0 || cfg.RAGConfig.ChunkMaxLines < cfg.RAGConfig.ChunkMinLines {
		return fmt.Errorf("config: chunk lines must satisfy 0 < chunk_min_lines <= chunk_max_lines")
	}
	return nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

// NormalizeMilvusEndpoint 将配置中的 Milvus endpoint 标准化为 SDK 需要的 host:port 格式。
func NormalizeMilvusEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return strings.TrimRight(endpoint, "/")
}

// GetConfigPath 获取默认配置文件路径
func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".go-ai-insight", "config.json")
}

// Save 保存配置
func Save(configPath string, cfg *Config) error {
	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存配置
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
