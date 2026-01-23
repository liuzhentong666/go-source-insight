package ai

import (
	"github.com/tmc/langchaingo/schema"
	"os"
	"path/filepath"
)

func ScanCode(rootPath string) ([]schema.Document, error) {
	var docs []schema.Document
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			content, _ := os.ReadFile(path)
			docs = append(docs, schema.Document{
				PageContent: string(content),
				Metadata:    map[string]any{"source": filepath.ToSlash(path)},
			})
		}
		return nil
	})
	return docs, err
}
