package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	api "go-ai-study/api/analysis"
	core "go-ai-study/internal/analysis"
	"go-ai-study/internal/cli/commands"
	"go-ai-study/internal/cli/output"
)

// Run the real CLI command and API entry point over the same source. The fixture
// exercises core rules the previous API copy omitted, including physical lines
// under a //line directive and two independently owned resources.
func TestCLIAndAPIShareStaticRules(t *testing.T) {
	code := "package sample\n" +
		"import \"os\"\n" +
		"//line mapped.go:900\n" +
		"const password = \"example-placeholder\"\n" +
		"var accessToken = \"example-placeholder\"\n" +
		"func read() {\n" +
		" closed, err := os.Open(\"closed.txt\")\n" +
		" if err != nil { return }\n" +
		" defer closed.Close()\n" +
		" leaked, _ := os.Open(\"leaked.txt\")\n" +
		" _ = leaked\n" +
		"}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte("package sample\nfunc TestIgnored() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := commands.NewAnalyzeCommand(nil)
	if err := cmd.Run(context.Background(), []string{dir}, output.WithWriter(output.NewJSONFormatter(), &out)); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Success bool                      `json:"success"`
		Result  core.StaticAnalysisResult `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success || envelope.Result.FilesAnalyzed != 1 || len(envelope.Result.Files) != 1 {
		t.Fatalf("unexpected CLI report: %s", out.String())
	}
	got, err := api.PerformAnalysis(code)
	if err != nil {
		t.Fatal(err)
	}
	// Project the CLI core result through the retained API schema.
	raw, err := json.Marshal(envelope.Result.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	var want api.AnalysisResult
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, got.AnalyzedAt); err != nil {
		t.Fatal(err)
	}
	got.AnalyzedAt = ""
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("API and CLI differ:\nAPI=%+v\nCLI=%+v", *got, want)
	}
	if len(got.Complexity.Functions) != 1 || got.Complexity.Functions[0].Line != 6 || got.Complexity.Functions[0].Lines != 7 {
		t.Fatalf("wrong physical function lines: %+v", got.Complexity.Functions)
	}
	secretLines := map[int]bool{}
	for _, issue := range got.Security.Issues {
		if issue.RuleID == "G101" {
			secretLines[issue.Line] = true
		}
	}
	if !reflect.DeepEqual(secretLines, map[int]bool{4: true, 5: true}) {
		t.Fatalf("const/var detection or physical lines regressed: %+v", got.Security.Issues)
	}
	leaks := 0
	for _, bug := range got.Bugs.Bugs {
		if bug.RuleID != "B102" {
			continue
		}
		leaks++
		if bug.Line != 10 || !strings.Contains(bug.CodeSnippet, "leaked") {
			t.Fatalf("wrong resource or physical location: %+v", bug)
		}
	}
	if leaks != 1 {
		t.Fatalf("B102 = %d, want 1; bugs=%+v", leaks, got.Bugs.Bugs)
	}
	if envelope.Result.SecurityIssues != got.Security.Total || envelope.Result.BugIssues != got.Bugs.Total {
		t.Fatal("CLI aggregate counts differ from API")
	}
	complexity, err := api.AnalyzeComplexity(code)
	if err != nil || !reflect.DeepEqual(complexity, got.Complexity) {
		t.Fatalf("complexity adapter mismatch: %v", err)
	}
	security, err := api.AnalyzeSecurity(code)
	if err != nil || !reflect.DeepEqual(security, got.Security) {
		t.Fatalf("security adapter mismatch: %v", err)
	}
	bugs, err := api.AnalyzeBugs(code)
	if err != nil || !reflect.DeepEqual(bugs, got.Bugs) {
		t.Fatalf("bug adapter mismatch: %v", err)
	}
}

func TestAnalysisRejectsInvalidCodeAndCanceledContext(t *testing.T) {
	for name, run := range map[string]func(string) error{
		"all":        func(code string) error { _, err := api.PerformAnalysis(code); return err },
		"complexity": func(code string) error { _, err := api.AnalyzeComplexity(code); return err },
		"security":   func(code string) error { _, err := api.AnalyzeSecurity(code); return err },
		"bugs":       func(code string) error { _, err := api.AnalyzeBugs(code); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run("package sample\nfunc {"); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := api.PerformAnalysisContext(ctx, "package sample"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled analysis error = %v", err)
	}
}

func TestLegacyReportJSONKeepsItsFields(t *testing.T) {
	result, err := api.PerformAnalysis("package sample\nconst password = \"example-placeholder\"\nfunc read(){ f, _ := os.Open(\"sample.txt\"); _ = f }")
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.ToJSON()), &report); err != nil {
		t.Fatal(err)
	}
	if len(report) != 4 {
		t.Fatalf("unexpected top-level fields: %v", report)
	}
	for _, key := range []string{"complexity", "security", "bugs", "analyzed_at"} {
		if _, ok := report[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	var bugs map[string]json.RawMessage
	if err := json.Unmarshal(report["bugs"], &bugs); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"skipped_files", "error_files", "recommendations"} {
		if _, ok := bugs[key]; ok {
			t.Fatalf("legacy JSON gained %s", key)
		}
	}
	var reloaded api.AnalysisResult
	if err := json.Unmarshal([]byte(result.ToJSON()), &reloaded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*result, reloaded) {
		t.Fatal("legacy report round-trip failed")
	}
}
