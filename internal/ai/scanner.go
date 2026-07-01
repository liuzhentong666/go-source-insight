package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/schema"
)

var ignoredScanDirs = map[string]struct{}{
	".git":         {},
	"vendor":       {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	".idea":        {},
	".vscode":      {},
}

func ScanCode(rootPath string) ([]schema.Document, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("stat scan path %q: %w", rootPath, err)
	}

	if !info.IsDir() {
		return scanCodeFile(rootPath, rootPath)
	}

	var docs []schema.Document
	err = filepath.WalkDir(rootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk scan path %q: %w", path, walkErr)
		}

		if d.IsDir() {
			if shouldSkipScanDir(d.Name()) && path != rootPath {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		doc, ok, err := buildCodeDocument(rootPath, path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return docs, nil
}

func scanCodeFile(rootPath, path string) ([]schema.Document, error) {
	if filepath.Ext(path) != ".go" {
		return nil, nil
	}

	doc, ok, err := buildCodeDocument(rootPath, path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	return []schema.Document{doc}, nil
}

func buildCodeDocument(rootPath, path string) (schema.Document, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return schema.Document{}, false, fmt.Errorf("read scan file %q: %w", path, err)
	}

	if strings.TrimSpace(string(content)) == "" {
		return schema.Document{}, false, nil
	}

	relativeSource, err := filepath.Rel(rootPath, path)
	if err != nil {
		return schema.Document{}, false, fmt.Errorf("build relative source for %q from root %q: %w", path, rootPath, err)
	}

	return schema.Document{
		PageContent: string(content),
		Metadata: map[string]any{
			"source":          filepath.ToSlash(path),
			"relative_source": filepath.ToSlash(relativeSource),
			"file_name":       filepath.Base(path),
		},
	}, true, nil
}

func shouldSkipScanDir(name string) bool {
	_, ok := ignoredScanDirs[name]
	return ok
}
