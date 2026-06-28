package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewTestGenerator(t *testing.T) {
	logger := NewNoopLogger()
	generator := NewTestGenerator(logger)

	if generator == nil {
		t.Error("NewTestGenerator() returned nil")
	}

	if generator.Name() != "test_generator" {
		t.Errorf("Expected name 'test_generator', got '%s'", generator.Name())
	}
}

func TestValidate(t *testing.T) {
	logger := NewNoopLogger()
	generator := NewTestGenerator(logger)

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "valid request with function name",
			input:   GenerateRequest{FunctionName: "TestFunc"},
			wantErr: false,
		},
		{
			name:    "valid request with file path",
			input:   GenerateRequest{FilePath: "test_generator.go"},
			wantErr: false,
		},
		{
			name:    "invalid request - no target",
			input:   GenerateRequest{},
			wantErr: true,
		},
		{
			name:    "invalid input type",
			input:   "string",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generator.Validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunDoesNotOverwriteExistingTestFile(t *testing.T) {
	tmp := t.TempDir()
	sourcePath := filepath.Join(tmp, "sample.go")
	existingTestPath := filepath.Join(tmp, "sample_test.go")
	existingContent := "package sample\n\nfunc TestExisting(t *testing.T) {}\n"

	if err := os.WriteFile(sourcePath, []byte(`package sample

func Add(a int, b int) int {
	return a + b
}
`), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(existingTestPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("write existing test file: %v", err)
	}

	generator := NewTestGenerator(NewNoopLogger())
	_, err := generator.Run(context.Background(), GenerateRequest{
		FilePath: sourcePath,
		TestMode: TestModeTableDriven,
	})
	if err == nil {
		t.Fatal("expected error when test file already exists, got nil")
	}
	if !strings.Contains(err.Error(), "测试文件已存在") {
		t.Fatalf("error = %v, want message containing 测试文件已存在", err)
	}

	content, readErr := os.ReadFile(existingTestPath)
	if readErr != nil {
		t.Fatalf("read existing test file: %v", readErr)
	}
	if string(content) != existingContent {
		t.Fatalf("existing test file was overwritten; got %q, want %q", string(content), existingContent)
	}
}

func TestRunGeneratesCompilableTestFile(t *testing.T) {
	tmp := t.TempDir()
	sourcePath := filepath.Join(tmp, "sample.go")

	if err := os.WriteFile(sourcePath, []byte(`package sample

func Add(a int, b int) int {
	return a + b
}
`), 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	generator := NewTestGenerator(NewNoopLogger())
	_, err := generator.Run(context.Background(), GenerateRequest{
		FilePath: sourcePath,
		TestMode: TestModeTableDriven,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module sample\n\ngo 1.25.5\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmp
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated test file does not compile: %v\n%s", err, string(output))
	}
}
