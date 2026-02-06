package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	DefaultOutput  string   `json:"default_output"`
	DefaultFormat  string   `json:"default_format"`
	Verbose        bool     `json:"verbose"`
	OllamaEndpoint string   `json:"ollama_endpoint"`
	MilvusEndpoint string   `json:"milvus_endpoint"`
	LogConfig      LogConfig `json:"log_config"`
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
	// 默认配置
	cfg := &Config{
		DefaultOutput:  "stdout",
		DefaultFormat:  "text",
		Verbose:        false,
		OllamaEndpoint: "http://localhost:11434",
		MilvusEndpoint: "http://localhost:19530",
		LogConfig: LogConfig{
			Level:    "info",
			Format:   "text",
			Output:   "stdout",
			FilePath: "",
		},
	}

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

	// 从环境变量加载
	if val := os.Getenv("GO_AI_INSIGHT_VERBOSE"); val != "" {
		cfg.Verbose = val == "true"
	}

	if val := os.Getenv("GO_AI_INSIGHT_FORMAT"); val != "" {
		cfg.DefaultFormat = val
	}

	// 从环境变量加载日志配置
	if val := os.Getenv("GO_AI_INSIGHT_LOG_LEVEL"); val != "" {
		cfg.LogConfig.Level = val
	}

	if val := os.Getenv("GO_AI_INSIGHT_LOG_FORMAT"); val != "" {
		cfg.LogConfig.Format = val
	}

	if val := os.Getenv("GO_AI_INSIGHT_LOG_OUTPUT"); val != "" {
		cfg.LogConfig.Output = val
	}

	if val := os.Getenv("GO_AI_INSIGHT_LOG_FILE"); val != "" {
		cfg.LogConfig.FilePath = val
	}

	return cfg, nil
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
