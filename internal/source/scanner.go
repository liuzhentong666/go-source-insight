// Package source discovers Go source files for both static analysis and indexing.
package source

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type ScanResult struct {
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

// AccessError distinguishes source I/O failures from source syntax failures
// while preserving existing CLI error text and errors.Is behavior.
type AccessError struct{ Err error }

func (e *AccessError) Error() string { return e.Err.Error() }
func (e *AccessError) Unwrap() error { return e.Err }

func ShouldSkipDir(name string) bool {
	if runtime.GOOS == "windows" {
		name = strings.ToLower(name)
	}
	switch name {
	case ".git", "vendor", "node_modules", "dist", "build", "bin", ".runtime", ".idea", ".vscode", ".hermes", ".qoder", ".codex":
		return true
	default:
		return false
	}
}

func IsGoFile(path string) bool   { return filepath.Ext(path) == ".go" }
func IsTestFile(path string) bool { return IsGoFile(path) && strings.HasSuffix(path, "_test.go") }

// Scan reports paths that can be opened by the caller. Indexing converts them to
// project-relative references before sending metadata to the vector store.
// Unreadable non-ignored sources fail the scan; a partial result must never be
// used to replace an index because missing files could be mistaken for deletions.
func Scan(ctx context.Context, target string) (ScanResult, error) {
	return scan(ctx, target, os.Lstat, os.ReadFile, filepath.WalkDir)
}

// ScanFS uses caller-controlled filesystem access and returns relative paths.
// A confined filesystem can enforce stronger boundaries than ordinary CLI paths.
func ScanFS(ctx context.Context, filesystem fs.FS, target string) (ScanResult, error) {
	if !fs.ValidPath(target) {
		return ScanResult{}, fmt.Errorf("invalid relative source target")
	}
	skipDir := ShouldSkipDir
	if policy, ok := filesystem.(interface{ ShouldSkipSourceDir(string) bool }); ok {
		skipDir = policy.ShouldSkipSourceDir
	}
	return scanWithPolicy(ctx, target,
		func(name string) (fs.FileInfo, error) { return fs.Stat(filesystem, name) },
		func(name string) ([]byte, error) { return fs.ReadFile(filesystem, name) },
		func(name string, fn fs.WalkDirFunc) error { return fs.WalkDir(filesystem, name, fn) }, skipDir)
}

func scan(ctx context.Context, target string, stat func(string) (fs.FileInfo, error), read func(string) ([]byte, error), walk func(string, fs.WalkDirFunc) error) (ScanResult, error) {
	return scanWithPolicy(ctx, target, stat, read, walk, ShouldSkipDir)
}
func scanWithPolicy(ctx context.Context, target string, stat func(string) (fs.FileInfo, error), read func(string) ([]byte, error), walk func(string, fs.WalkDirFunc) error, skipDir func(string) bool) (ScanResult, error) {
	result := ScanResult{TargetPath: target, Files: []string{}, ProductionFiles: []string{}, TestFiles: []string{}, SkippedDirs: []string{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	info, err := stat(target)
	if err != nil {
		return result, fmt.Errorf("访问源码路径失败: %w", &AccessError{Err: err})
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("源码入口不能是符号链接")
	}
	components := strings.Split(filepath.ToSlash(target), "/")
	if !info.IsDir() {
		components = components[:len(components)-1]
	}
	for _, component := range components {
		if skipDir(component) {
			return result, fmt.Errorf("source target is inside an excluded directory: %w", &AccessError{Err: fs.ErrPermission})
		}
	}
	add := func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		content, err := read(path)
		if err != nil {
			return fmt.Errorf("读取源码失败: %w", &AccessError{Err: err})
		}
		if len(content) > 0 {
			result.TotalLines += strings.Count(string(content), "\n")
			if content[len(content)-1] != '\n' {
				result.TotalLines++
			}
		}
		result.Files = append(result.Files, path)
		if IsTestFile(path) {
			result.TestFiles = append(result.TestFiles, path)
		} else {
			result.ProductionFiles = append(result.ProductionFiles, path)
		}
		return nil
	}
	if !info.IsDir() {
		if !IsGoFile(target) {
			return result, fmt.Errorf("目标不是 Go 源文件")
		}
		err = add(target)
	} else {
		err = walk(target, func(path string, d fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return fmt.Errorf("无法读取源码目录: %w", &AccessError{Err: walkErr})
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				if path != target && skipDir(d.Name()) {
					result.SkippedDirs = append(result.SkippedDirs, path)
					return filepath.SkipDir
				}
				return nil
			}
			if IsGoFile(path) {
				return add(path)
			}
			return nil
		})
	}
	if err != nil {
		return result, err
	}
	sort.Strings(result.Files)
	sort.Strings(result.ProductionFiles)
	sort.Strings(result.TestFiles)
	sort.Strings(result.SkippedDirs)
	result.TotalFiles = len(result.Files)
	result.ProductionFileCount = len(result.ProductionFiles)
	result.TestFileCount = len(result.TestFiles)
	return result, nil
}
