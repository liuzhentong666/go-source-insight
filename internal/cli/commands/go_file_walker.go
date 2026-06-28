package commands

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GoFileScanResult describes the Go files discovered under a target path.
type GoFileScanResult struct {
	TargetPath          string   `json:"target_path"`
	TotalFiles          int      `json:"total_files"`
	ProductionFileCount int      `json:"production_file_count"`
	TestFileCount       int      `json:"test_file_count"`
	TotalLines          int      `json:"total_lines"`
	SkippedDirs         []string `json:"skipped_dirs"`
	Files               []string `json:"files"`
	ProductionFiles     []string `json:"production_files"`
	TestFiles           []string `json:"test_files"`
}

func collectGoFiles(target string) (GoFileScanResult, error) {
	info, err := os.Stat(target)
	if err != nil {
		return GoFileScanResult{}, fmt.Errorf("访问路径失败: %w", err)
	}

	result := GoFileScanResult{TargetPath: target}

	if !info.IsDir() {
		if !isGoFile(target) {
			return GoFileScanResult{}, fmt.Errorf("目标不是 Go 源文件: %s", target)
		}
		if err := addGoFile(&result, target); err != nil {
			return GoFileScanResult{}, err
		}
		finalizeGoFileScanResult(&result)
		return result, nil
	}

	err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.SkippedDirs = append(result.SkippedDirs, path)
			return nil
		}

		if d.IsDir() {
			if path == target {
				return nil
			}
			if shouldSkipDir(d.Name()) {
				result.SkippedDirs = append(result.SkippedDirs, path)
				return filepath.SkipDir
			}
			return nil
		}

		if isGoFile(path) {
			return addGoFile(&result, path)
		}

		return nil
	})
	if err != nil {
		return GoFileScanResult{}, fmt.Errorf("扫描路径失败: %w", err)
	}

	finalizeGoFileScanResult(&result)
	return result, nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "dist", "build", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func isGoFile(path string) bool {
	return filepath.Ext(path) == ".go"
}

func isGoTestFile(path string) bool {
	return filepath.Ext(path) == ".go" && strings.HasSuffix(path, "_test.go")
}

func addGoFile(result *GoFileScanResult, path string) error {
	lineCount, err := countFileLines(path)
	if err != nil {
		return err
	}

	result.Files = append(result.Files, path)
	result.TotalLines += lineCount
	if isGoTestFile(path) {
		result.TestFiles = append(result.TestFiles, path)
	} else {
		result.ProductionFiles = append(result.ProductionFiles, path)
	}
	return nil
}

func countFileLines(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("读取文件失败 %s: %w", path, err)
	}
	if len(content) == 0 {
		return 0, nil
	}

	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	if content[len(content)-1] == '\n' {
		lines--
	}
	return lines, nil
}

func finalizeGoFileScanResult(result *GoFileScanResult) {
	sort.Strings(result.Files)
	sort.Strings(result.ProductionFiles)
	sort.Strings(result.TestFiles)
	sort.Strings(result.SkippedDirs)
	result.TotalFiles = len(result.Files)
	result.ProductionFileCount = len(result.ProductionFiles)
	result.TestFileCount = len(result.TestFiles)
}
