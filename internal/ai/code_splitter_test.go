package ai

import (
	"strings"
	"testing"

	"github.com/tmc/langchaingo/schema"
)

func TestCodeSplitter_FunctionChunkMetadata(t *testing.T) {
	doc := schema.Document{
		PageContent: strings.Join([]string{
			"package main",
			"",
			"// Alpha handles alpha.",
			"func Alpha() {",
			"\tprintln(\"alpha\")",
			"}",
			"",
			"func Beta() {",
			"\tprintln(\"beta\")",
			"}",
		}, "\n"),
		Metadata: map[string]any{
			"source":          "main.go",
			"relative_source": "main.go",
			"file_name":       "main.go",
		},
	}

	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatalf("SplitDocuments returned error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}

	assertChunkMetadata(t, chunks[0].Metadata, "main.go", "function", "Alpha", 3, 6)
	assertChunkMetadata(t, chunks[1].Metadata, "main.go", "function", "Beta", 8, 10)

	if strings.Contains(chunks[0].PageContent, "func Beta") {
		t.Fatal("first function chunk contains following function Beta")
	}
}

func TestCodeSplitter_PreservesSource(t *testing.T) {
	doc := schema.Document{
		PageContent: strings.Join([]string{
			"package main",
			"",
			"func Alpha() {}",
			"",
			"func Beta() {}",
		}, "\n"),
		Metadata: map[string]any{
			"source":          "main.go",
			"relative_source": "main.go",
			"file_name":       "main.go",
			"custom":          "keep-me",
		},
	}

	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatalf("SplitDocuments returned error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}

	for _, chunk := range chunks {
		if chunk.Metadata["source"] != "main.go" {
			t.Fatalf("source = %#v, want main.go", chunk.Metadata["source"])
		}
		if chunk.Metadata["custom"] != "keep-me" {
			t.Fatalf("custom = %#v, want keep-me", chunk.Metadata["custom"])
		}
	}

	chunks[0].Metadata["source"] = "polluted"
	chunks[0].Metadata["custom"] = "polluted"
	chunks[0].Metadata["symbol"] = "polluted"

	if doc.Metadata["source"] != "main.go" {
		t.Fatalf("input metadata source was mutated: %#v", doc.Metadata["source"])
	}
	if doc.Metadata["custom"] != "keep-me" {
		t.Fatalf("input metadata custom was mutated: %#v", doc.Metadata["custom"])
	}
	if _, ok := doc.Metadata["symbol"]; ok {
		t.Fatalf("input metadata got chunk-only symbol: %#v", doc.Metadata["symbol"])
	}
	if chunks[1].Metadata["source"] != "main.go" {
		t.Fatalf("second chunk source was polluted: %#v", chunks[1].Metadata["source"])
	}
	if chunks[1].Metadata["custom"] != "keep-me" {
		t.Fatalf("second chunk custom was polluted: %#v", chunks[1].Metadata["custom"])
	}
	if chunks[1].Metadata["symbol"] == "polluted" {
		t.Fatal("chunk metadata maps are shared")
	}
}

func TestCodeSplitter_MetadataValueIsolation(t *testing.T) {
	doc := schema.Document{
		PageContent: strings.Join([]string{
			"package main",
			"",
			"func Alpha() {}",
			"",
			"func Beta() {}",
		}, "\n"),
		Metadata: map[string]any{
			"source":  "main.go",
			"tags":    []string{"auth", "api"},
			"weights": []float64{0.5, 0.8},
			"flags":   []bool{true, false},
			"nested": map[string][]string{
				"owners": {"team-a", "team-b"},
			},
			"extra": map[string]any{
				"owner":  "team-a",
				"scores": []float64{1.5, 2.5},
			},
		},
	}

	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatalf("SplitDocuments returned error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}

	chunks[0].Metadata["tags"].([]string)[0] = "polluted"
	chunks[0].Metadata["weights"].([]float64)[0] = 9.9
	chunks[0].Metadata["flags"].([]bool)[0] = false
	chunks[0].Metadata["nested"].(map[string][]string)["owners"][0] = "polluted"
	chunks[0].Metadata["extra"].(map[string]any)["owner"] = "polluted"
	chunks[0].Metadata["extra"].(map[string]any)["scores"].([]float64)[0] = 9.9

	if doc.Metadata["tags"].([]string)[0] != "auth" {
		t.Fatalf("input metadata tags were mutated: %#v", doc.Metadata["tags"])
	}
	if chunks[1].Metadata["tags"].([]string)[0] != "auth" {
		t.Fatalf("second chunk tags were polluted: %#v", chunks[1].Metadata["tags"])
	}
	if doc.Metadata["weights"].([]float64)[0] != 0.5 {
		t.Fatalf("input metadata weights were mutated: %#v", doc.Metadata["weights"])
	}
	if chunks[1].Metadata["weights"].([]float64)[0] != 0.5 {
		t.Fatalf("second chunk weights were polluted: %#v", chunks[1].Metadata["weights"])
	}
	if doc.Metadata["flags"].([]bool)[0] != true {
		t.Fatalf("input metadata flags were mutated: %#v", doc.Metadata["flags"])
	}
	if chunks[1].Metadata["flags"].([]bool)[0] != true {
		t.Fatalf("second chunk flags were polluted: %#v", chunks[1].Metadata["flags"])
	}
	if doc.Metadata["nested"].(map[string][]string)["owners"][0] != "team-a" {
		t.Fatalf("input metadata nested owners were mutated: %#v", doc.Metadata["nested"])
	}
	if chunks[1].Metadata["nested"].(map[string][]string)["owners"][0] != "team-a" {
		t.Fatalf("second chunk nested owners were polluted: %#v", chunks[1].Metadata["nested"])
	}
	if doc.Metadata["extra"].(map[string]any)["owner"] != "team-a" {
		t.Fatalf("input metadata extra owner was mutated: %#v", doc.Metadata["extra"])
	}
	if chunks[1].Metadata["extra"].(map[string]any)["owner"] != "team-a" {
		t.Fatalf("second chunk extra owner was polluted: %#v", chunks[1].Metadata["extra"])
	}
	if doc.Metadata["extra"].(map[string]any)["scores"].([]float64)[0] != 1.5 {
		t.Fatalf("input metadata extra scores were mutated: %#v", doc.Metadata["extra"])
	}
	if chunks[1].Metadata["extra"].(map[string]any)["scores"].([]float64)[0] != 1.5 {
		t.Fatalf("second chunk extra scores were polluted: %#v", chunks[1].Metadata["extra"])
	}
}

func TestCodeSplitter_SyntaxErrorFallback(t *testing.T) {
	splitter := &CodeSplitter{MaxLines: 2, MinLines: 1}
	doc := schema.Document{
		PageContent: strings.Join([]string{
			"package main",
			"func broken(",
			"line three",
			"line four",
			"line five",
		}, "\n"),
		Metadata: map[string]any{"source": "broken.go"},
	}

	chunks, err := splitter.SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatalf("SplitDocuments returned error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("len(chunks) = %d, want 3", len(chunks))
	}

	assertTextChunkMetadata(t, chunks[0].Metadata, "broken.go", 1, 2)
	assertTextChunkMetadata(t, chunks[1].Metadata, "broken.go", 3, 4)
	assertTextChunkMetadata(t, chunks[2].Metadata, "broken.go", 5, 5)
}

func TestCodeSplitter_LargeFunctionSplit(t *testing.T) {
	splitter := &CodeSplitter{MaxLines: 4, MinLines: 1}
	doc := schema.Document{
		PageContent: strings.Join([]string{
			"package main",
			"",
			"func Big() {",
			"\tif true {",
			"\t\tprintln(1)",
			"\t}",
			"\tfor i := 0; i < 1; i++ {",
			"\t\tprintln(i)",
			"\t}",
			"}",
		}, "\n"),
		Metadata: map[string]any{"source": "big.go"},
	}

	chunks, err := splitter.SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatalf("SplitDocuments returned error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want at least 2", len(chunks))
	}

	for _, chunk := range chunks {
		if chunk.Metadata["chunk_type"] != "function" {
			t.Fatalf("chunk_type = %#v, want function", chunk.Metadata["chunk_type"])
		}
		if chunk.Metadata["symbol"] != "Big" {
			t.Fatalf("symbol = %#v, want Big", chunk.Metadata["symbol"])
		}
		if chunk.Metadata["source"] != "big.go" {
			t.Fatalf("source = %#v, want big.go", chunk.Metadata["source"])
		}
		if _, ok := chunk.Metadata["start_line"]; !ok {
			t.Fatalf("missing start_line in metadata: %#v", chunk.Metadata)
		}
		if _, ok := chunk.Metadata["end_line"]; !ok {
			t.Fatalf("missing end_line in metadata: %#v", chunk.Metadata)
		}
	}
}

func assertChunkMetadata(t *testing.T, got map[string]any, source, chunkType, symbol string, startLine, endLine int) {
	t.Helper()

	if got["source"] != source {
		t.Fatalf("source = %#v, want %q", got["source"], source)
	}
	if got["chunk_type"] != chunkType {
		t.Fatalf("chunk_type = %#v, want %q", got["chunk_type"], chunkType)
	}
	if got["symbol"] != symbol {
		t.Fatalf("symbol = %#v, want %q", got["symbol"], symbol)
	}
	if got["start_line"] != startLine {
		t.Fatalf("start_line = %#v, want %d", got["start_line"], startLine)
	}
	if got["end_line"] != endLine {
		t.Fatalf("end_line = %#v, want %d", got["end_line"], endLine)
	}
}

func assertTextChunkMetadata(t *testing.T, got map[string]any, source string, startLine, endLine int) {
	t.Helper()

	if got["source"] != source {
		t.Fatalf("source = %#v, want %q", got["source"], source)
	}
	if got["chunk_type"] != "text" {
		t.Fatalf("chunk_type = %#v, want text", got["chunk_type"])
	}
	if got["start_line"] != startLine {
		t.Fatalf("start_line = %#v, want %d", got["start_line"], startLine)
	}
	if got["end_line"] != endLine {
		t.Fatalf("end_line = %#v, want %d", got["end_line"], endLine)
	}
}
