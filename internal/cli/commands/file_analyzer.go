package commands

import (
	"context"
	"go-ai-study/internal/analysis"
	"go-ai-study/internal/tools"
)

// These aliases preserve the CLI report contract while the shared service owns it.
type StaticAnalysisResult = analysis.StaticAnalysisResult
type FileAnalysisResult = analysis.FileAnalysisResult

func analyzeTarget(ctx context.Context, manager *tools.ToolManager, target string) (StaticAnalysisResult, error) {
	return analysis.NewStaticAnalyzer(manager).AnalyzeTarget(ctx, target)
}
