package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-ai-study/internal/ai"
	"go-ai-study/internal/cli/commands"
)

func TestExecuteScanStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--format", "json", "scan", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d: %s", code, stderr.String())
	}
	var envelope struct {
		Success bool
		Result  struct {
			TotalFiles int `json:"total_files"`
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not pure JSON: %v", err)
	}
	if !envelope.Success || envelope.Result.TotalFiles != 1 {
		t.Fatalf("wrong report: %s", stdout.String())
	}
}

func TestExecuteRejectsUnusableLogFilesWithoutJSONOutput(t *testing.T) {
	for _, filePath := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing", "tool.log")} {
		var stdout, stderr bytes.Buffer
		code := Execute(context.Background(), []string{"-f", "json", "--log-output", "file", "--log-file", filePath, "list"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("unusable log file returned exit=%d; want 1", code)
		}
		if stdout.Len() != 0 {
			t.Fatalf("initialization failure polluted JSON stdout: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "日志初始化失败") {
			t.Fatalf("initialization error was not reported on stderr: %s", stderr.String())
		}
	}
}

func TestExecuteOutputFileAndFailureDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	report := filepath.Join(dir, "report.json")
	if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"-f", "json", "-o", report, "analyze", source}
	if code := Execute(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit=%d: %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatal("report also leaked to stdout")
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("invalid file output: %s", data)
	}
	args[len(args)-1] = filepath.Join(dir, "missing.go")
	if code := Execute(context.Background(), args, &stdout, &stderr); code != 1 {
		t.Fatalf("exit=%d; want 1", code)
	}
	preserved, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, preserved) {
		t.Fatal("failed analysis overwrote existing report")
	}
}

type fakeRAG struct {
	path, question, filter string
	closed                 bool
	failure                error
}

func (f *fakeRAG) Index(_ context.Context, path string) (*ai.IndexSummary, error) {
	f.path = path
	return &ai.IndexSummary{FilesScanned: 1, ChunksCreated: 1, ChunksIndexed: 1}, f.failure
}
func (f *fakeRAG) Ask(_ context.Context, question, filter string) (*ai.AnswerResult, error) {
	f.question = question
	f.filter = filter
	return &ai.AnswerResult{Question: question, Answer: "示例回答", Sources: []ai.SearchResult{{Source: "main.go", StartLine: 1, EndLine: 2}}}, f.failure
}
func (f *fakeRAG) Close() error { f.closed = true; return nil }

func TestCLIRAGCommandsAndLifecycle(t *testing.T) {
	app, err := NewCLI("", "json", "", false, "error", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	app.writer = &out
	current := &fakeRAG{}
	collection := ""
	app.ragFactory = func(_ context.Context, name string) (commands.RAGClient, error) {
		collection = name
		return current, nil
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"index", dir, "--collection", "sample_code"}); err != nil {
		t.Fatal(err)
	}
	if current.path != dir || collection != "sample_code" || !current.closed {
		t.Fatalf("index did not use shared factory: %+v", current)
	}
	out.Reset()
	current = &fakeRAG{}
	if err := app.Run(context.Background(), []string{"ask", "解释入口", "--file", "main.go", "--collection=sample_code"}); err != nil {
		t.Fatal(err)
	}
	if current.question != "解释入口" || current.filter != "main.go" || !current.closed {
		t.Fatalf("ask arguments/lifecycle wrong: %+v", current)
	}
	if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), "\"sources\"") {
		t.Fatalf("invalid answer report: %s", out.String())
	}
	out.Reset()
	current = &fakeRAG{failure: errors.New("model unavailable")}
	if err := app.Run(context.Background(), []string{"ask", "question"}); err == nil {
		t.Fatal("model failure was hidden")
	}
	if out.Len() != 0 || !current.closed {
		t.Fatal("failed question produced output or leaked connection")
	}
}

func TestCLIRejectsInvalidRAGArgumentsBeforeOpeningServices(t *testing.T) {
	app, err := NewCLI("", "json", "", false, "error", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	app.ragFactory = func(context.Context, string) (commands.RAGClient, error) {
		t.Fatal("invalid arguments opened AI services")
		return nil, nil
	}
	for _, args := range [][]string{{"index"}, {"ask"}, {"ask", "--file", "../main.go", "question"}, {"ask", "--file", "/main.go", "question"}, {"index", "--unknown", "."}, {"index", "README.md"}} {
		if err := app.Run(context.Background(), args); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestStaticCLIWorksWithInvalidAIConfig(t *testing.T) {
	t.Setenv("GO_AI_INSIGHT_OLLAMA_ENDPOINT", "invalid")
	t.Setenv("GO_AI_INSIGHT_VECTOR_DIM", "invalid")
	app, err := NewCLI("", "json", "", false, "error", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	app.writer = &out
	if err := app.Run(context.Background(), []string{"scan", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("invalid report: %s", out.String())
	}
	if err := app.Run(context.Background(), []string{"ask", "question"}); err == nil {
		t.Fatal("invalid AI settings were accepted")
	}
}

func TestCommandLineOverridesLowerPriorityLogConfiguration(t *testing.T) {
	t.Setenv("GO_AI_INSIGHT_LOG_OUTPUT", "file")
	t.Setenv("GO_AI_INSIGHT_LOG_FILE", "")
	app, err := NewCLI("", "json", "", false, "error", "", "stderr", "")
	if err != nil {
		t.Fatalf("explicit log destination could not override environment: %v", err)
	}
	var out bytes.Buffer
	app.writer = &out
	if err := app.Run(context.Background(), []string{"scan", t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteDefaultOutputEnvironment(t *testing.T) {
	report := filepath.Join(t.TempDir(), "scan.json")
	t.Setenv("GO_AI_INSIGHT_OUTPUT", report)
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), []string{"-f", "json", "scan", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d: %s", code, stderr.String())
	}
	data, err := os.ReadFile(report)
	if err != nil || !json.Valid(data) || stdout.Len() != 0 {
		t.Fatalf("default output did not receive report: %v", err)
	}
}

func TestDoctorChecksAndClosesWithoutIndexing(t *testing.T) {
	app, err := NewCLI("", "json", "", false, "error", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakeRAG{}
	app.ragFactory = func(context.Context, string) (commands.RAGClient, error) { return svc, nil }
	var out bytes.Buffer
	app.writer = &out
	if err := app.Run(context.Background(), []string{"doctor"}); err != nil {
		t.Fatal(err)
	}
	if !svc.closed || svc.path != "" || svc.question != "" {
		t.Fatal("doctor indexed/asked or leaked connection")
	}
	if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), "milvus_connection") {
		t.Fatalf("invalid health report: %s", out.String())
	}
}
