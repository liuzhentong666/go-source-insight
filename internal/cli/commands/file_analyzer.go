package commands

import (
	"context"
	"go-ai-study/internal/analysis"
	"go-ai-study/internal/localfiles"
	"go-ai-study/internal/tools"
	"io/fs"
)

// These aliases preserve the CLI report contract while the shared service owns it.
type StaticAnalysisResult = analysis.StaticAnalysisResult
type FileAnalysisResult = analysis.FileAnalysisResult

func analyzeTarget(ctx context.Context, manager *tools.ToolManager, target string) (StaticAnalysisResult, error) {
	provider, err := localfiles.FromEnvironment()
	if err != nil {
		return StaticAnalysisResult{}, err
	}
	if provider.URL != "" && localfiles.IsAbsolute(target) {
		var result StaticAnalysisResult
		err := provider.WithFS(ctx, target, func(files fs.FS, relative string) error {
			var err error
			result, err = analysis.NewStaticAnalyzer(manager).AnalyzeFS(ctx, files, relative)
			return err
		})
		return result, err
	}
	return analysis.NewStaticAnalyzer(manager).AnalyzeTarget(ctx, target)
}
