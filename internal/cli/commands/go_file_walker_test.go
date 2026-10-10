package commands

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCollectGoFilesFromSingleProductionFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result, err := collectGoFiles(file)
	if err != nil {
		t.Fatalf("collectGoFiles failed: %v", err)
	}

	if result.TargetPath != file {
		t.Fatalf("TargetPath = %q, want %q", result.TargetPath, file)
	}
	if len(result.ProductionFiles) != 1 {
		t.Fatalf("ProductionFiles length = %d, want 1", len(result.ProductionFiles))
	}
	if result.ProductionFiles[0] != file {
		t.Fatalf("ProductionFiles[0] = %q, want %q", result.ProductionFiles[0], file)
	}
	if result.TotalFiles != 1 || result.ProductionFileCount != 1 || result.TestFileCount != 0 {
		t.Fatalf("unexpected counts: %+v", result)
	}
}

func TestCollectGoFilesFromDirectorySkipsNoiseAndCountsTests(t *testing.T) {
	tmp := t.TempDir()
	files := []string{
		"main.go",
		"main_test.go",
		"README.md",
		filepath.Join("pkg", "service.go"),
		filepath.Join("vendor", "dep.go"),
		filepath.Join("node_modules", "module.go"),
		filepath.Join(".git", "ignored.go"),
		filepath.Join("build", "generated.go"),
	}

	for _, rel := range files {
		path := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	result, err := collectGoFiles(tmp)
	if err != nil {
		t.Fatalf("collectGoFiles failed: %v", err)
	}

	if result.TotalFiles != 3 {
		t.Fatalf("TotalFiles = %d, want 3; files=%v", result.TotalFiles, result.Files)
	}
	if result.ProductionFileCount != 2 {
		t.Fatalf("ProductionFileCount = %d, want 2", result.ProductionFileCount)
	}
	if result.TestFileCount != 1 {
		t.Fatalf("TestFileCount = %d, want 1", result.TestFileCount)
	}
	if len(result.SkippedDirs) != 4 {
		t.Fatalf("SkippedDirs length = %d, want 4; dirs=%v", len(result.SkippedDirs), result.SkippedDirs)
	}

	for _, got := range result.Files {
		base := filepath.ToSlash(got)
		for _, skipped := range []string{"/vendor/", "/node_modules/", "/.git/", "/build/"} {
			if strings.Contains(base, skipped) {
				t.Fatalf("collected file from skipped dir: %s", got)
			}
		}
	}
}

func TestCollectGoFilesRejectsUnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permission behavior differs on Windows")
	}

	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "main.go"), "package main\n")
	writeTestFile(t, filepath.Join(tmp, "blocked", "blocked.go"), "package blocked\n")

	blockedDir := filepath.Join(tmp, "blocked")
	if err := os.Chmod(blockedDir, 0000); err != nil {
		t.Fatalf("chmod blocked dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blockedDir, 0755)
	})

	if _, err := os.ReadDir(blockedDir); err == nil {
		t.Skip("requires a user without permission to read the blocked directory")
	}
	_, err := collectGoFiles(tmp)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("collectGoFiles error = %v, want permission error rather than incomplete success", err)
	}
}

func TestCollectGoFilesUsesGoToolchainExtensionRules(t *testing.T) {
	tmp := t.TempDir()
	upperGo := filepath.Join(tmp, "main.GO")
	if err := os.WriteFile(upperGo, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := collectGoFiles(upperGo)
	if err == nil {
		t.Fatal("expected error for uppercase .GO extension, got nil")
	}
}

func TestCollectGoFilesRejectsNonGoFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(file, []byte("# docs\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := collectGoFiles(file)
	if err == nil {
		t.Fatal("expected error for non-Go file, got nil")
	}
}

func TestCollectGoFilesEmptyDirectory(t *testing.T) {
	tmp := t.TempDir()

	result, err := collectGoFiles(tmp)
	if err != nil {
		t.Fatalf("collectGoFiles failed: %v", err)
	}

	if result.TotalFiles != 0 {
		t.Fatalf("TotalFiles = %d, want 0", result.TotalFiles)
	}
	if result.ProductionFileCount != 0 {
		t.Fatalf("ProductionFileCount = %d, want 0", result.ProductionFileCount)
	}
	if result.TestFileCount != 0 {
		t.Fatalf("TestFileCount = %d, want 0", result.TestFileCount)
	}
}

func TestCollectGoFilesOnlyTestFiles(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "handler_test.go"), "package main\nfunc TestHandler(t *testing.T) {}\n")
	writeTestFile(t, filepath.Join(tmp, "service_test.go"), "package main\nfunc TestService(t *testing.T) {}\n")

	result, err := collectGoFiles(tmp)
	if err != nil {
		t.Fatalf("collectGoFiles failed: %v", err)
	}

	if result.TotalFiles != 2 {
		t.Fatalf("TotalFiles = %d, want 2", result.TotalFiles)
	}
	if result.ProductionFileCount != 0 {
		t.Fatalf("ProductionFileCount = %d, want 0", result.ProductionFileCount)
	}
	if result.TestFileCount != 2 {
		t.Fatalf("TestFileCount = %d, want 2", result.TestFileCount)
	}
}

func TestCollectGoFilesNonExistentPath(t *testing.T) {
	_, err := collectGoFiles("/nonexistent/path/abc")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}
