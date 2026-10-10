package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkipsToolMetadataAndCountsFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"main.go": "package main\nfunc main() {}\n", "main_test.go": "package main\n", "readme.md": "ignored", ".hermes/private.go": "package private\n", ".qoder/private.go": "package private\n", "node_modules/dependency.go": "package dep\n"}
	for p, s := range files {
		p = filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalFiles != 2 || result.TotalLines != 3 || result.ProductionFileCount != 1 || result.TestFileCount != 1 || len(result.SkippedDirs) != 3 {
		t.Fatalf("bad discovery: %+v", result)
	}
}
func TestScanCancellationAndUnsupportedFiles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	p := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(p, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), p); err == nil {
		t.Fatal("non-Go file accepted")
	}
}

func TestScanDoesNotReadFilesOutsideRootThroughSymlinks(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "external.go")
	if err := os.WriteFile(outside, []byte("package external\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "linked.go")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalFiles != 0 {
		t.Fatalf("external linked source was read: %+v", result)
	}
	if _, err := Scan(context.Background(), link); err == nil {
		t.Fatal("single-file symlink accepted")
	}
}
