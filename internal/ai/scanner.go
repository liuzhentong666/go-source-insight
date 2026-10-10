package ai

import (
	"context"
	"fmt"
	"github.com/tmc/langchaingo/schema"
	"go-ai-study/internal/source"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ScanCode(rootPath string) ([]schema.Document, error) {
	return scanCode(context.Background(), rootPath)
}
func scanCode(ctx context.Context, rootPath string) ([]schema.Document, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("访问源码路径失败: %w", err)
	}
	if !info.IsDir() && !source.IsGoFile(rootPath) {
		return []schema.Document{}, nil
	}
	scan, err := source.Scan(ctx, rootPath)
	if err != nil {
		return nil, err
	}
	base := rootPath
	if !info.IsDir() {
		base = filepath.Dir(rootPath)
	}
	return documentsFromFiles(ctx, scan.Files, os.ReadFile, func(name string) (string, error) {
		rel, err := filepath.Rel(base, name)
		return filepath.ToSlash(rel), err
	})
}

// FSScanner reads only through the caller's confined filesystem. Sources stay
// relative to that filesystem's root even when a nested target is indexed.
type FSScanner struct{ FS fs.FS }

func (s *FSScanner) Scan(ctx context.Context, target string) ([]schema.Document, error) {
	if s == nil || s.FS == nil {
		return nil, fmt.Errorf("source filesystem is required")
	}
	scanned, err := source.ScanFS(ctx, s.FS, target)
	if err != nil {
		return nil, err
	}
	return documentsFromFiles(ctx, scanned.Files, func(name string) ([]byte, error) { return fs.ReadFile(s.FS, name) }, func(name string) (string, error) { return name, nil })
}

func documentsFromFiles(ctx context.Context, files []string, read func(string) ([]byte, error), relative func(string) (string, error)) ([]schema.Document, error) {
	docs := make([]schema.Document, 0, len(files))
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		content, err := read(path)
		if err != nil {
			return nil, fmt.Errorf("读取源码失败: %w", &source.AccessError{Err: err})
		}
		if strings.TrimSpace(string(content)) == "" {
			continue
		}
		rel, err := relative(path)
		if err != nil {
			return nil, fmt.Errorf("计算源码相对路径失败: %w", err)
		}
		rel = filepath.ToSlash(rel)
		docs = append(docs, schema.Document{PageContent: string(content), Metadata: map[string]any{
			"source": rel, "relative_source": rel, "file_name": filepath.Base(path),
		}})
	}
	return docs, nil
}
func shouldSkipScanDir(name string) bool { return source.ShouldSkipDir(name) }
