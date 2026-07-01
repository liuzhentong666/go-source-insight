package ai

import (
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
	var chunks []schema.Document

	for _, doc := range docs {
		// 解析 Go 代码
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", doc.PageContent, parser.ParseComments)
		if err != nil {
			// 如果解析失败（比如不是Go代码），用简单的行分割
			simpleChunks := cs.simpleSplitByLines(doc)
			chunks = append(chunks, simpleChunks...)
			continue
		}

		// 提取代码行
		lines := strings.Split(doc.PageContent, "\n")

		// 遍历 AST，提取函数
		ast.Inspect(node, func(n ast.Node) bool {
			if fnDecl, ok := n.(*ast.FuncDecl); ok {
				// 获取函数的起始和结束位置
				start := fset.Position(fnDecl.Pos()).Line - 1
				end := fset.Position(fnDecl.End()).Line - 1

				// 边界检查
				if start < 0 || end >= len(lines) || start > end {
					return true
				}

				// 检查函数大小
				if end-start+1 <= cs.MaxLines {
					// 函数不大，直接作为一个块
					code, contextStart, contextEnd := cs.addContext(lines, start, end)
					chunks = append(chunks, schema.Document{
						PageContent: code,
						Metadata: chunkMetadata(doc.Metadata, map[string]any{
							"chunk_type": "function",
							"symbol":     fnDecl.Name.Name,
							"start_line": contextStart + 1,
							"end_line":   contextEnd + 1,
						}),
					})
				} else {
					// 函数太大，按逻辑子块分割
					subChunks := cs.splitLargeFunction(lines, start, end, fnDecl.Name.Name, doc.Metadata)
					chunks = append(chunks, subChunks...)
				}
			}
			return true
		})
	}

	return chunks, nil
}

// addContext 添加注释上下文
// 向前查找关联的注释，不向后吞并相邻函数，避免污染检索内容
func (cs *CodeSplitter) addContext(lines []string, start, end int) (string, int, int) {
	// 往前查找注释
	contextStart := start
	for i := start - 1; i >= 0; i-- {
		trimmedLine := strings.TrimSpace(lines[i])
		if trimmedLine == "" {
			continue
		}
		if strings.HasPrefix(trimmedLine, "//") {
			contextStart = i
		} else {
			break
		}
	}

	contextEnd := end

	// 边界保护
	if contextStart < 0 {
		contextStart = 0
	}
	if contextEnd >= len(lines) {
		contextEnd = len(lines) - 1
	}
	if contextStart > contextEnd {
		return "", contextStart, contextEnd
	}

	return strings.Join(lines[contextStart:contextEnd+1], "\n"), contextStart, contextEnd
}

// splitLargeFunction 分割大函数
// 按照逻辑分割点将大函数拆分为多个较小的块
func (cs *CodeSplitter) splitLargeFunction(lines []string, start, end int, symbol string, metadata map[string]any) []schema.Document {
	var chunks []schema.Document
	currentStart := start
	commentBuffer := ""

	for i := start; i <= end; i++ {
		line := strings.TrimSpace(lines[i])

		// 收集注释
		if strings.HasPrefix(line, "//") {
			if commentBuffer == "" {
				currentStart = i
			}
			commentBuffer += lines[i] + "\n"
			continue
		}

		// 遇到代码，检查是否应该分割
		if line != "" {
			// 检查是否达到最大行数或逻辑分割点
			if i-currentStart >= cs.MaxLines || cs.isLogicalSplitPoint(line) {
				// 创建一个块
				code := commentBuffer + strings.Join(lines[currentStart:i+1], "\n")
				chunks = append(chunks, schema.Document{
					PageContent: code,
					Metadata: chunkMetadata(metadata, map[string]any{
						"chunk_type": "function",
						"symbol":     symbol,
						"start_line": currentStart + 1,
						"end_line":   i + 1,
					}),
				})
				// 重置
				currentStart = i + 1
				commentBuffer = ""
			}
		}
	}

	// 添加最后一块
	if currentStart <= end {
		code := commentBuffer + strings.Join(lines[currentStart:end+1], "\n")
		chunks = append(chunks, schema.Document{
			PageContent: code,
			Metadata: chunkMetadata(metadata, map[string]any{
				"chunk_type": "function",
				"symbol":     symbol,
				"start_line": currentStart + 1,
				"end_line":   end + 1,
			}),
		})
	}

	return chunks
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

// simpleSplitByLines 简单的行分割（用于非Go代码）
// 按照固定行数分割非Go代码或解析失败的内容
func (cs *CodeSplitter) simpleSplitByLines(doc schema.Document) []schema.Document {
	var chunks []schema.Document
	lines := strings.Split(doc.PageContent, "\n")

	for i := 0; i < len(lines); i += cs.MaxLines {
		end := i + cs.MaxLines
		if end > len(lines) {
			end = len(lines)
		}
		chunks = append(chunks, schema.Document{
			PageContent: strings.Join(lines[i:end], "\n"),
			Metadata: chunkMetadata(doc.Metadata, map[string]any{
				"chunk_type": "text",
				"start_line": i + 1,
				"end_line":   end,
			}),
		})
	}

	return chunks
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
