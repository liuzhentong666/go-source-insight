package source

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

type failingFS struct{ fs.FS }

func (s failingFS) Open(name string) (fs.File, error) {
	if name == "blocked.go" {
		return nil, fs.ErrPermission
	}
	return s.FS.Open(name)
}

func TestScanFSKeepsRelativePathsAndFailsOnUnreadableSources(t *testing.T) {
	files := fstest.MapFS{"main.go": {Data: []byte("package sample\n")}, "main_test.go": {Data: []byte("package sample\n")}, "pkg/helper.go": {Data: []byte("package sample\n")}, "vendor/ignored.go": {Data: []byte("ignored")}}
	got, err := ScanFS(context.Background(), files, ".")
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetPath != "." || got.TotalFiles != 3 || got.ProductionFileCount != 2 || got.TestFileCount != 1 || strings.Join(got.Files, ",") != "main.go,main_test.go,pkg/helper.go" || strings.Join(got.SkippedDirs, ",") != "vendor" {
		t.Fatalf("incorrect fs scan: %+v", got)
	}
	files["blocked.go"] = &fstest.MapFile{Data: []byte("package sample")}
	_, err = ScanFS(context.Background(), failingFS{files}, ".")
	var access *AccessError
	if !errors.Is(err, fs.ErrPermission) || !errors.As(err, &access) {
		t.Fatalf("source access failure not preserved: %v", err)
	}
}

func TestExcludedDirectoriesCannotBeDirectTargets(t *testing.T) {
	files := fstest.MapFS{"main.go": {Data: []byte("package p")}, ".runtime/secret.go": {Data: []byte("package secret")}, "bin/output.go": {Data: []byte("package output")}, "business/sources/code.go": {Data: []byte("package business")}}
	result, err := ScanFS(context.Background(), files, ".")
	if err != nil || result.TotalFiles != 2 {
		t.Fatalf("default exclusions=%+v %v", result, err)
	}
	for _, target := range []string{".runtime", ".runtime/secret.go", "bin", "bin/output.go"} {
		if _, err := ScanFS(context.Background(), files, target); !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("direct excluded target %q accepted: %v", target, err)
		}
	}
}
