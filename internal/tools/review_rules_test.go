package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestResourceCloseTracksTheOpenedBinding(t *testing.T) {
	cases := []struct {
		name, body string
		leaks      int
	}{
		{"missing close", `f, _ := os.Open("example.txt"); _ = f`, 1},
		{"direct close", `f, _ := os.Open("example.txt"); f.Close()`, 0},
		{"checked close", `f, _ := os.Open("example.txt"); err := f.Close(); _ = err`, 0},
		{"returned close", `f, _ := os.Open("example.txt"); return f.Close()`, 0},
		{"defer after error guard", `f, err := os.Open("example.txt"); if err != nil { return }; defer f.Close()`, 0},
		{"deferred closure", `f, _ := os.Open("example.txt"); defer func() { _ = f.Close() }()`, 0},
		{"var declaration", `var f, err = os.Create("example.txt"); if err != nil { return }; defer f.Close()`, 0},
		{"openfile", `f, _ := os.OpenFile("example.txt", 0, 0600); defer f.Close()`, 0},
		{"writefile owns handle", `_ = os.WriteFile("example.txt", nil, 0600)`, 0},
		{"different resource", `f, _ := os.Open("a.txt"); g, _ := os.Open("b.txt"); defer g.Close(); _ = f`, 1},
		{"two missing closes", `f, _ := os.Open("a.txt"); g, _ := os.Open("b.txt"); _, _ = f, g`, 2},
		{"shadowed resource", `f, _ := os.Open("a.txt"); { f, _ := os.Open("b.txt"); defer f.Close() }; _ = f`, 1},
		{"overwritten resource", `f, _ := os.Open("a.txt"); f, _ = os.Open("b.txt"); defer f.Close()`, 1},
		{"defer captures receiver", `f, _ := os.Open("a.txt"); defer f.Close(); f, _ = os.Open("b.txt"); defer f.Close()`, 0},
		{"closure captures variable", `f, _ := os.Open("a.txt"); defer func() { f.Close() }(); f, _ = os.Open("b.txt"); f.Close()`, 1},
		{"uninvoked closure", `f, _ := os.Open("example.txt"); cleanup := func() { f.Close() }; _ = cleanup`, 1},
		{"conditional close", `f, _ := os.Open("example.txt"); if enabled { f.Close() }`, 1},
		{"both branches close", `f, _ := os.Open("example.txt"); if enabled { f.Close() } else { defer f.Close() }`, 0},
		{"early return leaks", `f, _ := os.Open("example.txt"); if enabled { return }; defer f.Close()`, 1},
		{"changed error cannot guard", `f, err := os.Open("example.txt"); err = other(); if err != nil { return }; defer f.Close()`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Keep separate opens on separate lines so issue deduplication cannot
			// hide a false positive on a correctly closed second resource.
			body := strings.ReplaceAll(tc.body, "; ", ";\n")
			bugs, err := NewBugDetector().analyzeCode("package sample\nimport \"os\"\nfunc check() {\n"+body+"\n}", "sample.go")
			if err != nil {
				t.Fatal(err)
			}
			got := 0
			for _, bug := range bugs {
				if bug.RuleID == "B102" {
					got++
				}
			}
			if got != tc.leaks {
				t.Fatalf("B102 count = %d, want %d; issues=%+v", got, tc.leaks, bugs)
			}
		})
	}
}

func TestHardcodedSecretsIncludesDeclarationsAndCorrespondingValues(t *testing.T) {
	cases := []struct {
		name, code string
		issues     int
	}{
		{"package const", `const password = "example-placeholder"`, 1},
		{"package var", "var accessToken string = `example-placeholder`", 1},
		{"local const", `func check() { const apiKey = "example-placeholder" }`, 1},
		{"local var", `func check() { var credential = "example-placeholder" }`, 1},
		{"grouped", "const (\n password = \"example-one\"\n apiKey = \"example-two\"\n)", 2},
		{"implicit const", "const (\n label = \"example-placeholder\"\n password\n)", 1},
		{"multiple values", `var label, password = getLabel(), "example-placeholder"`, 1},
		{"assignment corresponding value", `func check() { label, password := getLabel(), "example-placeholder"; _, _ = label, password }`, 1},
		{"environment value", `var password = os.Getenv("EXAMPLE_PASSWORD")`, 0},
		{"wrong value must not match", `var label, password = "example-label", os.Getenv("EXAMPLE_PASSWORD")`, 0},
		{"assignment wrong value", `func check() { label, password := "example-label", os.Getenv("EXAMPLE_PASSWORD"); _, _ = label, password }`, 0},
		{"not string", `var tokenCount = 3`, 0},
		{"no initializer", `var password string`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := NewSecurityScanner().Run(context.Background(), "package sample\n"+tc.code)
			if err != nil {
				t.Fatal(err)
			}
			var result SecurityResult
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			got := 0
			for _, issue := range result.Issues {
				if issue.RuleID == "G101" {
					got++
				}
			}
			if got != tc.issues {
				t.Fatalf("G101 count = %d, want %d; issues=%+v", got, tc.issues, result.Issues)
			}
		})
	}
}

func TestStaticIssuesUsePhysicalLines(t *testing.T) {
	code := "package sample\n//line mapped.go:900\nconst password = \"example-placeholder\"\nfunc read() {\n f, _ := os.Open(\"example.txt\"); _ = f\n}\n"
	bugs, err := NewBugDetector().analyzeCode(code, "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, bug := range bugs {
		if bug.RuleID == "B102" {
			found = true
			if bug.Line != 5 || !strings.Contains(bug.CodeSnippet, "os.Open") {
				t.Fatalf("wrong location: %+v", bug)
			}
		}
	}
	if !found {
		t.Fatal("missing B102")
	}
	raw, err := NewSecurityScanner().Run(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	var result SecurityResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Line != 3 || !strings.Contains(result.Issues[0].CodeSnippet, "password") {
		t.Fatalf("wrong locations: %+v", result.Issues)
	}
	raw, err = NewComplexityAnalyzer().Run(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	var complexity ComplexityResult
	if err := json.Unmarshal([]byte(raw), &complexity); err != nil {
		t.Fatal(err)
	}
	if len(complexity.Functions) != 1 || complexity.Functions[0].Line != 4 || complexity.Functions[0].Lines != 3 {
		t.Fatalf("wrong physical function range: %+v", complexity.Functions)
	}
}
