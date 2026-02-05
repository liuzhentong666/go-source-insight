package ai

import (
	"github.com/tmc/langchaingo/schema"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// CodeSplitter 智能代码分块器
type CodeSplitter struct {
	MaxLines int // 单个块最大行数
	MinLines int // 单个块最小行数
}

// NewCodeSplitter 创建新的分块器
func NewCodeSplitter() *CodeSplitter {
	return &CodeSplitter{
		MaxLines: 100, // 最大100行
		MinLines: 10,  // 最小10行
	}
}

// SplitDocuments 按 Go 函数/结构分块
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

				// 检查函数大小
				if end-start+1 <= cs.MaxLines {
					// 函数不大，直接作为一个块
					chunks = append(chunks, schema.Document{
						PageContent: cs.addContext(lines, start, end, doc.Metadata),
						Metadata:    doc.Metadata,
					})
				} else {
					// 函数太大，按逻辑子块分割
					subChunks := cs.splitLargeFunction(lines, start, end, doc.Metadata)
					chunks = append(chunks, subChunks...)
				}
			}
			return true
		})
	}

	return chunks, nil
}

// addContext 添加注释和上下文
func (cs *CodeSplitter) addContext(lines []string, start, end int, metadata map[string]any) string {
	// 往前查找注释
	contextStart := start
	for i := start - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "//") {
			contextStart = i
		} else {
			break
		}
	}

	// 往后查找相邻函数
	contextEnd := end
	if end+1 < len(lines) && end-start < 50 {
		// 如果函数较小，可能包含相邻的小函数
		for i := end + 1; i < len(lines) && i < end+30; i++ {
			if strings.TrimSpace(lines[i]) != "" {
				contextEnd = i
			}
		}
	}

	return strings.Join(lines[contextStart:contextEnd+1], "\n")
}

// splitLargeFunction 分割大函数
func (cs *CodeSplitter) splitLargeFunction(lines []string, start, end int, metadata map[string]any) []schema.Document {
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
			if i-currentStart >= cs.MaxLines ||
				cs.isLogicalSplitPoint(line) {
				// 创建一个块
				code := commentBuffer +
					strings.Join(lines[currentStart:i+1], "\n")
				chunks = append(chunks, schema.Document{
					PageContent: code,
					Metadata:    metadata,
				})
				// 重置
				currentStart = i + 1
				commentBuffer = ""
			}
		}
	}

	// 添加最后一块
	if currentStart <= end {
		code := commentBuffer + strings.Join(lines[currentStart:end+1],
			"\n")
		chunks = append(chunks, schema.Document{
			PageContent: code,
			Metadata:    metadata,
		})
	}

	return chunks
}

// isLogicalSplitPoint 判断是否是逻辑分割点
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
			Metadata:    doc.Metadata,
		})
	}

	return chunks
}
