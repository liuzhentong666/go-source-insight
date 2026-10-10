package ai

import (
	"testing"

	"github.com/tmc/langchaingo/schema"
)

func TestCodeChunkFromDocument_PreservesReferenceMetadata(t *testing.T) {
	doc := schema.Document{
		PageContent: "func Example() {}",
		Metadata: map[string]any{
			"source":     "example.go",
			"chunk_type": "function",
			"start_line": 10,
			"end_line":   20,
		},
	}

	chunk := codeChunkFromDocument(2, doc)

	if chunk.ID != "example.go:10-20#2" {
		t.Fatalf("ID = %q, want %q", chunk.ID, "example.go:10-20#2")
	}
	if chunk.Source != "example.go" {
		t.Fatalf("Source = %q, want %q", chunk.Source, "example.go")
	}
	if chunk.StartLine != 10 || chunk.EndLine != 20 {
		t.Fatalf("line range = %d-%d, want 10-20", chunk.StartLine, chunk.EndLine)
	}
	if chunk.ChunkType != "function" {
		t.Fatalf("ChunkType = %q, want %q", chunk.ChunkType, "function")
	}
}

func TestCodeChunkFromDocument_AcceptsJSONDecodedNumberMetadata(t *testing.T) {
	doc := schema.Document{
		PageContent: "func Example() {}",
		Metadata: map[string]any{
			"source":     "example.go",
			"chunk_type": "function",
			"start_line": float64(10),
			"end_line":   float64(20),
		},
	}

	chunk := codeChunkFromDocument(2, doc)

	if chunk.ID != "example.go:10-20#2" {
		t.Fatalf("ID = %q, want %q", chunk.ID, "example.go:10-20#2")
	}
	if chunk.StartLine != 10 || chunk.EndLine != 20 {
		t.Fatalf("line range = %d-%d, want 10-20", chunk.StartLine, chunk.EndLine)
	}
}
