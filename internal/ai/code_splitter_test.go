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
	chunks = chunksOfType(chunks, "function")
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}

	assertChunkMetadata(t, chunks[0].Metadata, "main.go", "function", "Alpha", 3, 6)
	assertChunkMetadata(t, chunks[1].Metadata, "main.go", "function", "Beta", 8, 10)

	if strings.Contains(chunks[0].PageContent, "func Beta") {
		t.Fatal("first function chunk contains following function Beta")
	}
}

func TestCodeSplitterUsesPhysicalPositionsForLineDirectives(t *testing.T) {
	code := "package sample\n//line mapped.go:100\nfunc Hello() {}\n//line another.go:1\n// World is a second declaration.\nfunc World() {}"
	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{{PageContent: code, Metadata: map[string]any{"source": "sample.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(code, "\n")
	var reconstructed []string
	for _, chunk := range chunks {
		start, end := chunk.Metadata["start_line"].(int), chunk.Metadata["end_line"].(int)
		if start < 1 || end > len(lines) || end < start {
			t.Fatalf("invalid physical range: %+v", chunk.Metadata)
		}
		if chunk.PageContent != strings.Join(lines[start-1:end], "\n") {
			t.Fatalf("content differs from physical source: %+v", chunk)
		}
		if chunk.Metadata["source"] != "sample.go" {
			t.Fatalf("source replaced by directive: %+v", chunk.Metadata)
		}
		reconstructed = append(reconstructed, chunk.PageContent)
	}
	if strings.Join(reconstructed, "\n") != code {
		t.Fatal("source omitted or duplicated")
	}
	functions := chunksOfType(chunks, "function")
	if len(functions) != 2 || functions[0].Metadata["end_line"] != 3 || functions[1].Metadata["end_line"] != 6 {
		t.Fatalf("wrong declaration positions: %+v", functions)
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
	chunks = chunksOfType(chunks, "function")
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
	chunks = chunksOfType(chunks, "function")
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
	chunks = chunksOfType(chunks, "function")
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

func chunksOfType(chunks []schema.Document, chunkType string) []schema.Document {
	var selected []schema.Document
	for _, chunk := range chunks {
		if chunk.Metadata["chunk_type"] == chunkType {
			selected = append(selected, chunk)
		}
	}
	return selected
}

func TestCodeSplitter_IncludesAllDeclarations(t *testing.T) {
	doc := schema.Document{PageContent: strings.Join([]string{
		"// Package demo explains the example.", "package demo", "", "import \"fmt\"", "",
		"// Options describes configuration.", "type Options struct {", "    Enabled bool", "}", "",
		"const Default = 1", "var Current = Options{}", "func Show() { fmt.Println(Current) }",
	}, "\n"), Metadata: map[string]any{"source": "demo.go"}}
	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"file", "import", "type", "const", "var", "function"} {
		if len(chunksOfType(chunks, kind)) == 0 {
			t.Errorf("missing %s chunk", kind)
		}
	}
	lines := strings.Split(doc.PageContent, "\n")
	covered := make(map[int]bool)
	for _, chunk := range chunks {
		start, end := metadataInt(chunk.Metadata["start_line"]), metadataInt(chunk.Metadata["end_line"])
		if chunk.PageContent != strings.Join(lines[start-1:end], "\n") {
			t.Fatalf("content does not match source range %d-%d: %q", start, end, chunk.PageContent)
		}
		for line := start; line <= end; line++ {
			covered[line] = true
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) != "" && !covered[i+1] {
			t.Errorf("source line %d was omitted", i+1)
		}
	}
}

func TestCodeSplitter_PackageOnlyAndInvalidConfiguration(t *testing.T) {
	chunks, err := NewCodeSplitter().SplitDocuments([]schema.Document{{PageContent: "package demo"}})
	if err != nil || len(chunks) != 1 || chunks[0].PageContent != "package demo" {
		t.Fatalf("package-only source: chunks=%v err=%v", chunks, err)
	}
	for _, maxLines := range []int{0, -1} {
		_, err := (&CodeSplitter{MaxLines: maxLines}).SplitDocuments([]schema.Document{{PageContent: "invalid source"}})
		if err == nil {
			t.Fatalf("expected error for MaxLines=%d", maxLines)
		}
	}
}

func TestCodeSplitter_LongCommentAndTrailingLinesArePreserved(t *testing.T) {
	lines := []string{"package demo", ""}
	for i := 0; i < 9; i++ {
		lines = append(lines, "// comment")
	}
	lines = append(lines, "func Long() {", "    if true {", "        println(1)", "    }", "    println(2)", "}")
	doc := schema.Document{PageContent: strings.Join(lines, "\n")}
	chunks, err := (&CodeSplitter{MaxLines: 4, MinLines: 3}).SplitDocuments([]schema.Document{doc})
	if err != nil {
		t.Fatal(err)
	}
	var covered []int
	for _, chunk := range chunksOfType(chunks, "function") {
		start, end := metadataInt(chunk.Metadata["start_line"]), metadataInt(chunk.Metadata["end_line"])
		if end-start+1 > 4 {
			t.Fatalf("chunk exceeds line limit: %d-%d", start, end)
		}
		if chunk.PageContent != strings.Join(lines[start-1:end], "\n") {
			t.Fatalf("duplicate or fabricated lines in %d-%d", start, end)
		}
		for i := start; i <= end; i++ {
			covered = append(covered, i)
		}
	}
	if len(covered) != len(lines)-2 {
		t.Fatalf("covered %d function lines, want %d", len(covered), len(lines)-2)
	}
	for i, line := range covered {
		if line != i+3 {
			t.Fatalf("function lines missing or duplicated: %v", covered)
		}
	}
}

func TestCodeSplitter_RespectsUTF8ByteLimit(t *testing.T) {
	line := "// " + strings.Repeat("码", 12000)
	chunks, err := (&CodeSplitter{MaxLines: 5, MinLines: 1}).SplitDocuments([]schema.Document{{PageContent: "package demo\n" + line + "\n" + line}})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatal("UTF-8 source was not split at the byte limit")
	}
	for _, chunk := range chunks {
		if len(chunk.PageContent) > maxLenContent {
			t.Fatalf("chunk has %d bytes", len(chunk.PageContent))
		}
	}
	_, err = NewCodeSplitter().SplitDocuments([]schema.Document{{PageContent: "package demo\n// " + strings.Repeat("x", maxLenContent)}})
	if err == nil {
		t.Fatal("expected oversized single line to produce an actionable error")
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
