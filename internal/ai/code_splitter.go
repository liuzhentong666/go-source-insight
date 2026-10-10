package ai

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/tmc/langchaingo/schema"
)

// CodeSplitter 智能代码分块器
// 用于将Go源代码按照函数、结构体等逻辑单元进行智能分割
type CodeSplitter struct {
	MaxLines int // 单个块最大行数
	MinLines int // 单个块最小行数
}

// NewCodeSplitter 创建新的分块器
// 返回一个具有默认配置的分块器实例
func NewCodeSplitter() *CodeSplitter {
	return &CodeSplitter{
		MaxLines: 100, // 最大100行
		MinLines: 10,  // 最小10行
	}
}

// SplitDocuments 按 Go 函数/结构分块
// 将输入的文档切片按照Go语言的语法结构进行智能分割
func (cs *CodeSplitter) SplitDocuments(docs []schema.Document) ([]schema.Document, error) {
	if cs == nil || cs.MaxLines <= 0 {
		return nil, fmt.Errorf("split: max lines must be positive")
	}
	if cs.MinLines < 0 || cs.MinLines > cs.MaxLines {
		return nil, fmt.Errorf("split: min lines must be between zero and max lines")
	}
	var chunks []schema.Document

	for _, doc := range docs {
		// 解析 Go 代码
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", doc.PageContent, parser.ParseComments)
		if err != nil {
			// A syntax error must not silently omit source from the index.
			simpleChunks, splitErr := cs.splitRange(strings.Split(doc.PageContent, "\n"), 0, len(strings.Split(doc.PageContent, "\n"))-1, "text", "", doc.Metadata)
			if splitErr != nil {
				return nil, splitErr
			}
			chunks = append(chunks, simpleChunks...)
			continue
		}

		// 提取代码行
		lines := strings.Split(doc.PageContent, "\n")

		cursor := 0
		for _, declaration := range node.Decls {
			start, end := fset.PositionFor(declaration.Pos(), false).Line-1, fset.PositionFor(declaration.End(), false).Line-1
			chunkType, symbol := "declaration", ""
			switch decl := declaration.(type) {
			case *ast.FuncDecl:
				chunkType, symbol = "function", decl.Name.Name
				if decl.Doc != nil {
					start = fset.PositionFor(decl.Doc.Pos(), false).Line - 1
				}
			case *ast.GenDecl:
				chunkType = decl.Tok.String()
				if decl.Doc != nil {
					start = fset.PositionFor(decl.Doc.Pos(), false).Line - 1
				}
				if len(decl.Specs) > 0 {
					switch spec := decl.Specs[0].(type) {
					case *ast.TypeSpec:
						symbol = spec.Name.Name
					case *ast.ValueSpec:
						if len(spec.Names) > 0 {
							symbol = spec.Names[0].Name
						}
					}
				}
			}
			// Include package, detached comments and other source between declarations.
			if cursor < start {
				prefix, err := cs.splitRange(lines, cursor, start-1, "file", "", doc.Metadata)
				if err != nil {
					return nil, err
				}
				chunks = append(chunks, prefix...)
			}
			declarationChunks, err := cs.splitRange(lines, start, end, chunkType, symbol, doc.Metadata)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, declarationChunks...)
			cursor = end + 1
		}
		if cursor < len(lines) {
			tail, err := cs.splitRange(lines, cursor, len(lines)-1, "file", "", doc.Metadata)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, tail...)
		}
	}

	return chunks, nil
}

// splitRange preserves contiguous source and never discards a short trailing chunk.
// Each chunk respects both the line limit and Milvus VarChar's UTF-8 byte limit.
func (cs *CodeSplitter) splitRange(lines []string, start, end int, chunkType, symbol string, metadata map[string]any) ([]schema.Document, error) {
	var chunks []schema.Document
	if start < 0 || end >= len(lines) || start > end {
		return nil, fmt.Errorf("split: invalid source range %d-%d", start+1, end+1)
	}
	for currentStart := start; currentStart <= end; {
		limit := min(currentStart+cs.MaxLines, end+1)
		currentEnd, byteCount := currentStart, 0
		for currentEnd < limit {
			lineBytes := len(lines[currentEnd])
			if lineBytes > maxLenContent {
				return nil, fmt.Errorf("split: source line %d exceeds %d bytes", currentEnd+1, maxLenContent)
			}
			if currentEnd > currentStart {
				lineBytes++ // newline separating adjacent source lines
			}
			if byteCount+lineBytes > maxLenContent {
				break
			}
			byteCount += lineBytes
			currentEnd++
		}
		// Prefer a nearby logical boundary for long functions without losing any line.
		if chunkType == "function" && currentEnd <= end {
			for boundary := currentEnd - 1; boundary >= currentStart+max(1, cs.MinLines); boundary-- {
				if cs.isLogicalSplitPoint(strings.TrimSpace(lines[boundary])) {
					currentEnd = boundary
					break
				}
			}
		}
		code := strings.Join(lines[currentStart:currentEnd], "\n")
		if strings.TrimSpace(code) != "" {
			chunks = append(chunks, schema.Document{
				PageContent: code,
				Metadata: chunkMetadata(metadata, map[string]any{
					"chunk_type": chunkType,
					"symbol":     symbol,
					"start_line": currentStart + 1,
					"end_line":   currentEnd,
				}),
			})
		}
		currentStart = currentEnd
	}

	return chunks, nil
}

// isLogicalSplitPoint 判断是否是逻辑分割点
// 根据关键字识别代码中的逻辑分割点
func (cs *CodeSplitter) isLogicalSplitPoint(line string) bool {
	logicalKeywords := []string{
		"if ", "for ", "switch ", "case ",
		"} else {", "} else if",
		"//分割点", "// section",
	}

	for _, keyword := range logicalKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

func chunkMetadata(base map[string]any, fields map[string]any) map[string]any {
	metadata := make(map[string]any, len(base)+len(fields))
	for k, v := range base {
		metadata[k] = cloneMetadataValue(v)
	}
	for k, v := range fields {
		metadata[k] = cloneMetadataValue(v)
	}
	return metadata
}

func cloneMetadataValue(value any) any {
	if value == nil {
		return nil
	}
	return cloneMetadataReflect(reflect.ValueOf(value)).Interface()
}

func cloneMetadataReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneMetadataReflect(value.Elem())
		if cloned.Type().AssignableTo(value.Type()) {
			return cloned
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Elem().Type())
		cloned.Elem().Set(cloneMetadataReflect(value.Elem()))
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneMetadataReflect(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneMetadataReflect(value.Index(i)))
		}
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		for _, key := range value.MapKeys() {
			cloned.SetMapIndex(key, cloneMetadataReflect(value.MapIndex(key)))
		}
		return cloned
	default:
		return value
	}
}
