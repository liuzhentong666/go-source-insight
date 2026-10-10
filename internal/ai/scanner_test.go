package ai

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScanCodeRejectsUnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX directory permissions")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "readable.go"), "package sample\n")
	blocked := filepath.Join(root, "blocked")
	writeTestFile(t, filepath.Join(blocked, "source.go"), "package sample\n")
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0700) })
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("requires non-privileged user")
	}
	docs, err := ScanCode(root)
	if !errors.Is(err, fs.ErrPermission) || len(docs) != 0 {
		t.Fatalf("got %d docs, error=%v; incomplete sources must not reach index replacement", len(docs), err)
	}
}

func TestScanCode_FilePath(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "main.go")
	writeTestFile(t, filePath, "package main\n\nfunc main() {}\n")

	docs, err := ScanCode(filePath)
	if err != nil {
		t.Fatalf("ScanCode returned error: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}
	if docs[0].PageContent != "package main\n\nfunc main() {}\n" {
		t.Fatalf("PageContent = %q", docs[0].PageContent)
	}
	assertMetadata(t, docs[0].Metadata, "main.go", "main.go", "main.go")
}

func TestScanCode_Directory(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.go")
	pkgPath := filepath.Join(root, "pkg", "handler.go")
	textPath := filepath.Join(root, "README.md")
	emptyPath := filepath.Join(root, "empty.go")

	writeTestFile(t, mainPath, "package main\n")
	writeTestFile(t, pkgPath, "package pkg\n\nfunc Handle() {}\n")
	writeTestFile(t, textPath, "not go\n")
	writeTestFile(t, emptyPath, "\n\t\n")

	docs, err := ScanCode(root)
	if err != nil {
		t.Fatalf("ScanCode returned error: %v", err)
	}

	if len(docs) != 2 {
		t.Fatalf("len(docs) = %d, want 2", len(docs))
	}

	byFileName := map[string]int{}
	for _, doc := range docs {
		name, ok := doc.Metadata["file_name"].(string)
		if !ok {
			t.Fatalf("file_name metadata missing or non-string: %#v", doc.Metadata["file_name"])
		}
		byFileName[name]++
	}

	if byFileName["main.go"] != 1 {
		t.Fatalf("main.go count = %d, want 1", byFileName["main.go"])
	}
	if byFileName["handler.go"] != 1 {
		t.Fatalf("handler.go count = %d, want 1", byFileName["handler.go"])
	}
	if byFileName["empty.go"] != 0 {
		t.Fatalf("empty.go count = %d, want 0", byFileName["empty.go"])
	}
}

func TestScanCode_SkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")

	ignoredDirs := []string{".git", "vendor", "node_modules", "dist", "build", ".idea", ".vscode"}
	for _, dir := range ignoredDirs {
		writeTestFile(t, filepath.Join(root, dir, "ignored.go"), "package ignored\n")
	}

	docs, err := ScanCode(root)
	if err != nil {
		t.Fatalf("ScanCode returned error: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}

	assertMetadata(t, docs[0].Metadata, "main.go", "main.go", "main.go")
}

func TestScanCode_NonExistentPath(t *testing.T) {
	_, err := ScanCode(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("ScanCode returned nil error for non-existent path")
	}
}

func TestScanCode_NonGoFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "README.md")
	writeTestFile(t, filePath, "not go\n")

	docs, err := ScanCode(filePath)
	if err != nil {
		t.Fatalf("ScanCode returned error: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("len(docs) = %d, want 0", len(docs))
	}
}

func assertMetadata(t *testing.T, got map[string]any, source, relativeSource, fileName string) {
	t.Helper()

	if got["source"] != filepath.ToSlash(source) {
		t.Fatalf("source = %#v, want %q", got["source"], filepath.ToSlash(source))
	}
	if got["relative_source"] != filepath.ToSlash(relativeSource) {
		t.Fatalf("relative_source = %#v, want %q", got["relative_source"], filepath.ToSlash(relativeSource))
	}
	if got["file_name"] != fileName {
		t.Fatalf("file_name = %#v, want %q", got["file_name"], fileName)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
