package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go-ai-study/internal/tools"
)

func TestStaticAnalyzerHonorsConfiguredTools(t *testing.T) {
	manager := tools.NewToolManager(nil)
	config := tools.DefaultToolConfig("complexity_analyzer")
	config.Enabled = false
	if err := manager.Register(tools.NewComplexityAnalyzer(), config); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStaticAnalyzer(manager).AnalyzeCode(context.Background(), "package sample", "sample.go"); !errors.Is(err, tools.ErrToolDisabled) {
		t.Fatalf("configured tool policy was ignored: %v", err)
	}
}

func TestStaticAnalyzerTargetReturnsNoPartialReport(t *testing.T) {
	dir := t.TempDir()
	for name, code := range map[string]string{"valid.go": "package sample\nfunc Valid() {}", "invalid.go": "package sample\nfunc {"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(code), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := NewStaticAnalyzer(nil).AnalyzeTarget(context.Background(), dir)
	if err == nil || result.FilesAnalyzed != 0 || len(result.Files) != 0 {
		t.Fatalf("partial report returned as complete: result=%+v, error=%v", result, err)
	}
}
