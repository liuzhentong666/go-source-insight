// Package analysis provides shared static analysis without model or database dependencies.
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"go-ai-study/internal/source"
	"go-ai-study/internal/tools"
)

// StaticAnalysisResult is the aggregated static-analysis report for one target.
type StaticAnalysisResult struct {
	TargetPath        string               `json:"target_path"`
	FilesAnalyzed     int                  `json:"files_analyzed"`
	FunctionsAnalyzed int                  `json:"functions_analyzed"`
	ComplexityIssues  int                  `json:"complexity_issues"`
	SecurityIssues    int                  `json:"security_issues"`
	BugIssues         int                  `json:"bug_issues"`
	Files             []FileAnalysisResult `json:"files"`
}

// FileAnalysisResult is the static-analysis summary for one Go source file.
type FileAnalysisResult struct {
	Path             string                  `json:"path"`
	Functions        int                     `json:"functions"`
	ComplexityIssues int                     `json:"complexity_issues"`
	SecurityIssues   int                     `json:"security_issues"`
	BugIssues        int                     `json:"bug_issues"`
	Complexity       *tools.ComplexityResult `json:"complexity"`
	Security         *tools.SecurityResult   `json:"security"`
	Bugs             *tools.BugResult        `json:"bugs"`
}

func (s *StaticAnalyzer) AnalyzeTarget(ctx context.Context, target string) (StaticAnalysisResult, error) {
	scanResult, err := source.Scan(ctx, target)
	if err != nil {
		return StaticAnalysisResult{}, err
	}
	return s.analyzeFiles(ctx, target, scanResult, s.AnalyzeFile)
}

// AnalyzeFS keeps scanning and reads inside the caller's filesystem boundary.
func (s *StaticAnalyzer) AnalyzeFS(ctx context.Context, filesystem fs.FS, target string) (StaticAnalysisResult, error) {
	scanned, err := source.ScanFS(ctx, filesystem, target)
	if err != nil {
		return StaticAnalysisResult{}, err
	}
	return s.analyzeFiles(ctx, target, scanned, func(ctx context.Context, name string) (FileAnalysisResult, error) {
		if err := ctx.Err(); err != nil {
			return FileAnalysisResult{}, err
		}
		data, err := fs.ReadFile(filesystem, name)
		if err != nil {
			return FileAnalysisResult{}, &source.AccessError{Err: err}
		}
		return s.AnalyzeCode(ctx, string(data), name)
	})
}

func (s *StaticAnalyzer) analyzeFiles(ctx context.Context, target string, scanResult source.ScanResult, analyze func(context.Context, string) (FileAnalysisResult, error)) (StaticAnalysisResult, error) {
	result := StaticAnalysisResult{TargetPath: target, Files: []FileAnalysisResult{}}
	for _, file := range scanResult.ProductionFiles {
		fileResult, err := analyze(ctx, file)
		if err != nil {
			return StaticAnalysisResult{}, err
		}
		result.Files = append(result.Files, fileResult)
		result.FilesAnalyzed++
		result.FunctionsAnalyzed += fileResult.Functions
		result.ComplexityIssues += fileResult.ComplexityIssues
		result.SecurityIssues += fileResult.SecurityIssues
		result.BugIssues += fileResult.BugIssues
	}

	return result, nil
}

func (s *StaticAnalyzer) AnalyzeFile(ctx context.Context, path string) (FileAnalysisResult, error) {
	if err := ctx.Err(); err != nil {
		return FileAnalysisResult{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	return s.AnalyzeCode(ctx, string(content), path)
}

// AnalyzeCode analyzes a source string. Path labels the report; it is never opened.
func (s *StaticAnalyzer) AnalyzeCode(ctx context.Context, code, path string) (FileAnalysisResult, error) {
	complexity, err := s.AnalyzeComplexity(ctx, code)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	security, err := s.AnalyzeSecurity(ctx, code)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	bugs, err := s.AnalyzeBugs(ctx, code)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	complexityIssues := 0
	for _, fn := range complexity.Functions {
		if len(fn.Issues) > 0 {
			complexityIssues++
		}
	}
	return FileAnalysisResult{
		Path: path, Functions: len(complexity.Functions),
		ComplexityIssues: complexityIssues, SecurityIssues: security.Total, BugIssues: bugs.Total,
		Complexity: complexity, Security: security, Bugs: bugs,
	}, nil
}

// StaticAnalyzer runs the same tool registry for both CLI and API callers.
// Constructing it does not contact Ollama or Milvus.
type StaticAnalyzer struct{ manager *tools.ToolManager }

// NewStaticAnalyzer accepts an existing configured registry, or installs the
// standard static tools when manager is nil.
func NewStaticAnalyzer(manager *tools.ToolManager) *StaticAnalyzer {
	if manager == nil {
		manager = tools.NewToolManager(nil)
		for _, tool := range []tools.Tool{tools.NewComplexityAnalyzer(), tools.NewSecurityScanner(), tools.NewBugDetector()} {
			// A fresh registry has no duplicate names and these tools have fixed names.
			_ = manager.Register(tool, tools.DefaultToolConfig(tool.Name()))
		}
	}
	return &StaticAnalyzer{manager: manager}
}

func (s *StaticAnalyzer) AnalyzeComplexity(ctx context.Context, code string) (*tools.ComplexityResult, error) {
	var result tools.ComplexityResult
	if err := s.decodeTool(ctx, "complexity_analyzer", code, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (s *StaticAnalyzer) AnalyzeSecurity(ctx context.Context, code string) (*tools.SecurityResult, error) {
	var result tools.SecurityResult
	if err := s.decodeTool(ctx, "security_scanner", code, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (s *StaticAnalyzer) AnalyzeBugs(ctx context.Context, code string) (*tools.BugResult, error) {
	var result tools.BugResult
	if err := s.decodeTool(ctx, "bug_detector", code, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (s *StaticAnalyzer) decodeTool(ctx context.Context, name, code string, target any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := runTool(ctx, s.manager, name, code)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return fmt.Errorf("解析 %s 输出失败: %w", name, err)
	}
	return nil
}

func runTool(ctx context.Context, toolManager *tools.ToolManager, name string, input any) (string, error) {
	result, err := toolManager.Run(ctx, name, input)
	if err != nil {
		return "", fmt.Errorf("执行 %s 失败: %w", name, err)
	}
	if !result.Success {
		if result.Error != "" {
			return "", fmt.Errorf("%s 执行失败: %s: %w", name, result.Error, tools.ErrToolExecution)
		}
		return "", fmt.Errorf("%s 执行失败: %w", name, tools.ErrToolExecution)
	}
	return result.Result, nil
}
