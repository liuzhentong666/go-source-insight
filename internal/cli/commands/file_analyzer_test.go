package commands

import (
	"context"
	"path/filepath"
	"testing"

	"go-ai-study/internal/tools"
)

func TestAnalyzeTargetAggregatesProductionGoFiles(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "main.go"), `package main

import (
	"fmt"
	"os"
)

func Risky(userID string) {
	password := "admin123"
	fmt.Println(password)
	_ = os.Remove(userID)
}
`)
	writeTestFile(t, filepath.Join(tmp, "main_test.go"), `package main
func TestRisky() {}
`)

	manager := newTestToolManager()
	result, err := analyzeTarget(context.Background(), manager, tmp)
	if err != nil {
		t.Fatalf("analyzeTarget failed: %v", err)
	}

	if result.FilesAnalyzed != 1 {
		t.Fatalf("FilesAnalyzed = %d, want 1", result.FilesAnalyzed)
	}
	if result.FunctionsAnalyzed == 0 {
		t.Fatal("FunctionsAnalyzed = 0, want > 0")
	}
	if result.SecurityIssues == 0 {
		t.Fatal("SecurityIssues = 0, want > 0")
	}
	if result.BugIssues == 0 {
		t.Fatal("BugIssues = 0, want > 0")
	}
}

func TestAnalyzeTargetOnlyTestFiles(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "handler_test.go"), `package main
func TestHandler(t *testing.T) {}
`)
	writeTestFile(t, filepath.Join(tmp, "service_test.go"), `package main
func TestService(t *testing.T) {}
`)

	manager := newTestToolManager()
	result, err := analyzeTarget(context.Background(), manager, tmp)
	if err != nil {
		t.Fatalf("analyzeTarget failed: %v", err)
	}

	if result.FilesAnalyzed != 0 {
		t.Fatalf("FilesAnalyzed = %d, want 0 (no production code)", result.FilesAnalyzed)
	}
	if len(result.Files) != 0 {
		t.Fatalf("len(Files) = %d, want 0", len(result.Files))
	}
}

func newTestToolManager() *tools.ToolManager {
	manager := tools.NewToolManager(nil)
	_ = manager.Register(tools.NewComplexityAnalyzer(), tools.DefaultToolConfig("complexity_analyzer"))
	_ = manager.Register(tools.NewSecurityScanner(), tools.DefaultToolConfig("security_scanner"))
	_ = manager.Register(tools.NewBugDetector(), tools.DefaultToolConfig("bug_detector"))
	return manager
}
