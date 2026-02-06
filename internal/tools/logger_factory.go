package tools

import (
	"log/slog"
	"os"
	"strconv"

	"go-ai-study/internal/config"
)

// LoggerFactory 日志工厂接口
type LoggerFactory interface {
	CreateLogger(cfg *config.LogConfig) (Logger, error)
}

// loggerFactory 日志工厂实现
type loggerFactory struct{}

// NewLoggerFactory 创建日志工厂
func NewLoggerFactory(cfg *config.LogConfig) Logger {
	factory := &loggerFactory{}
	logger, err := factory.CreateLogger(cfg)
	if err != nil {
		// 如果创建失败，返回默认的 stderr logger
		return NewDefaultLogger(slog.LevelError)
	}
	return logger
}

// CreateLogger 根据配置创建日志记录器
func (lf *loggerFactory) CreateLogger(cfg *config.LogConfig) (Logger, error) {
	// 1. 解析日志级别
	level := parseLogLevel(cfg.Level)

	// 2. 创建 handler
	var handler slog.Handler
	var err error

	switch cfg.Output {
	case "stdout":
		handler = createHandler(os.Stdout, cfg.Format, level)
	case "stderr":
		handler = createHandler(os.Stderr, cfg.Format, level)
	case "file":
		handler, err = createFileHandler(cfg.FilePath, cfg.Format, level)
		if err != nil {
			return nil, err
		}
	default:
		// 默认输出到 stdout
		handler = createHandler(os.Stdout, cfg.Format, level)
	}

	// 3. 创建 logger
	return &DefaultLogger{
		logger: slog.New(handler),
	}, nil
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo // 默认 info 级别
	}
}

// createHandler 创建输出到指定 writer 的 handler
func createHandler(writer *os.File, format string, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch format {
	case "json":
		return slog.NewJSONHandler(writer, opts)
	case "text":
		return slog.NewTextHandler(writer, opts)
	default:
		// 默认使用 text 格式
		return slog.NewTextHandler(writer, opts)
	}
}

// createFileHandler 创建文件输出 handler
func createFileHandler(filePath, format string, level slog.Level) (slog.Handler, error) {
	// 打开文件（追加模式，不存在则创建）
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return createHandler(file, format, level), nil
}

// LogLevel 从字符串解析日志级别（用于命令行参数）
func LogLevel(levelStr string) (slog.Level, error) {
	switch levelStr {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		// 尝试解析为数字
		if level, err := strconv.Atoi(levelStr); err == nil {
			return slog.Level(level), nil
		}
		return slog.LevelInfo, nil
	}
}
