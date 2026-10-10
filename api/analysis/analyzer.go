// Package analysis preserves the existing API report schema while using the
// shared core rules. Stored reports and route contracts remain readable.
package analysis

import (
	"context"
	"encoding/json"
	"time"

	core "go-ai-study/internal/analysis"
)

type ComplexityResult struct {
	File       string           `json:"file"`
	Total      int              `json:"total"`
	Functions  []FunctionResult `json:"functions"`
	Summary    string           `json:"summary"`
	Statistics Statistics       `json:"statistics"`
}

type FunctionResult struct {
	Name       string   `json:"name"`
	Line       int      `json:"line"`
	Complexity int      `json:"complexity"`
	Lines      int      `json:"lines"`
	Issues     []string `json:"issues"`
}

type Statistics struct {
	TotalFunctions       int `json:"total_functions"`
	SimpleFunctions      int `json:"simple_functions"`
	MediumFunctions      int `json:"medium_functions"`
	ComplexFunctions     int `json:"complex_functions"`
	VeryComplexFunctions int `json:"very_complex_functions"`
}

type SecurityResult struct {
	File       string          `json:"file"`
	Total      int             `json:"total"`
	Issues     []SecurityIssue `json:"issues"`
	Summary    string          `json:"summary"`
	Statistics SecurityStats   `json:"statistics"`
}

type SecurityIssue struct {
	ID          string `json:"id"`
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Line        int    `json:"line"`
	CodeSnippet string `json:"code_snippet"`
	Suggestion  string `json:"suggestion"`
}

type SecurityStats struct {
	TotalIssues int `json:"total_issues"`
	Critical    int `json:"critical"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
}

type BugResult struct {
	Language      string     `json:"language"`
	Status        string     `json:"status"`
	TotalFiles    int        `json:"total_files"`
	AnalyzedFiles int        `json:"analyzed_files"`
	Total         int        `json:"total"`
	Bugs          []BugIssue `json:"bugs"`
	Summary       string     `json:"summary"`
	Statistics    BugStats   `json:"statistics"`
}

type BugIssue struct {
	ID            string `json:"id"`
	RuleID        string `json:"rule_id"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Line          int    `json:"line"`
	CodeSnippet   string `json:"code_snippet"`
	FixSuggestion string `json:"fix_suggestion"`
}

type BugStats struct {
	TotalIssues int `json:"total_issues"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
}

type AnalysisResult struct {
	Complexity *ComplexityResult `json:"complexity"`
	Security   *SecurityResult   `json:"security"`
	Bugs       *BugResult        `json:"bugs"`
	AnalyzedAt string            `json:"analyzed_at"`
}

// projectReport maps only the existing API fields; rule logic stays in the core.
func projectReport[T any](result any, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var compatible T
	if err := json.Unmarshal(data, &compatible); err != nil {
		return nil, err
	}
	return &compatible, nil
}

func AnalyzeComplexity(code string) (*ComplexityResult, error) {
	result, err := core.NewStaticAnalyzer(nil).AnalyzeComplexity(context.Background(), code)
	return projectReport[ComplexityResult](result, err)
}
func AnalyzeSecurity(code string) (*SecurityResult, error) {
	result, err := core.NewStaticAnalyzer(nil).AnalyzeSecurity(context.Background(), code)
	return projectReport[SecurityResult](result, err)
}
func AnalyzeBugs(code string) (*BugResult, error) {
	result, err := core.NewStaticAnalyzer(nil).AnalyzeBugs(context.Background(), code)
	return projectReport[BugResult](result, err)
}

// PerformAnalysis retains the existing entry point for legacy callers.
func PerformAnalysis(code string) (*AnalysisResult, error) {
	return PerformAnalysisContext(context.Background(), code)
}

func PerformAnalysisContext(ctx context.Context, code string) (*AnalysisResult, error) {
	result, err := core.NewStaticAnalyzer(nil).AnalyzeCode(ctx, code, "")
	if err != nil {
		return nil, err
	}
	compatible, err := projectReport[AnalysisResult](result, nil)
	if err != nil {
		return nil, err
	}
	compatible.AnalyzedAt = time.Now().Format(time.RFC3339)
	return compatible, nil
}

func (r *AnalysisResult) ToJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}
