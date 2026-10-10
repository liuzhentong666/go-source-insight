package commands

import (
	"context"
	"go-ai-study/internal/source"
)

type GoFileScanResult = source.ScanResult

func collectGoFiles(target string) (GoFileScanResult, error) {
	return source.Scan(context.Background(), target)
}
func shouldSkipDir(name string) bool { return source.ShouldSkipDir(name) }
func isGoFile(path string) bool      { return source.IsGoFile(path) }
func isGoTestFile(path string) bool  { return source.IsTestFile(path) }
