package ai

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go-ai-study/internal/source"
)

type blockedSourceFS struct{ fs.FS }

func (s blockedSourceFS) Open(name string) (fs.File, error) {
	if name == "pkg/blocked.go" {
		return nil, fs.ErrPermission
	}
	return s.FS.Open(name)
}

func TestFSScannerKeepsProjectRelativeSourcesAndPhysicalLines(t *testing.T) {
	code := "package sample\n//line virtual.go:900\nfunc Helper() {\n println(\"example\")\n}\n"
	files := fstest.MapFS{"pkg/helper.go": {Data: []byte(code)}, "pkg/helper_test.go": {Data: []byte("package sample\nfunc TestHelper() {}")}, "pkg/vendor/ignored.go": {Data: []byte("ignored")}, "other.go": {Data: []byte("package sample")}}
	scanner := &FSScanner{FS: files}
	docs, err := scanner.Scan(context.Background(), "pkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("documents=%d want 2", len(docs))
	}
	for _, doc := range docs {
		source, ok := doc.Metadata["source"].(string)
		if !ok || (source != "pkg/helper.go" && source != "pkg/helper_test.go") {
			t.Fatalf("lost project-relative source: %+v", doc.Metadata)
		}
	}
	splitter := NewCodeSplitter()
	splitter.MinLines = 1
	chunks, err := splitter.SplitDocuments(docs)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.PageContent, "func Helper") {
			found = true
			if chunk.Metadata["start_line"] != 2 || chunk.Metadata["end_line"] != 5 || chunk.Metadata["source"] != "pkg/helper.go" {
				t.Fatalf("physical metadata changed: %+v", chunk.Metadata)
			}
		}
	}
	if !found {
		t.Fatal("missing Helper chunk")
	}
	files["pkg/blocked.go"] = &fstest.MapFile{Data: []byte("package sample")}
	failed, err := (&FSScanner{FS: blockedSourceFS{files}}).Scan(context.Background(), "pkg")
	var access *source.AccessError
	if len(failed) != 0 || !errors.Is(err, fs.ErrPermission) || !errors.As(err, &access) {
		t.Fatalf("partial sources admitted: %d %v", len(failed), err)
	}
}
