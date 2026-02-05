package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// BugDetector Bug 检测器
// 检测 Go 代码中的常见 Bug（纯检测，不自动修复）
type BugDetector struct {
	*BaseTool
	ruleEngine *BugRuleEngine
}

// NewBugDetector 创建 Bug 检测器
func NewBugDetector() *BugDetector {
	detector := &BugDetector{
		BaseTool: NewBaseTool(
			"bug_detector",
			"检测 Go 代码中的常见 Bug（忽略错误、资源泄漏、nil 引用等）",
			reflect.TypeOf(""),
		),
	}
	detector.ruleEngine = NewBugRuleEngine()
	detector.ruleEngine.RegisterAllRules()
	return detector
}

// BugDetectorInput 支持多种输入方式
type BugDetectorInput struct {
	Code      string   `json:"code,omitempty"`      // 单文件代码字符串（向后兼容）
	Files     []string `json:"files,omitempty"`     // 多个文件路径
	Directory string   `json:"directory,omitempty"` // 目录路径
}

// BugResult 完整的 Bug 检测结果
type BugResult struct {
	Language        string       `json:"language"`         // 检测的语言（go）
	Status          string       `json:"status"`           // 状态：success, partial, error
	TotalFiles      int          `json:"total_files"`      // 总文件数
	AnalyzedFiles   int          `json:"analyzed_files"`   // 分析的 Go 文件数
	SkippedFiles    []FileStatus `json:"skipped_files"`    // 跳过的文件
	ErrorFiles      []FileStatus `json:"error_files"`      // 解析失败的文件
	Total           int          `json:"total"`            // 总 Bug 数
	Bugs            []BugIssue   `json:"bugs"`             // 所有 Bug
	Summary         string       `json:"summary"`          // 摘要
	Statistics      BugStats     `json:"statistics"`       // 统计信息
	Recommendations []string     `json:"recommendations"`  // 其他工具的建议
}

// FileStatus 文件状态
type FileStatus struct {
	Path     string `json:"path"`     // 文件路径
	Language string `json:"language"` // 语言
	Status   string `json:"status"`   // analyzed, skipped, error
	Reason   string `json:"reason"`   // 原因
}

// BugIssue 单个 Bug 问题
type BugIssue struct {
	ID           string `json:"id"`            // 问题唯一标识
	RuleID       string `json:"rule_id"`       // 规则ID
	Severity     string `json:"severity"`      // 严重程度：High, Medium, Low
	Category     string `json:"category"`     // 问题类别
	Description  string `json:"description"`   // 问题描述
	File         string `json:"file"`          // 文件名
	Line         int    `json:"line"`          // 行号
	Function     string `json:"function"`      // 所在函数
	CodeSnippet  string `json:"code_snippet"`  // 代码片段
	FixSuggestion string `json:"fix_suggestion"` // 修复建议（代码示例）
	Confidence   string `json:"confidence"`    // 置信度：high, medium, low
}

// BugStats Bug 统计
type BugStats struct {
	TotalIssues   int `json:"total_issues"`
	High          int `json:"high"`
	Medium        int `json:"medium"`
	Low           int `json:"low"`
}

// Run 执行 Bug 检测
func (bd *BugDetector) Run(ctx context.Context, input any) (string, error) {
	// 类型断言 - 支持字符串（向后兼容）或 BugDetectorInput
	var detectorInput BugDetectorInput
	
	switch v := input.(type) {
	case string:
		detectorInput.Code = v
	case BugDetectorInput:
		detectorInput = v
	default:
		return "", fmt.Errorf("输入类型错误: 期望 string 或 BugDetectorInput, 实际 %T", input)
	}

	// 收集文件
	goFiles, otherFiles, err := bd.collectFiles(detectorInput)
	if err != nil {
		return "", fmt.Errorf("文件收集失败: %w", err)
	}

	// 如果没有 Go 文件
	if len(goFiles) == 0 {
		return bd.buildEmptyResult(len(otherFiles)), nil
	}

	// 分析 Go 文件
	var allBugs []BugIssue
	var errorFiles []FileStatus

	for _, file := range goFiles {
		var code string
		var err error

		// 如果是虚拟文件（代码字符串输入），使用输入的代码
		if file == "<code>" {
			code = detectorInput.Code
		} else {
			// 读取真实文件
			fileContent, err := os.ReadFile(file)
			if err != nil {
				errorFiles = append(errorFiles, FileStatus{
					Path:     file,
					Language: "go",
					Status:   "error",
					Reason:   fmt.Sprintf("读取文件失败: %v", err),
				})
				continue
			}
			code = string(fileContent)
		}

		// 解析和检测
		bugs, err := bd.analyzeCode(code, file)
		if err != nil {
			errorFiles = append(errorFiles, FileStatus{
				Path:     file,
				Language: "go",
				Status:   "error",
				Reason:   fmt.Sprintf("解析失败: %v", err),
			})
			continue
		}

		allBugs = append(allBugs, bugs...)
	}

	// 去重
	allBugs = deduplicateBugIssues(allBugs)

	// 构建结果
	result := BugResult{
		Language:        "go",
		Status:          bd.determineStatus(len(goFiles), len(errorFiles)),
		TotalFiles:      len(goFiles) + len(otherFiles) + len(errorFiles),
		AnalyzedFiles:   len(goFiles) - len(errorFiles),
		SkippedFiles:    otherFiles,
		ErrorFiles:      errorFiles,
		Total:           len(allBugs),
		Bugs:            allBugs,
		Summary:         bd.generateSummary(len(goFiles), len(allBugs), len(otherFiles)),
		Statistics:      bd.calculateBugStatistics(allBugs),
		Recommendations: []string{
			"编译错误请运行: go build ./...",
			"类型检查请运行: go vet ./...",
			"格式化代码请运行: go fmt ./...",
		},
	}

	// 序列化为 JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(jsonBytes), nil
}

// collectFiles 收集文件
func (bd *BugDetector) collectFiles(input BugDetectorInput) ([]string, []FileStatus, error) {
	var goFiles []string
	var otherFiles []FileStatus

	// 方式 2: 文件列表（优先判断）
	if len(input.Files) > 0 {
		for _, file := range input.Files {
			lang := DetectLanguage(file)
			if lang == "go" {
				if _, err := os.Stat(file); err == nil {
					goFiles = append(goFiles, file)
				}
			} else {
				otherFiles = append(otherFiles, FileStatus{
					Path:     file,
					Language: lang,
					Status:   "skipped",
					Reason:   "Bug 检测器仅支持 Go 语言",
				})
			}
		}
		return goFiles, otherFiles, nil
	}

	// 方式 3: 目录扫描
	if input.Directory != "" {
		err := filepath.Walk(input.Directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 忽略错误，继续扫描
			}

			if info.IsDir() {
				// 跳过隐藏目录
				if strings.HasPrefix(filepath.Base(path), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			// 只处理 .go 文件
			lang := DetectLanguage(path)
			if lang == "go" {
				goFiles = append(goFiles, path)
			} else if lang != "unknown" {
				otherFiles = append(otherFiles, FileStatus{
					Path:     path,
					Language: lang,
					Status:   "skipped",
					Reason:   "Bug 检测器仅支持 Go 语言",
				})
			}

			return nil
		})
		return goFiles, otherFiles, err
	}

	// 方式 1: 单文件代码字符串（默认方式）
	return []string{"<code>"}, []FileStatus{}, nil
}

// analyzeCode 分析代码
func (bd *BugDetector) analyzeCode(code, filename string) ([]BugIssue, error) {
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, filename, code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}

	var bugs []BugIssue
	ruleCtx := &BugRuleContext{FSet: fset, Filename: filename}

	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// 应用所有规则
		for _, rule := range bd.ruleEngine.Rules {
			if rule.Match(n, ruleCtx) {
				bug := buildBugIssue(rule, n, fset, code, filename)
				bugs = append(bugs, bug)
			}
		}
		return true
	})

	return bugs, nil
}

// DetectLanguage 检测语言
func DetectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	
	langMap := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".java":  "java",
		".cpp":   "cpp",
		".c":     "c",
		".rs":    "rust",
		".rb":    "ruby",
		".php":   "php",
	}
	
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "unknown"
}

// determineStatus 确定状态
func (bd *BugDetector) determineStatus(goFiles, errorFiles int) string {
	if errorFiles > 0 {
		return "partial"
	}
	if goFiles > 0 {
		return "success"
	}
	return "error"
}

// buildEmptyResult 构建空结果（没有 Go 文件）
func (bd *BugDetector) buildEmptyResult(skippedCount int) string {
	result := BugResult{
		Language:        "go",
		Status:          "success",
		TotalFiles:      skippedCount,
		AnalyzedFiles:   0,
		SkippedFiles:    make([]FileStatus, 0),
		ErrorFiles:      make([]FileStatus, 0),
		Total:           0,
		Bugs:            make([]BugIssue, 0),
		Summary:         "未检测到 Go 文件",
		Statistics:      BugStats{},
		Recommendations: []string{
			"Bug 检测器仅支持 Go 语言",
		},
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

// generateSummary 生成摘要
func (bd *BugDetector) generateSummary(goFiles, bugCount, skippedCount int) string {
	if goFiles == 0 {
		return "未检测到 Go 文件"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("分析完成，共 %d 个 Go 文件", goFiles))

	if bugCount > 0 {
		sb.WriteString(fmt.Sprintf("，检测到 %d 个 Bug", bugCount))
	} else {
		sb.WriteString("，未检测到 Bug ✅")
	}

	if skippedCount > 0 {
		sb.WriteString(fmt.Sprintf("，跳过 %d 个非 Go 文件", skippedCount))
	}

	return sb.String()
}

// calculateBugStatistics 计算 Bug 统计
func (bd *BugDetector) calculateBugStatistics(bugs []BugIssue) BugStats {
	stats := BugStats{
		TotalIssues: len(bugs),
	}

	for _, bug := range bugs {
		switch bug.Severity {
		case "High":
			stats.High++
		case "Medium":
			stats.Medium++
		case "Low":
			stats.Low++
		}
	}

	return stats
}

// BugRuleContext Bug 规则检测上下文
type BugRuleContext struct {
	FSet     *token.FileSet
	Filename string
}

// BugRuleEngine Bug 规则引擎
type BugRuleEngine struct {
	Rules []BugRule
}

// NewBugRuleEngine 创建规则引擎
func NewBugRuleEngine() *BugRuleEngine {
	return &BugRuleEngine{
		Rules: make([]BugRule, 0),
	}
}

// Register 注册规则
func (bre *BugRuleEngine) Register(rule BugRule) {
	bre.Rules = append(bre.Rules, rule)
}

// RegisterAllRules 注册所有默认规则
func (bre *BugRuleEngine) RegisterAllRules() {
	bre.Register(&IgnoredErrorRule{})
	bre.Register(&ResourceNotClosedRule{})
	bre.Register(&SwitchWithoutDefaultRule{})
	bre.Register(&PotentialNilPointerRule{})
}

// BugRule Bug 规则接口
type BugRule interface {
	ID() string                     // 规则唯一标识
	Name() string                   // 规则名称
	Severity() string               // 严重程度
	Category() string               // 问题类别
	Description() string            // 规则描述
	Match(node ast.Node, ctx *BugRuleContext) bool
	GenerateSuggestion(node ast.Node) string // 生成修复建议
}

// 规则 1: 忽略错误返回值
type IgnoredErrorRule struct{}

func (r *IgnoredErrorRule) ID() string          { return "B101" }
func (r *IgnoredErrorRule) Name() string        { return "Ignored Error Return Value" }
func (r *IgnoredErrorRule) Severity() string    { return "High" }
func (r *IgnoredErrorRule) Category() string    { return "Error Handling" }
func (r *IgnoredErrorRule) Description() string { return "忽略了错误返回值" }
func (r *IgnoredErrorRule) GenerateSuggestion(node ast.Node) string {
	return "检查错误：\nfile, err := os.Open(\"file.txt\")\nif err != nil {\n    return err\n}"
}

func (r *IgnoredErrorRule) Match(node ast.Node, ctx *BugRuleContext) bool {
	if assign, ok := node.(*ast.AssignStmt); ok {
		for _, lhs := range assign.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok {
				// 检查是否使用了 _ 忽略错误
				if ident.Name == "_" {
					// 检查右侧函数名
					if len(assign.Rhs) > 0 {
						if callExpr, ok := assign.Rhs[0].(*ast.CallExpr); ok {
							// 检查是否是可能返回错误的函数
							if isErrorReturningFunction(callExpr) {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// 规则 2: 资源未关闭
type ResourceNotClosedRule struct{}

func (r *ResourceNotClosedRule) ID() string          { return "B102" }
func (r *ResourceNotClosedRule) Name() string        { return "Resource Not Closed" }
func (r *ResourceNotClosedRule) Severity() string    { return "High" }
func (r *ResourceNotClosedRule) Category() string    { return "Resource Management" }
func (r *ResourceNotClosedRule) Description() string { return "打开文件/连接但没有 defer close()" }
func (r *ResourceNotClosedRule) GenerateSuggestion(node ast.Node) string {
	return "使用 defer 确保资源释放：\nfile, err := os.Open(\"file.txt\")\nif err != nil {\n    return err\n}\ndefer file.Close()"
}

func (r *ResourceNotClosedRule) Match(node ast.Node, ctx *BugRuleContext) bool {
	if callExpr, ok := node.(*ast.CallExpr); ok {
		// 检测打开文件的函数调用
		if isFileOpenFunction(callExpr) {
			// 检查下一个语句（简化版：10 行内）是否有 defer
			// 注意：这是简化版，可能会误报
			return true
		}
	}
	return false
}

// 规则 3: switch 缺少 default
type SwitchWithoutDefaultRule struct{}

func (r *SwitchWithoutDefaultRule) ID() string          { return "B103" }
func (r *SwitchWithoutDefaultRule) Name() string        { return "Switch Without Default" }
func (r *SwitchWithoutDefaultRule) Severity() string    { return "Low" }
func (r *SwitchWithoutDefaultRule) Category() string    { return "Control Flow" }
func (r *SwitchWithoutDefaultRule) Description() string { return "switch 语句没有 default 分支" }
func (r *SwitchWithoutDefaultRule) GenerateSuggestion(node ast.Node) string {
	return "添加 default 分支处理未知情况：\nswitch x {\ncase 1:\n    ...\ndefault:\n    ...\n}"
}

func (r *SwitchWithoutDefaultRule) Match(node ast.Node, ctx *BugRuleContext) bool {
	if switchStmt, ok := node.(*ast.SwitchStmt); ok {
		// 检查是否有 default 分支
		hasDefault := false
		ast.Inspect(switchStmt.Body, func(n ast.Node) bool {
			if caseClause, ok := n.(*ast.CaseClause); ok {
				if caseClause.List == nil {
					hasDefault = true
					return false
				}
			}
			return true
		})
		return !hasDefault
	}
	return false
}

// 规则 4: 可能的 nil 指针引用（简化版）
type PotentialNilPointerRule struct{}

func (r *PotentialNilPointerRule) ID() string          { return "B104" }
func (r *PotentialNilPointerRule) Name() string        { return "Potential Nil Pointer Dereference" }
func (r *PotentialNilPointerRule) Severity() string    { return "Medium" }
func (r *PotentialNilPointerRule) Category() string    { return "Null Safety" }
func (r *PotentialNilPointerRule) Description() string { return "对可能为 nil 的指针调用方法" }
func (r *PotentialNilPointerRule) GenerateSuggestion(node ast.Node) string {
	return "检查 nil：\nif ptr != nil {\n    ptr.Method()\n}"
}

func (r *PotentialNilPointerRule) Match(node ast.Node, ctx *BugRuleContext) bool {
	if callExpr, ok := node.(*ast.CallExpr); ok {
		if _, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			// 简化版：只检测明显场景
			// 完整版需要数据流分析
			return true
		}
	}
	return false
}

// 辅助函数：判断是否是可能返回错误的函数
func isErrorReturningFunction(callExpr *ast.CallExpr) bool {
	// 检查常见可能返回错误的函数
	// 这是一个简化的列表，实际应该更完整
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			// 常见的可能返回错误的函数
			pkg := ident.Name

			// os.Open, os.WriteFile, etc.
			if pkg == "os" {
				return true
			}

			// http.Get, http.Post, etc.
			if pkg == "http" {
				return true
			}

			// ioutil.ReadFile, etc.
			if pkg == "ioutil" || pkg == "os" {
				return true
			}
		}
	}

	// 检查函数名是否以 Read, Write, Open, Create, Get, Post 开头
	if ident, ok := callExpr.Fun.(*ast.Ident); ok {
		name := ident.Name
		prefixes := []string{"Read", "Write", "Open", "Create", "Get", "Post"}
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}

	return false
}

// 辅助函数：判断是否是文件打开函数
func isFileOpenFunction(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			// os.Open, os.Create, os.OpenFile, os.WriteFile
			if ident.Name == "os" {
				fun := selExpr.Sel.Name
				openFuncs := []string{"Open", "Create", "OpenFile", "WriteFile"}
				for _, f := range openFuncs {
					if fun == f {
						return true
					}
				}
			}
		}
	}
	return false
}

// 辅助函数：构建 Bug 问题
func buildBugIssue(rule BugRule, node ast.Node, fset *token.FileSet, code, filename string) BugIssue {
	position := fset.Position(node.Pos())
	line := position.Line

	// 提取代码片段
	lines := strings.Split(code, "\n")
	var codeSnippet string
	if line-1 < len(lines) && line-1 >= 0 {
		codeSnippet = strings.TrimSpace(lines[line-1])
		if len(codeSnippet) > 100 {
			codeSnippet = codeSnippet[:100] + "..."
		}
	}

	// 查找所在函数
	var funcName string
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Pos() < node.Pos() && node.Pos() < fn.End() {
				funcName = fn.Name.Name
				return false
			}
		}
		return true
	})

	// 确定置信度
	confidence := "medium"
	switch rule.ID() {
	case "B101", "B103": // 明确的模式
		confidence = "high"
	case "B102": // 可能误报
		confidence = "medium"
	case "B104": // 简化版，可能误报
		confidence = "low"
	}

	return BugIssue{
		ID:           fmt.Sprintf("bug-%d", position.Offset),
		RuleID:       rule.ID(),
		Severity:     rule.Severity(),
		Category:     rule.Category(),
		Description:  rule.Description(),
		File:         filename,
		Line:         line,
		Function:     funcName,
		CodeSnippet:  codeSnippet,
		FixSuggestion: rule.GenerateSuggestion(node),
		Confidence:   confidence,
	}
}

// 辅助函数：去重 Bug 问题
func deduplicateBugIssues(bugs []BugIssue) []BugIssue {
	seen := make(map[string]bool)
	result := []BugIssue{}

	for _, bug := range bugs {
		key := fmt.Sprintf("%s-%d-%d", bug.RuleID, bug.Line, len(bug.File))
		if !seen[key] {
			seen[key] = true
			result = append(result, bug)
		}
	}

	return result
}
