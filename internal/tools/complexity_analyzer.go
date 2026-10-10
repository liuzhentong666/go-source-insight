package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// ComplexityAnalyzer 代码复杂度分析器
// 分析 Go 代码的圈复杂度，识别过于复杂的函数
type ComplexityAnalyzer struct {
	*BaseTool
}

// NewComplexityAnalyzer 创建复杂度分析器
func NewComplexityAnalyzer() *ComplexityAnalyzer {
	return &ComplexityAnalyzer{
		BaseTool: NewBaseTool(
			"complexity_analyzer",
			"分析 Go 代码的圈复杂度，识别过于复杂的函数（圈复杂度 > 10）",
			reflect.TypeOf(""),
		),
	}
}

// Run 执行复杂度分析
func (ca *ComplexityAnalyzer) Run(ctx context.Context, input any) (string, error) {
	// 类型断言
	code, ok := input.(string)
	if !ok {
		return "", fmt.Errorf("输入类型错误: 期望 string, 实际 %T", input)
	}

	// 创建文件集
	fset := token.NewFileSet()

	// 解析 Go 代码
	node, err := parser.ParseFile(fset, "", code, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("解析 Go 代码失败: %w", err)
	}

	// 收集所有函数
	var functions []*ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			functions = append(functions, fn)
		}
		return true
	})

	// 分析每个函数
	var functionResults []FunctionResult
	totalComplexity := 0

	for _, fn := range functions {
		// 计算复杂度
		complexity := calculateComplexity(fn)

		// 计算行数
		line := fset.PositionFor(fn.Pos(), false).Line
		lines := calculateLines(fset, fn)

		// 生成问题列表
		issues := generateIssues(complexity, lines)

		result := FunctionResult{
			Name:       fn.Name.Name,
			Line:       line,
			Complexity: complexity,
			Lines:      lines,
			Issues:     issues,
		}

		functionResults = append(functionResults, result)
		totalComplexity += complexity
	}

	// 构建结果
	result := ComplexityResult{
		File:       "",
		Total:      totalComplexity,
		Functions:  functionResults,
		Summary:    generateSummary(functionResults),
		Statistics: calculateStatistics(functionResults),
	}

	// 序列化为 JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(jsonBytes), nil
}

// FunctionResult 单个函数的分析结果
type FunctionResult struct {
	Name       string   `json:"name"`       // 函数名
	Line       int      `json:"line"`       // 起始行号
	Complexity int      `json:"complexity"` // 圈复杂度
	Lines      int      `json:"lines"`      // 函数行数
	Issues     []string `json:"issues"`     // 问题列表
}

// ComplexityResult 完整的分析结果
type ComplexityResult struct {
	File       string           `json:"file"`       // 文件名（如果提供）
	Total      int              `json:"total"`      // 总复杂度
	Functions  []FunctionResult `json:"functions"`  // 所有函数
	Summary    string           `json:"summary"`    // 摘要
	Statistics Statistics       `json:"statistics"` // 统计信息
}

// Statistics 统计信息
type Statistics struct {
	TotalFunctions       int `json:"total_functions"`        // 总函数数
	SimpleFunctions      int `json:"simple_functions"`       // 简单函数（1-10）
	MediumFunctions      int `json:"medium_functions"`       // 中等函数（11-20）
	ComplexFunctions     int `json:"complex_functions"`      // 复杂函数（21-50）
	VeryComplexFunctions int `json:"very_complex_functions"` // 非常复杂函数（>50）
}

// calculateComplexity 计算函数的圈复杂度
// 公式: 圈复杂度 = 1 (基础路径) + 判定点数量
func calculateComplexity(fn *ast.FuncDecl) int {
	count := 1 // 基础复杂度

	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		// if 语句
		case *ast.IfStmt:
			count++

		// for 循环
		case *ast.ForStmt:
			count++

		// range 循环
		case *ast.RangeStmt:
			count++

		// switch 语句
		case *ast.SwitchStmt:
			count++

		// case 分支
		case *ast.CaseClause:
			// 跳过 switch 的默认 case（它不是独立的判定点）
			if node.List != nil {
				count++
			}

		// type switch
		case *ast.TypeSwitchStmt:
			count++

		// select 语句
		case *ast.SelectStmt:
			count++

		// select case
		case *ast.CommClause:
			count++

		// 逻辑运算符 && 和 ||
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				count++
			}
		}
		return true
	})

	return count
}

// calculateLines 计算函数的代码行数
func calculateLines(fset *token.FileSet, fn *ast.FuncDecl) int {
	start := fset.PositionFor(fn.Pos(), false).Line
	end := fset.PositionFor(fn.End(), false).Line
	return end - start + 1
}

// generateIssues 根据复杂度和行数生成问题列表
func generateIssues(complexity, lines int) []string {
	var issues []string

	// 复杂度检查
	if complexity > 50 {
		issues = append(issues, "🚨 圈复杂度过高（>50），必须拆分函数！")
	} else if complexity > 20 {
		issues = append(issues, "❌ 圈复杂度较高（>20），建议拆分函数")
	} else if complexity > 10 {
		issues = append(issues, "⚠️ 圈复杂度偏高（>10），可能需要重构")
	}

	// 行数检查（辅助指标）
	if lines > 100 {
		issues = append(issues, "📏 函数过长（>100行），建议拆分")
	} else if lines > 50 {
		issues = append(issues, "📏 函数较长（>50行），可考虑拆分")
	}

	// 复杂度/行数比检查（密度过高）
	if lines > 0 {
		density := float64(complexity) / float64(lines)
		if density > 0.5 && lines > 20 {
			issues = append(issues, "📊 复杂度密度过高，逻辑过于密集")
		}
	}

	return issues
}

// generateSummary 生成摘要信息
func generateSummary(results []FunctionResult) string {
	if len(results) == 0 {
		return "未找到任何函数"
	}

	// 计算平均复杂度
	total := 0
	for _, r := range results {
		total += r.Complexity
	}
	avg := float64(total) / float64(len(results))

	// 统计问题函数
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

// calculateStatistics 计算统计信息
func calculateStatistics(results []FunctionResult) Statistics {
	stats := Statistics{
		TotalFunctions: len(results),
	}

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
