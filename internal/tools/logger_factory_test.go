package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go-ai-study/internal/config"
)

func TestNewLoggerFactory(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.LogConfig
		wantNil bool
	}{
		{
			name: "text format stdout",
			config: &config.LogConfig{
				Level:  "info",
				Format: "text",
				Output: "stdout",
			},
			wantNil: false,
		},
		{
			name: "json format stdout",
			config: &config.LogConfig{
				Level:  "debug",
				Format: "json",
				Output: "stdout",
			},
			wantNil: false,
		},
		{
			name: "text format stderr",
			config: &config.LogConfig{
				Level:  "warn",
				Format: "text",
				Output: "stderr",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLoggerFactory(tt.config)
			if (logger == nil) != tt.wantNil {
				t.Errorf("NewLoggerFactory() = %v, wantNil %v", logger, tt.wantNil)
			}
		})
	}
}

func TestLoggerFactory_CreateLogger(t *testing.T) {
	factory := &loggerFactory{}

	tests := []struct {
		name    string
		config  *config.LogConfig
		wantErr bool
	}{
		{
			name: "stdout text",
			config: &config.LogConfig{
				Level:  "info",
				Format: "text",
				Output: "stdout",
			},
			wantErr: false,
		},
		{
			name: "stderr json",
			config: &config.LogConfig{
				Level:  "error",
				Format: "json",
				Output: "stderr",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := factory.CreateLogger(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && logger == nil {
				t.Error("CreateLogger() returned nil logger")
			}
		})
	}
}

func TestLoggerFactory_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := &config.LogConfig{
		Level:    "info",
		Format:   "text",
		Output:   "file",
		FilePath: logFile,
	}

	factory := &loggerFactory{}
	logger, err := factory.CreateLogger(config)
	if err != nil {
		t.Fatalf("CreateLogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("CreateLogger() returned nil logger")
	}

	logger.Info("test message", "key", "value")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file does not exist")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"warning", "warning", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"invalid", "invalid", slog.LevelInfo},
		{"empty", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLogLevel(tt.level); got != tt.want {
				t.Errorf("parseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		levelStr string
		want    slog.Level
		wantErr bool
	}{
		{"debug string", "debug", slog.LevelDebug, false},
		{"info string", "info", slog.LevelInfo, false},
		{"warn string", "warn", slog.LevelWarn, false},
		{"error string", "error", slog.LevelError, false},
		{"numeric", "-4", slog.Level(-4), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LogLevel(tt.levelStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger(slog.LevelInfo)
	logger.Info("test message")
	logger.Debug("debug message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestJSONLogger(t *testing.T) {
	logger := NewJSONLogger(slog.LevelDebug)
	logger.Info("test message")
	logger.Debug("debug message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestNoopLogger(t *testing.T) {
	logger := NewNoopLogger()
	logger.Info("test message")
	logger.Debug("debug message")
	logger.Warn("warn message")
	logger.Error("error message")
}
