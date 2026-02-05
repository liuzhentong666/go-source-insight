package tools

import (
	"log/slog"
	"os"
)

// Logger 工具日志接口
type Logger interface {
	Info(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// DefaultLogger 默认日志实现
type DefaultLogger struct {
	logger *slog.Logger
}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger(level slog.Level) *DefaultLogger {
	return &DefaultLogger{
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})),
	}
}

// NewJSONLogger 创建 JSON 格式日志记录器
func NewJSONLogger(level slog.Level) *DefaultLogger {
	return &DefaultLogger{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})),
	}
}

// NewFileLogger 创建文件日志记录器
func NewFileLogger(level slog.Level, filePath string) (*DefaultLogger, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &DefaultLogger{
		logger: slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{
			Level: level,
		})),
	}, nil
}

func (dl *DefaultLogger) Info(msg string, args ...any) {
	dl.logger.Info(msg, args...)
}

func (dl *DefaultLogger) Debug(msg string, args ...any) {
	dl.logger.Debug(msg, args...)
}

func (dl *DefaultLogger) Warn(msg string, args ...any) {
	dl.logger.Warn(msg, args...)
}

func (dl *DefaultLogger) Error(msg string, args ...any) {
	dl.logger.Error(msg, args...)
}

// NoopLogger 空日志记录器（用于测试）
type NoopLogger struct{}

func (nl *NoopLogger) Info(msg string, args ...any) {}

func (nl *NoopLogger) Debug(msg string, args ...any) {}

func (nl *NoopLogger) Warn(msg string, args ...any) {}

func (nl *NoopLogger) Error(msg string, args ...any) {}

// NewNoopLogger 创建空日志记录器
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}
