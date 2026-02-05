package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	DefaultOutput  string `json:"default_output"`
	DefaultFormat  string `json:"default_format"`
	Verbose        bool   `json:"verbose"`
	OllamaEndpoint string `json:"ollama_endpoint"`
	MilvusEndpoint string `json:"milvus_endpoint"`
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
