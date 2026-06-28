package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-ai-study/internal/cli/output"
)

func TestScanCommandScansProjectStructure(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(tmp, "main_test.go"), "package main\nfunc TestMain() {}\n")
	writeTestFile(t, filepath.Join(tmp, "vendor", "dep.go"), "package dep\n")

	cmd := NewScanCommand()
	stdout := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), []string{tmp}, output.NewJSONFormatter()); err != nil {
			t.Fatalf("scan command failed: %v", err)
		}
	})

	var envelope struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("unmarshal formatter envelope: %v\noutput=%s", err, stdout)
	}
	if !envelope.Success {
		t.Fatalf("Success = false, want true")
	}

	var result GoFileScanResult
	if err := json.Unmarshal([]byte(envelope.Result), &result); err != nil {
		t.Fatalf("unmarshal scan result: %v\nresult=%s", err, envelope.Result)
	}

	if result.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2; result=%+v", result.TotalFiles, result)
	}
	if result.ProductionFileCount != 1 {
		t.Fatalf("ProductionFileCount = %d, want 1", result.ProductionFileCount)
	}
	if result.TestFileCount != 1 {
		t.Fatalf("TestFileCount = %d, want 1", result.TestFileCount)
	}
	if len(result.SkippedDirs) != 1 || !strings.Contains(filepath.ToSlash(result.SkippedDirs[0]), "/vendor") {
		t.Fatalf("SkippedDirs = %v, want vendor skipped", result.SkippedDirs)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
