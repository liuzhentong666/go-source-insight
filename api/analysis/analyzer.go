package analysis

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"time"
)

// ComplexityResult 复杂度分析结果
type ComplexityResult struct {
	File       string           `json:"file"`
	Total      int              `json:"total"`
	Functions  []FunctionResult `json:"functions"`
	Summary    string           `json:"summary"`
	Statistics Statistics       `json:"statistics"`
}

type FunctionResult struct {
	Name       string   `json:"name"`
	Line       int      `json:"line"`
	Complexity int      `json:"complexity"`
	Lines      int      `json:"lines"`
	Issues     []string `json:"issues"`
}

type Statistics struct {
	TotalFunctions       int `json:"total_functions"`
	SimpleFunctions      int `json:"simple_functions"`
	MediumFunctions      int `json:"medium_functions"`
	ComplexFunctions     int `json:"complex_functions"`
	VeryComplexFunctions int `json:"very_complex_functions"`
}

// AnalyzeComplexity 分析代码复杂度
func AnalyzeComplexity(code string) (*ComplexityResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析代码失败: %w", err)
	}

	var functions []FunctionResult
	totalComplexity := 0

	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			complexity := calculateComplexity(fn)
			line := fset.Position(fn.Pos()).Line
			lines := calculateLines(fset, fn)
			issues := generateIssues(complexity, lines)

			functions = append(functions, FunctionResult{
				Name:       fn.Name.Name,
				Line:       line,
				Complexity: complexity,
				Lines:      lines,
				Issues:     issues,
			})
			totalComplexity += complexity
		}
		return true
	})

	return &ComplexityResult{
		File:       "",
		Total:      totalComplexity,
		Functions:  functions,
		Summary:    generateSummary(functions),
		Statistics: calculateStatistics(functions),
	}, nil
}

func calculateComplexity(fn *ast.FuncDecl) int {
	count := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			count++
		case *ast.ForStmt:
			count++
		case *ast.RangeStmt:
			count++
		case *ast.SwitchStmt:
			count++
		case *ast.CaseClause:
			if node.List != nil {
				count++
			}
		case *ast.TypeSwitchStmt:
			count++
		case *ast.SelectStmt:
			count++
		case *ast.CommClause:
			count++
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				count++
			}
		}
		return true
	})
	return count
}

func calculateLines(fset *token.FileSet, fn *ast.FuncDecl) int {
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	return end - start + 1
}

func generateIssues(complexity, lines int) []string {
	var issues []string
	if complexity > 50 {
		issues = append(issues, "🚨 圈复杂度过高（>50），必须拆分函数！")
	} else if complexity > 20 {
		issues = append(issues, "❌ 圈复杂度较高（>20），建议拆分函数")
	} else if complexity > 10 {
		issues = append(issues, "⚠️ 圈复杂度偏高（>10），可能需要重构")
	}
	if lines > 100 {
		issues = append(issues, "📏 函数过长（>100行），建议拆分")
	} else if lines > 50 {
		issues = append(issues, "📏 函数较长（>50行），可考虑拆分")
	}
	return issues
}

func generateSummary(results []FunctionResult) string {
	if len(results) == 0 {
		return "未找到任何函数"
	}
	total := 0
	for _, r := range results {
		total += r.Complexity
	}
	avg := float64(total) / float64(len(results))
	problemCount := 0
	for _, r := range results {
		if len(r.Issues) > 0 {
			problemCount++
		}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("分析完成，共 %d 个函数，平均复杂度 %.1f", len(results), avg))
	if problemCount > 0 {
		sb.WriteString(fmt.Sprintf("，发现 %d 个函数存在潜在问题", problemCount))
	} else {
		sb.WriteString("，所有函数复杂度正常 ✅")
	}
	return sb.String()
}

func calculateStatistics(results []FunctionResult) Statistics {
	stats := Statistics{TotalFunctions: len(results)}
	for _, r := range results {
		switch {
		case r.Complexity <= 10:
			stats.SimpleFunctions++
		case r.Complexity <= 20:
			stats.MediumFunctions++
		case r.Complexity <= 50:
			stats.ComplexFunctions++
		default:
			stats.VeryComplexFunctions++
		}
	}
	return stats
}

// SecurityResult 安全扫描结果
type SecurityResult struct {
	File       string          `json:"file"`
	Total      int             `json:"total"`
	Issues     []SecurityIssue `json:"issues"`
	Summary    string          `json:"summary"`
	Statistics SecurityStats   `json:"statistics"`
}

type SecurityIssue struct {
	ID          string `json:"id"`
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Line        int    `json:"line"`
	CodeSnippet string `json:"code_snippet"`
	Suggestion  string `json:"suggestion"`
}

type SecurityStats struct {
	TotalIssues int `json:"total_issues"`
	Critical    int `json:"critical"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
}

// AnalyzeSecurity 执行安全扫描
func AnalyzeSecurity(code string) (*SecurityResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析代码失败: %w", err)
	}

	var issues []SecurityIssue

	// 简单的安全规则检测
	ast.Inspect(node, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			checkHardcodedSecret(assign, fset, code, &issues)
		}
		return true
	})

	return &SecurityResult{
		File:       "",
		Total:      len(issues),
		Issues:     issues,
		Summary:    generateSecuritySummary(issues),
		Statistics: calculateSecurityStatistics(issues),
	}, nil
}

func checkHardcodedSecret(assign *ast.AssignStmt, fset *token.FileSet, code string, issues *[]SecurityIssue) {
	secretKeywords := []string{"password", "secret", "api_key", "token", "private_key"}

	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			varName := strings.ToLower(ident.Name)
			for _, keyword := range secretKeywords {
				if strings.Contains(varName, keyword) {
					if len(assign.Rhs) > 0 {
						if _, ok := assign.Rhs[0].(*ast.BasicLit); ok {
							position := fset.Position(assign.Pos())
							*issues = append(*issues, SecurityIssue{
								ID:          fmt.Sprintf("sec-%d", position.Offset),
								RuleID:      "G101",
								Severity:    "Critical",
								Category:    "Credentials",
								Description: "检测到硬编码的密码/密钥/Token",
								Line:        position.Line,
								CodeSnippet: extractLine(code, position.Line),
								Suggestion:  "使用环境变量或配置文件存储敏感信息",
							})
							return
						}
					}
				}
			}
		}
	}
}

func extractLine(code string, line int) string {
	lines := strings.Split(code, "\n")
	if line-1 < len(lines) && line-1 >= 0 {
		snippet := strings.TrimSpace(lines[line-1])
		if len(snippet) > 100 {
			return snippet[:100] + "..."
		}
		return snippet
	}
	return ""
}

func generateSecuritySummary(issues []SecurityIssue) string {
	if len(issues) == 0 {
		return "✅ 未检测到安全问题"
	}
	return fmt.Sprintf("检测到 %d 个安全问题", len(issues))
}

func calculateSecurityStatistics(issues []SecurityIssue) SecurityStats {
	stats := SecurityStats{TotalIssues: len(issues)}
	for _, issue := range issues {
		switch issue.Severity {
		case "Critical":
			stats.Critical++
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

// BugResult Bug检测结果
type BugResult struct {
	Language      string     `json:"language"`
	Status        string     `json:"status"`
	TotalFiles    int        `json:"total_files"`
	AnalyzedFiles int        `json:"analyzed_files"`
	Total         int        `json:"total"`
	Bugs          []BugIssue `json:"bugs"`
	Summary       string     `json:"summary"`
	Statistics    BugStats   `json:"statistics"`
}

type BugIssue struct {
	ID            string `json:"id"`
	RuleID        string `json:"rule_id"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Line          int    `json:"line"`
	CodeSnippet   string `json:"code_snippet"`
	FixSuggestion string `json:"fix_suggestion"`
}

type BugStats struct {
	TotalIssues int `json:"total_issues"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
}

// AnalyzeBugs 执行Bug检测
func AnalyzeBugs(code string) (*BugResult, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析代码失败: %w", err)
	}

	var bugs []BugIssue

	// 检测忽略错误返回值
	ast.Inspect(node, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			checkIgnoredError(assign, fset, code, &bugs)
		}
		return true
	})

	return &BugResult{
		Language:      "go",
		Status:        "success",
		TotalFiles:    1,
		AnalyzedFiles: 1,
		Total:         len(bugs),
		Bugs:          bugs,
		Summary:       generateBugSummary(bugs),
		Statistics:    calculateBugStatistics(bugs),
	}, nil
}

func checkIgnoredError(assign *ast.AssignStmt, fset *token.FileSet, code string, bugs *[]BugIssue) {
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if ident.Name == "_" {
				if len(assign.Rhs) > 0 {
					if callExpr, ok := assign.Rhs[0].(*ast.CallExpr); ok {
						if isErrorReturningFunction(callExpr) {
							position := fset.Position(assign.Pos())
							*bugs = append(*bugs, BugIssue{
								ID:            fmt.Sprintf("bug-%d", position.Offset),
								RuleID:        "B101",
								Severity:      "High",
								Category:      "Error Handling",
								Description:   "忽略了错误返回值",
								Line:          position.Line,
								CodeSnippet:   extractLine(code, position.Line),
								FixSuggestion: "检查错误：if err != nil { return err }",
							})
						}
					}
				}
			}
		}
	}
}

func isErrorReturningFunction(callExpr *ast.CallExpr) bool {
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "os" || ident.Name == "http" || ident.Name == "ioutil" {
				return true
			}
		}
	}
	return false
}

func generateBugSummary(bugs []BugIssue) string {
	if len(bugs) == 0 {
		return "未检测到 Bug ✅"
	}
	return fmt.Sprintf("检测到 %d 个 Bug", len(bugs))
}

func calculateBugStatistics(bugs []BugIssue) BugStats {
	stats := BugStats{TotalIssues: len(bugs)}
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

// AnalysisResult 综合分析结果
type AnalysisResult struct {
	Complexity *ComplexityResult `json:"complexity"`
	Security   *SecurityResult   `json:"security"`
	Bugs       *BugResult        `json:"bugs"`
	AnalyzedAt string            `json:"analyzed_at"`
}

// PerformAnalysis 执行完整的代码分析
func PerformAnalysis(code string) (*AnalysisResult, error) {
	complexity, err := AnalyzeComplexity(code)
	if err != nil {
		return nil, err
	}

	security, err := AnalyzeSecurity(code)
	if err != nil {
		return nil, err
	}

	bugs, err := AnalyzeBugs(code)
	if err != nil {
		return nil, err
	}

	return &AnalysisResult{
		Complexity: complexity,
		Security:   security,
		Bugs:       bugs,
		AnalyzedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// ToJSON 将结果转为JSON字符串
func (r *AnalysisResult) ToJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}
