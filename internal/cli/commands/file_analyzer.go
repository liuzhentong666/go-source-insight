package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	Path             string `json:"path"`
	Functions        int    `json:"functions"`
	ComplexityIssues int    `json:"complexity_issues"`
	SecurityIssues   int    `json:"security_issues"`
	BugIssues        int    `json:"bug_issues"`
}

func analyzeTarget(ctx context.Context, toolManager *tools.ToolManager, target string) (StaticAnalysisResult, error) {
	scanResult, err := collectGoFiles(target)
	if err != nil {
		return StaticAnalysisResult{}, err
	}

	result := StaticAnalysisResult{TargetPath: target}
	for _, file := range scanResult.ProductionFiles {
		fileResult, err := analyzeGoFile(ctx, toolManager, file)
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

func analyzeGoFile(ctx context.Context, toolManager *tools.ToolManager, path string) (FileAnalysisResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	code := string(content)

	complexityOutput, err := runTool(ctx, toolManager, "complexity_analyzer", code)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	securityOutput, err := runTool(ctx, toolManager, "security_scanner", code)
	if err != nil {
		return FileAnalysisResult{}, err
	}
	bugOutput, err := runTool(ctx, toolManager, "bug_detector", code)
	if err != nil {
		return FileAnalysisResult{}, err
	}

	var complexity tools.ComplexityResult
	if err := json.Unmarshal([]byte(complexityOutput), &complexity); err != nil {
		return FileAnalysisResult{}, fmt.Errorf("解析 complexity_analyzer 输出失败: %w", err)
	}
	var security tools.SecurityResult
	if err := json.Unmarshal([]byte(securityOutput), &security); err != nil {
		return FileAnalysisResult{}, fmt.Errorf("解析 security_scanner 输出失败: %w", err)
	}
	var bug tools.BugResult
	if err := json.Unmarshal([]byte(bugOutput), &bug); err != nil {
		return FileAnalysisResult{}, fmt.Errorf("解析 bug_detector 输出失败: %w", err)
	}

	complexityIssues := 0
	for _, fn := range complexity.Functions {
		if len(fn.Issues) > 0 {
			complexityIssues++
		}
	}

	return FileAnalysisResult{
		Path:             path,
		Functions:        len(complexity.Functions),
		ComplexityIssues: complexityIssues,
		SecurityIssues:   security.Total,
		BugIssues:        bug.Total,
	}, nil
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
