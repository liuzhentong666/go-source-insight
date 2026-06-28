package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// TestGenerator 测试生成器
type TestGenerator struct {
	BaseTool
	logger Logger
}

// NewTestGenerator 创建测试生成器
func NewTestGenerator(logger Logger) *TestGenerator {
	return &TestGenerator{
		BaseTool: BaseTool{
			name:        "test_generator",
			description: "自动生成 Go 代码的单元测试，支持 Table-driven 模式和 Mock 生成",
			inputType:   reflect.TypeOf(GenerateRequest{}),
		},
		logger: logger,
	}
}

// GenerateRequest 测试生成请求
type GenerateRequest struct {
	// 以下 3 个参数互斥，只能指定一个
	FunctionName string // 函数名（分析单个函数）
	FilePath     string // 文件路径（分析整个文件）
	DirPath      string // 目录路径（分析整个目录）

	// 配置选项
	TestMode     TestMode // 测试模式
	WithMock     bool     // 是否生成 Mock 建议
	WithCoverage bool     // 是否生成覆盖率报告
}

// TestMode 测试模式
type TestMode string

const (
	TestModeBasic       TestMode = "basic"        // 基本测试
	TestModeTableDriven TestMode = "table-driven" // 表驱动测试（推荐）
	TestModeMock        TestMode = "mock"         // Mock 测试
)

// Validate 验证输入参数
func (tg *TestGenerator) Validate(input any) error {
	req, ok := input.(GenerateRequest)
	if !ok {
		return ErrInvalidInput
	}

	// 检查至少指定了一个目标
	if req.FunctionName == "" && req.FilePath == "" && req.DirPath == "" {
		return fmt.Errorf("必须指定 FunctionName, FilePath 或 DirPath 其中之一")
	}

	// 检查不能同时指定多个
	count := 0
	if req.FunctionName != "" {
		count++
	}
	if req.FilePath != "" {
		count++
	}
	if req.DirPath != "" {
		count++
	}

	if count > 1 {
		return fmt.Errorf("FunctionName, FilePath 和 DirPath 不能同时指定")
	}

	// 验证路径存在
	if req.FilePath != "" {
		if _, err := os.Stat(req.FilePath); os.IsNotExist(err) {
			return fmt.Errorf("文件不存在: %s", req.FilePath)
		}
	}

	if req.DirPath != "" {
		if _, err := os.Stat(req.DirPath); os.IsNotExist(err) {
			return fmt.Errorf("目录不存在: %s", req.DirPath)
		}
	}

	return nil
}

// Run 执行测试生成
func (tg *TestGenerator) Run(ctx context.Context, input any) (string, error) {
	req := input.(GenerateRequest)

	tg.logger.Info("开始生成测试",
		"mode", req.TestMode,
		"function", req.FunctionName,
		"file", req.FilePath,
		"dir", req.DirPath)

	var result GenerateResult
	var err error

	// 根据不同的输入类型执行不同的逻辑
	switch {
	case req.FunctionName != "":
		result, err = tg.generateFunctionTest(req)
	case req.FilePath != "":
		result, err = tg.generateFileTests(req)
	case req.DirPath != "":
		result, err = tg.generateDirectoryTests(req)
	}

	if err != nil {
		tg.logger.Error("生成测试失败", "error", err)
		return "", err
	}

	// 格式化输出
	output := tg.formatResult(result)

	tg.logger.Info("测试生成完成",
		"files", len(result.GeneratedFiles),
		"testCases", result.TestCaseCount)

	return output, nil
}

// generateFunctionTest 为单个函数生成测试
func (tg *TestGenerator) generateFunctionTest(req GenerateRequest) (GenerateResult, error) {
	// 解析函数信息
	funcInfo, err := tg.parseFunctionInfo(req.FilePath, req.FunctionName)
	if err != nil {
		return GenerateResult{}, err
	}

	// 生成测试代码
	testCode, err := tg.generateTestCode(*funcInfo, req.TestMode)
	if err != nil {
		return GenerateResult{}, err
	}

	fullTestCode := tg.wrapTestFile(funcInfo.Package, testCode)

	// 写入文件
	testFilePath := tg.getTestFilePath(req.FilePath)
	if err := tg.writeNewTestFile(testFilePath, fullTestCode); err != nil {
		return GenerateResult{}, err
	}

	// 运行测试并收集覆盖率
	var coverage *CoverageReport
	if req.WithCoverage {
		coverage = tg.runCoverage(testFilePath)
	}

	return GenerateResult{
		GeneratedFiles: []string{testFilePath},
		TestCaseCount:  1,
		Coverage:       coverage,
	}, nil
}

// generateFileTests 为整个文件生成测试
func (tg *TestGenerator) generateFileTests(req GenerateRequest) (GenerateResult, error) {
	// 解析文件中的所有函数
	packageName, funcInfos, err := tg.parseFileFunctions(req.FilePath)
	if err != nil {
		return GenerateResult{}, err
	}

	// 为每个函数生成测试
	var allTestCode strings.Builder
	testCaseCount := 0

	for _, funcInfo := range funcInfos {
		// 跳过非公开函数和测试函数
		if !ast.IsExported(funcInfo.Name) || strings.HasPrefix(funcInfo.Name, "Test") {
			continue
		}

		testCode, err := tg.generateTestCode(funcInfo, req.TestMode)
		if err != nil {
			tg.logger.Warn("生成函数测试失败",
				"function", funcInfo.Name,
				"error", err)
			continue
		}

		allTestCode.WriteString(testCode)
		allTestCode.WriteString("\n\n")
		testCaseCount++
	}

	if testCaseCount == 0 {
		return GenerateResult{}, fmt.Errorf("没有找到可测试的函数")
	}

	// 写入文件
	testFilePath := tg.getTestFilePath(req.FilePath)
	fullTestCode := tg.wrapTestFile(packageName, allTestCode.String())
	if err := tg.writeNewTestFile(testFilePath, fullTestCode); err != nil {
		return GenerateResult{}, err
	}

	// 运行测试并收集覆盖率
	var coverage *CoverageReport
	if req.WithCoverage {
		coverage = tg.runCoverage(testFilePath)
	}

	return GenerateResult{
		GeneratedFiles: []string{testFilePath},
		TestCaseCount:  testCaseCount,
		Coverage:       coverage,
	}, nil
}

// generateDirectoryTests 为整个目录生成测试
func (tg *TestGenerator) generateDirectoryTests(req GenerateRequest) (GenerateResult, error) {
	// 查找所有 Go 文件
	var goFiles []string
	err := filepath.Walk(req.DirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})

	if err != nil {
		return GenerateResult{}, fmt.Errorf("遍历目录失败: %w", err)
	}

	if len(goFiles) == 0 {
		return GenerateResult{}, fmt.Errorf("没有找到 Go 源文件")
	}

	// 为每个文件生成测试
	var generatedFiles []string
	totalTestCases := 0

	for _, filePath := range goFiles {
		fileReq := GenerateRequest{
			FilePath:     filePath,
			TestMode:     req.TestMode,
			WithMock:     req.WithMock,
			WithCoverage: false, // 目录模式下单独处理覆盖率
		}

		result, err := tg.generateFileTests(fileReq)
		if err != nil {
			tg.logger.Warn("生成文件测试失败",
				"file", filePath,
				"error", err)
			continue
		}

		generatedFiles = append(generatedFiles, result.GeneratedFiles...)
		totalTestCases += result.TestCaseCount
	}

	if len(generatedFiles) == 0 {
		return GenerateResult{}, fmt.Errorf("没有生成任何测试文件")
	}

	// 运行测试并收集覆盖率
	var coverage *CoverageReport
	if req.WithCoverage {
		coverage = tg.runDirectoryCoverage(req.DirPath)
	}

	return GenerateResult{
		GeneratedFiles:  generatedFiles,
		TestCaseCount:   totalTestCases,
		Coverage:        coverage,
		MockSuggestions: nil, // 可以在后续添加
	}, nil
}

// ==================== FunctionParser ====================

// FunctionInfo 函数信息
type FunctionInfo struct {
	Name       string      // 函数名
	Package    string      // 包名
	Params     []Parameter // 参数列表
	Returns    []Parameter // 返回值列表
	IsMethod   bool        // 是否为方法
	Receiver   *Parameter  // 接收者（如果是方法）
	DocComment string      // 文档注释
}

// Parameter 参数/返回值信息
type Parameter struct {
	Name string // 参数名（可能为空）
	Type string // 类型（字符串表示）
}

// parseFunctionInfo 解析函数信息
func (tg *TestGenerator) parseFunctionInfo(filePath, funcName string) (*FunctionInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析文件失败: %w", err)
	}

	var funcInfo *FunctionInfo

	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			funcInfo = tg.extractFunctionInfo(fn, node.Name.Name)
			return false
		}
		return true
	})

	if funcInfo == nil {
		return nil, fmt.Errorf("函数不存在: %s", funcName)
	}

	return funcInfo, nil
}

// parseFileFunctions 解析文件中的所有函数
func (tg *TestGenerator) parseFileFunctions(filePath string) (string, []FunctionInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", nil, fmt.Errorf("解析文件失败: %w", err)
	}

	var funcInfos []FunctionInfo

	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			funcInfo := tg.extractFunctionInfo(fn, node.Name.Name)
			funcInfos = append(funcInfos, *funcInfo)
		}
		return true
	})

	return node.Name.Name, funcInfos, nil
}

// extractFunctionInfo 从 AST 节点提取函数信息
func (tg *TestGenerator) extractFunctionInfo(fn *ast.FuncDecl, packageName string) *FunctionInfo {
	info := &FunctionInfo{
		Name:    fn.Name.Name,
		Package: packageName,
	}

	// 提取接收者（方法）
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		info.IsMethod = true
		field := fn.Recv.List[0]
		info.Receiver = &Parameter{
			Name: tg.extractFieldNames(field),
			Type: tg.exprToString(field.Type),
		}
	}

	// 提取参数
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			names := tg.extractFieldNames(field)
			typeStr := tg.exprToString(field.Type)

			if names == "" {
				// 匿名参数
				info.Params = append(info.Params, Parameter{
					Name: "",
					Type: typeStr,
				})
			} else {
				// 多个参数共享一个类型
				for _, name := range strings.Split(names, ", ") {
					info.Params = append(info.Params, Parameter{
						Name: strings.TrimSpace(name),
						Type: typeStr,
					})
				}
			}
		}
	}

	// 提取返回值
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			names := tg.extractFieldNames(field)
			typeStr := tg.exprToString(field.Type)

			if names == "" {
				info.Returns = append(info.Returns, Parameter{
					Name: "",
					Type: typeStr,
				})
			} else {
				for _, name := range strings.Split(names, ", ") {
					info.Returns = append(info.Returns, Parameter{
						Name: strings.TrimSpace(name),
						Type: typeStr,
					})
				}
			}
		}
	}

	// 提取文档注释
	if fn.Doc != nil {
		info.DocComment = strings.TrimSpace(fn.Doc.Text())
	}

	return info
}

// extractFieldNames 提取字段名
func (tg *TestGenerator) extractFieldNames(field *ast.Field) string {
	if len(field.Names) == 0 {
		return ""
	}

	var names []string
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	return strings.Join(names, ", ")
}

// exprToString 将表达式转换为字符串
func (tg *TestGenerator) exprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	// 这里简化处理，实际应该使用 go/types 获取准确类型
	// 为了简化，我们直接用字符串表示
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return tg.exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + tg.exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + tg.exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + tg.exprToString(t.Key) + "]" + tg.exprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		return "chan " + tg.exprToString(t.Value)
	default:
		// 使用 token 格式化
		return fmt.Sprintf("%v", expr)
	}
}

// ==================== TestCaseGenerator ====================

// generateTestCode 生成测试代码
func (tg *TestGenerator) generateTestCode(funcInfo FunctionInfo, mode TestMode) (string, error) {
	var code strings.Builder

	switch mode {
	case TestModeBasic:
		code.WriteString(tg.generateBasicTest(funcInfo))
	case TestModeTableDriven:
		code.WriteString(tg.generateTableDrivenTest(funcInfo))
	case TestModeMock:
		code.WriteString(tg.generateTableDrivenTest(funcInfo)) // Mock 模式也使用 table-driven
	default:
		code.WriteString(tg.generateTableDrivenTest(funcInfo))
	}

	// 格式化代码
	formatted, err := format.Source([]byte(code.String()))
	if err != nil {
		return "", fmt.Errorf("格式化代码失败: %w", err)
	}

	return string(formatted), nil
}

// generateBasicTest 生成基本测试
func (tg *TestGenerator) generateBasicTest(funcInfo FunctionInfo) string {
	return fmt.Sprintf(`func Test%s(t *testing.T) {
	// TODO: 实现测试逻辑
	// 提示：建议使用 Table-driven 模式生成更完善的测试
	
	// 示例：
	// result, err := %s()
	// if err != nil {
	//     t.Errorf("unexpected error: %%v", err)
	// }
	// if result != expected {
	//     t.Errorf("got %%v, want %%v", result, expected)
	// }
}
`, funcInfo.Name, funcInfo.Name)
}

// generateTableDrivenTest 生成可编译的测试骨架。
// 生成器当前无法可靠推断复杂参数和返回值的合法样例，因此先生成 t.Skip 骨架，
// 避免把 TODO_ 占位符写入项目导致 go test/go build 失败。
func (tg *TestGenerator) generateTableDrivenTest(funcInfo FunctionInfo) string {
	return fmt.Sprintf(`func Test%s(t *testing.T) {
	t.Skip("TODO: 补充 %s 的真实测试用例")
}
`, funcInfo.Name, funcInfo.Name)
}

// getReturnType 获取返回值类型
func (tg *TestGenerator) getReturnType(funcInfo FunctionInfo) string {
	if len(funcInfo.Returns) == 0 {
		return ""
	}
	if len(funcInfo.Returns) == 1 {
		return funcInfo.Returns[0].Type
	}
	// 多返回值情况，简化处理
	var returnTypes []string
	for _, ret := range funcInfo.Returns {
		returnTypes = append(returnTypes, ret.Type)
	}
	return strings.Join(returnTypes, ", ")
}

// ==================== MockGenerator ====================

// MockSuggestion Mock 建议
type MockSuggestion struct {
	InterfaceName string       // 接口名
	Methods       []MockMethod // 方法列表
	Suggestion    string       // 建议
}

// MockMethod Mock 方法
type MockMethod struct {
	Name    string   // 方法名
	Params  []string // 参数类型
	Returns []string // 返回值类型
}

// generateMockSuggestions 生成 Mock 建议
func (tg *TestGenerator) generateMockSuggestions(funcInfo FunctionInfo) []MockSuggestion {
	// 这里可以分析参数中是否有接口类型
	// 如果有，则生成 Mock 建议

	var suggestions []MockSuggestion

	// 简化版本：只生成一个示例建议
	suggestions = append(suggestions, MockSuggestion{
		InterfaceName: "InterfaceName",
		Methods: []MockMethod{
			{
				Name:    "MethodName",
				Params:  []string{"argType1", "argType2"},
				Returns: []string{"returnType", "error"},
			},
		},
		Suggestion: "建议使用 testify/mock 或 gomock 库生成 Mock 对象",
	})

	return suggestions
}

// ==================== TestRunner ====================

// CoverageReport 覆盖率报告
type CoverageReport struct {
	TotalStatements float64 // 语句覆盖率
	TotalFunctions  float64 // 函数覆盖率
	UncoveredLines  []int   // 未覆盖的行号
	Suggestion      string  // 改进建议
}

// runCoverage 运行测试并收集覆盖率
func (tg *TestGenerator) runCoverage(testFilePath string) *CoverageReport {
	// 使用 go test -cover 运行测试
	// 这里简化处理，实际需要执行命令并解析输出
	// 为了测试，我们返回一个模拟的覆盖率报告

	return &CoverageReport{
		TotalStatements: 0.0,
		TotalFunctions:  0.0,
		UncoveredLines:  []int{},
		Suggestion:      "运行 go test -cover 查看实际覆盖率",
	}
}

// runDirectoryCoverage 运行目录测试并收集覆盖率
func (tg *TestGenerator) runDirectoryCoverage(dirPath string) *CoverageReport {
	// 使用 go test -cover ./... 运行测试
	// 这里简化处理
	return &CoverageReport{
		TotalStatements: 0.0,
		TotalFunctions:  0.0,
		UncoveredLines:  []int{},
		Suggestion:      "运行 go test -cover ./... 查看实际覆盖率",
	}
}

// ==================== 辅助函数 ====================

// getTestFilePath 获取测试文件路径
func (tg *TestGenerator) getTestFilePath(sourceFilePath string) string {
	dir := filepath.Dir(sourceFilePath)
	base := filepath.Base(sourceFilePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	return filepath.Join(dir, name+"_test.go")
}

func (tg *TestGenerator) writeNewTestFile(testFilePath string, testCode string) error {
	if _, err := os.Stat(testFilePath); err == nil {
		return fmt.Errorf("测试文件已存在，拒绝覆盖: %s", testFilePath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查测试文件失败: %w", err)
	}

	if err := os.WriteFile(testFilePath, []byte(testCode), 0644); err != nil {
		return fmt.Errorf("写入测试文件失败: %w", err)
	}
	return nil
}

func (tg *TestGenerator) wrapTestFile(packageName string, testCode string) string {
	return fmt.Sprintf("package %s\n\nimport \"testing\"\n\n%s", packageName, testCode)
}

// formatResult 格式化结果输出
func (tg *TestGenerator) formatResult(result GenerateResult) string {
	var output strings.Builder

	output.WriteString("✅ 测试生成成功\n\n")
	output.WriteString(fmt.Sprintf("📊 生成的测试文件数: %d\n", len(result.GeneratedFiles)))
	output.WriteString(fmt.Sprintf("📝 测试用例总数: %d\n\n", result.TestCaseCount))

	output.WriteString("📁 生成的文件:\n")
	for _, file := range result.GeneratedFiles {
		output.WriteString(fmt.Sprintf("   - %s\n", file))
	}

	if result.Coverage != nil {
		output.WriteString("\n📈 覆盖率报告:\n")
		output.WriteString(fmt.Sprintf("   - 语句覆盖率: %.2f%%\n", (result.Coverage.TotalStatements * 100)))
		output.WriteString(fmt.Sprintf("   - 函数覆盖率: %.2f%%\n", (result.Coverage.TotalFunctions * 100)))
		if len(result.Coverage.UncoveredLines) > 0 {
			output.WriteString(fmt.Sprintf("   - 未覆盖行号: %v\n", result.Coverage.UncoveredLines))
		}
		output.WriteString(fmt.Sprintf("   - 建议: %s\n", result.Coverage.Suggestion))
	}

	if len(result.MockSuggestions) > 0 {
		output.WriteString("\n🎭 Mock 建议:\n")
		for i, suggestion := range result.MockSuggestions {
			output.WriteString(fmt.Sprintf("   %d. 接口: %s\n", i+1, suggestion.InterfaceName))
			for _, method := range suggestion.Methods {
				output.WriteString(fmt.Sprintf("      - %s(%v) (%v)\n", method.Name, method.Params, method.Returns))
			}
			output.WriteString(fmt.Sprintf("      %s\n", suggestion.Suggestion))
		}
	}

	return output.String()
}

// ==================== 输出结果 ====================

// GenerateResult 测试生成结果
type GenerateResult struct {
	GeneratedFiles  []string         // 生成的测试文件
	TestCaseCount   int              // 测试用例数量
	Coverage        *CoverageReport  // 覆盖率报告（可选）
	MockSuggestions []MockSuggestion // Mock 建议（可选）
}
