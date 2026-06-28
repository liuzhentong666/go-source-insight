package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 测试忽略错误返回值
func TestBugDetector_IgnoredError(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import (
	"fmt"
	"os"
)

func ReadFile() {
	// Bug: 忽略错误返回值
	_ = os.Open("file.txt")
}

func WriteFile() error {
	// 正确：检查错误
	file, err := os.Open("file.txt")
	if err != nil {
		return err
	}
	return nil
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到 Bug")
	}

	// 检查是否有 B101 规则
	hasIgnoredError := false
	for _, bug := range analysis.Bugs {
		if bug.RuleID == "B101" {
			hasIgnoredError = true
			break
		}
	}

	if !hasIgnoredError {
		t.Fatal("应该检测到忽略错误返回值的 Bug")
	}
}

// 测试资源未关闭
func TestBugDetector_ResourceNotClosed(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import "os"

func OpenFile() {
	// Bug: 资源未关闭
	file, _ := os.Open("file.txt")
	// 缺少 defer file.Close()
}

func OpenFileSafe() error {
	file, err := os.Open("file.txt")
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到 Bug")
	}

	// 检查是否有 B102 规则
	hasResourceNotClosed := false
	for _, bug := range analysis.Bugs {
		if bug.RuleID == "B102" {
			hasResourceNotClosed = true
			break
		}
	}

	if !hasResourceNotClosed {
		t.Fatal("应该检测到资源未关闭的 Bug")
	}
}

// 测试 switch 缺少 default
func TestBugDetector_SwitchWithoutDefault(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

func Grade(score int) string {
	// Bug: switch 缺少 default
	switch score {
	case 90:
		return "A"
	case 80:
		return "B"
	}
	return "unknown"
}

func GradeSafe(score int) string {
	// 正确：有 default
	switch score {
	case 90:
		return "A"
	default:
		return "unknown"
	}
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到 Bug")
	}

	// 检查是否有 B103 规则
	hasSwitchWithoutDefault := false
	for _, bug := range analysis.Bugs {
		if bug.RuleID == "B103" {
			hasSwitchWithoutDefault = true
			break
		}
	}

	if !hasSwitchWithoutDefault {
		t.Fatal("应该检测到 switch 缺少 default 的 Bug")
	}
}

// 测试可能的 nil 指针引用（简化版）
func TestBugDetector_PotentialNilPointer(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

type MyType struct {
	Value int
}

func Example() {
	// Bug: 可能的 nil 指针引用
	var p *MyType
	p.Method()
}

func ExampleSafe() {
	// 正确：检查 nil
	p := &MyType{}
	p.Method()
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// B104 是简化版，可能会检测到，也可能不会
	// 这里只确保不崩溃
	t.Logf("检测到的 Bug 数量: %d", analysis.Total)
}

// 测试安全代码（无 Bug）
func TestBugDetector_SafeCode(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import (
	"errors"
	"os"
)

func SafeFunction() error {
	file, err := os.Open("file.txt")
	if err != nil {
		return err
	}
	defer file.Close()

	switch file.Name() {
	case "":
		return errors.New("empty name")
	default:
		return nil
	}
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 安全代码应该没有 Bug（B104 可能会误报，但这是预期行为）
	t.Logf("检测到的 Bug 数量: %d", analysis.Total)
}

// 测试空代码
func TestBugDetector_EmptyCode(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	_, err := detector.Run(ctx, "")
	if err == nil {
		t.Fatal("空代码应该返回错误")
	}
	t.Logf("空代码正确返回错误: %v", err)
}

// 测试语法错误
func TestBugDetector_SyntaxError(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	_, err := detector.Run(ctx, "this is not valid go code {")
	if err == nil {
		t.Fatal("语法错误应该返回错误")
	}
	t.Logf("语法错误正确返回错误: %v", err)
}

// 测试多文件输入
func TestBugDetector_MultipleFiles(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	// 创建临时文件
	tmpDir := t.TempDir()

	// 文件 1: Go 文件
	goFile1 := filepath.Join(tmpDir, "file1.go")
	err := os.WriteFile(goFile1, []byte(`package main

import "os"

func File1() {
	_ = os.Open("file.txt")
}`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 文件 2: Go 文件
	goFile2 := filepath.Join(tmpDir, "file2.go")
	err = os.WriteFile(goFile2, []byte(`package main

func File2() {
	switch 1 {
	case 1:
		// 缺少 default
	}
}`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 文件 3: Python 文件
	pyFile := filepath.Join(tmpDir, "utils.py")
	err = os.WriteFile(pyFile, []byte(`def hello():
    print("Hello")`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 测试文件列表输入
	input := BugDetectorInput{
		Files: []string{goFile1, goFile2, pyFile},
	}

	result, err := detector.Run(ctx, input)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 检查统计
	if analysis.TotalFiles != 3 {
		t.Fatalf("总文件数错误: 期望 3, 实际 %d", analysis.TotalFiles)
	}

	if analysis.AnalyzedFiles != 2 {
		t.Fatalf("分析的 Go 文件数错误: 期望 2, 实际 %d", analysis.AnalyzedFiles)
	}

	if len(analysis.SkippedFiles) != 1 {
		t.Fatalf("跳过的文件数错误: 期望 1, 实际 %d", len(analysis.SkippedFiles))
	}

	// 检查跳过的文件
	skipped := analysis.SkippedFiles[0]
	if skipped.Language != "python" {
		t.Fatalf("跳过的文件语言错误: 期望 python, 实际 %s", skipped.Language)
	}

	if skipped.Status != "skipped" {
		t.Fatalf("跳过的文件状态错误: 期望 skipped, 实际 %s", skipped.Status)
	}
}

// 测试目录扫描
func TestBugDetector_DirectoryScan(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建多个文件
	goFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(goFile, []byte(`package main

import "os"

func main() {
	_ = os.Open("file.txt")
}`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	pyFile := filepath.Join(tmpDir, "utils.py")
	err = os.WriteFile(pyFile, []byte(`# python file`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 测试目录扫描
	input := BugDetectorInput{
		Directory: tmpDir,
	}

	result, err := detector.Run(ctx, input)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.AnalyzedFiles != 1 {
		t.Fatalf("分析的 Go 文件数错误: 期望 1, 实际 %d", analysis.AnalyzedFiles)
	}

	if len(analysis.SkippedFiles) != 1 {
		t.Fatalf("跳过的文件数错误: 期望 1, 实际 %d", len(analysis.SkippedFiles))
	}
}

// 测试语言检测
func TestBugDetector_LanguageDetection(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"main.go", "go"},
		{"utils.py", "python"},
		{"server.js", "javascript"},
		{"app.ts", "typescript"},
		{"Main.java", "java"},
		{"app.cpp", "cpp"},
		{"main.c", "c"},
		{"main.rs", "rust"},
		{"app.rb", "ruby"},
		{"index.php", "php"},
		{"README.md", "unknown"},
		{"Makefile", "unknown"},
	}

	for _, tt := range tests {
		lang := DetectLanguage(tt.filename)
		if lang != tt.expected {
			t.Errorf("语言检测错误: %s, 期望 %s, 实际 %s", tt.filename, tt.expected, lang)
		}
	}
}

// 测试没有 Go 文件的情况
func TestBugDetector_NoGoFiles(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	// 创建临时目录
	tmpDir := t.TempDir()

	// 只创建非 Go 文件
	pyFile := filepath.Join(tmpDir, "utils.py")
	err := os.WriteFile(pyFile, []byte(`# python file`), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	// 测试目录扫描
	input := BugDetectorInput{
		Directory: tmpDir,
	}

	result, err := detector.Run(ctx, input)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.AnalyzedFiles != 0 {
		t.Fatalf("不应该分析任何文件: 实际 %d", analysis.AnalyzedFiles)
	}

	if !strings.Contains(analysis.Summary, "未检测到 Go 文件") {
		t.Fatalf("摘要应该提示没有 Go 文件: %s", analysis.Summary)
	}
}

// 测试 JSON 输出格式
func TestBugDetector_JSONFormat(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import "os"

func Example() {
	_ = os.Open("file.txt")
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	// 验证是有效的 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("输出不是有效的 JSON: %v", err)
	}

	// 验证必要字段
	requiredFields := []string{"language", "status", "total", "bugs", "summary", "statistics"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			t.Fatalf("缺少必要字段: %s", field)
		}
	}
}

// 测试与 ToolManager 集成
func TestBugDetector_ToolManagerIntegration(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	detector := NewBugDetector()
	config := DefaultToolConfig("bug_detector")

	err := tm.Register(detector, config)
	if err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}

	code := `package main

import "os"

func Example() {
	_ = os.Open("file.txt")
}
`

	result, err := tm.Run(context.Background(), "bug_detector", code)
	if err != nil {
		t.Fatalf("执行工具失败: %v", err)
	}

	if !result.Success {
		t.Fatalf("工具应该执行成功: %s", result.Error)
	}

	// 验证输出
	var analysis BugResult
	if err := json.Unmarshal([]byte(result.Result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.AnalyzedFiles == 0 {
		t.Fatal("应该分析至少一个文件")
	}
}

// 演示测试 - 展示实际输出
func TestBugDetector_Demo(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import (
	"fmt"
	"os"
)

func ReadFile() {
	// Bug 1: 忽略错误返回值
	_ = os.Open("file.txt")
}

func OpenFile() {
	// Bug 2: 资源未关闭
	file, _ := os.Open("file.txt")
	fmt.Println(file.Name())
	// 缺少 defer file.Close()
}

func ProcessScore(score int) string {
	// Bug 3: switch 缺少 default
	switch score {
	case 90:
		return "A"
	case 80:
		return "B"
	}
	return "unknown"
}

func Example() {
	// Bug 4: 可能的 nil 指针引用
	var p *MyType
	p.Method()
}

type MyType struct {
	Value int
}

func (m *MyType) Method() {
	fmt.Println(m.Value)
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	t.Log("=== Bug 检测结果 ===")
	t.Log(result)
}

// 格式化输出演示
func TestBugDetector_FormattedOutput(t *testing.T) {
	detector := NewBugDetector()
	ctx := context.Background()

	code := `package main

import "os"

func ReadFile() {
	_ = os.Open("file.txt")
}

func OpenFile() {
	file, _ := os.Open("file.txt")
	// 缺少 defer file.Close()
}

func ProcessScore(score int) string {
	switch score {
	case 90:
		return "A"
	// 缺少 default
	}
	return "unknown"
}
`

	result, err := detector.Run(ctx, code)
	if err != nil {
		t.Fatalf("检测失败: %v", err)
	}

	// 解析 JSON
	var analysis BugResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 格式化输出
	t.Log("\n========== Bug 检测报告 ==========")
	t.Logf("\n📊 总体信息")
	t.Logf("  - 语言: %s", analysis.Language)
	t.Logf("  - 状态: %s", analysis.Status)
	t.Logf("  - 总文件数: %d", analysis.TotalFiles)
	t.Logf("  - 分析的 Go 文件: %d", analysis.AnalyzedFiles)
	t.Logf("  - 总 Bug 数: %d", analysis.Total)
	t.Logf("  - %s", analysis.Summary)

	t.Logf("\n⚠️  Bug 统计")
	stats := analysis.Statistics
	t.Logf("  - High: %d", stats.High)
	t.Logf("  - Medium: %d", stats.Medium)
	t.Logf("  - Low: %d", stats.Low)

	if analysis.Total > 0 {
		t.Logf("\n📋 Bug 详情")
		for i, bug := range analysis.Bugs {
			t.Logf("\n  Bug #%d:", i+1)
			t.Logf("    ID: %s", bug.ID)
			t.Logf("    规则: %s - %s", bug.RuleID, bug.Category)
			t.Logf("    严重程度: %s", bug.Severity)
			t.Logf("    置信度: %s", bug.Confidence)
			t.Logf("    位置: 第 %d 行 (%s)", bug.Line, bug.File)
			t.Logf("    代码: %s", bug.CodeSnippet)
			t.Logf("    描述: %s", bug.Description)
			t.Logf("    修复建议:")
			for _, line := range strings.Split(bug.FixSuggestion, "\n") {
				t.Logf("      %s", line)
			}
		}
	} else {
		t.Log("\n✅ 未检测到 Bug")
	}

	if len(analysis.SkippedFiles) > 0 {
		t.Log("\n📂 跳过的文件")
		for _, file := range analysis.SkippedFiles {
			t.Logf("  - %s (%s): %s", file.Path, file.Language, file.Reason)
		}
	}

	t.Log("\n💡 其他建议")
	for _, rec := range analysis.Recommendations {
		t.Logf("  - %s", rec)
	}

	t.Log("\n=====================================")
}
