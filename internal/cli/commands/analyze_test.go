package commands

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"go-ai-study/internal/cli/output"
)

func TestAnalyzeCommandAnalyzesDirectory(t *testing.T) {
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

	cmd := NewAnalyzeCommand(newTestToolManager())
	stdout := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), []string{tmp}, output.NewJSONFormatter()); err != nil {
			t.Fatalf("analyze command failed: %v", err)
		}
	})

	var envelope struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("unmarshal formatter envelope: %v\noutput=%s", err, stdout)
	}

	var result StaticAnalysisResult
	if err := json.Unmarshal([]byte(envelope.Result), &result); err != nil {
		t.Fatalf("unmarshal analysis result: %v\nresult=%s", err, envelope.Result)
	}

	if result.FilesAnalyzed != 1 {
		t.Fatalf("FilesAnalyzed = %d, want 1", result.FilesAnalyzed)
	}
	if result.SecurityIssues == 0 {
		t.Fatal("SecurityIssues = 0, want > 0")
	}
	if result.BugIssues == 0 {
		t.Fatal("BugIssues = 0, want > 0")
	}
}
