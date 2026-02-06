package ai

import (
	"log/slog"
	"os"
)

// Logger 简单的日志包装器
type Logger struct {
	*slog.Logger
}

// NewLogger 创建新的日志记录器
func NewLogger(level slog.Level) *Logger {
	return &Logger{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})),
	}
}

// NewJSONLogger 创建 JSON 格式日志记录器
func NewJSONLogger(level slog.Level) *Logger {
	return &Logger{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})),
	}
}
