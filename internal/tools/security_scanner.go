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

// SecurityScanner 安全扫描器
// 检测 Go 代码中的安全漏洞和风险（纯检测，不自动修复）
type SecurityScanner struct {
	*BaseTool
	ruleEngine *RuleEngine
}

// NewSecurityScanner 创建安全扫描器
func NewSecurityScanner() *SecurityScanner {
	scanner := &SecurityScanner{
		BaseTool: NewBaseTool(
			"security_scanner",
			"检测 Go 代码中的安全漏洞和风险（硬编码密钥、SQL 注入、不安全随机数等）",
			reflect.TypeOf(""),
		),
	}
	scanner.ruleEngine = NewRuleEngine()
	scanner.ruleEngine.RegisterAllRules()
	return scanner
}

// Run 执行安全扫描
func (ss *SecurityScanner) Run(ctx context.Context, input any) (string, error) {
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

	// 扫描安全问题
	var issues []SecurityIssue
	ruleCtx := &RuleContext{FSet: fset}

	ast.Inspect(node, func(n ast.Node) bool {
		// 跳过 nil 节点
		if n == nil {
			return false
		}

		// 应用所有规则
		for _, rule := range ss.ruleEngine.Rules {
			if rule.Match(n, ruleCtx) {
				issue := buildSecurityIssue(rule, n, fset, code)
				issues = append(issues, issue)
			}
		}
		return true
	})

	// 去重（同一位置可能被多个规则匹配）
	issues = deduplicateIssues(issues)

	// 构建结果
	result := SecurityResult{
		File:       "",
		Total:      len(issues),
		Issues:     issues,
		Summary:    generateSecuritySummary(issues),
		Statistics: calculateSecurityStatistics(issues),
	}

	// 序列化为 JSON
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}

	return string(jsonBytes), nil
}

// SecurityIssue 单个安全问题
type SecurityIssue struct {
	ID          string `json:"id"`           // 问题唯一标识
	RuleID      string `json:"rule_id"`      // 规则ID
	Severity    string `json:"severity"`     // 严重程度：Critical, High, Medium, Low
	Category    string `json:"category"`     // 问题类别
	Description string `json:"description"`  // 问题描述
	File        string `json:"file"`         // 文件名
	Line        int    `json:"line"`         // 行号
	Function    string `json:"function"`     // 所在函数
	CodeSnippet string `json:"code_snippet"` // 代码片段
	Suggestion  string `json:"suggestion"`   // 修复建议
}

// SecurityResult 完整的安全扫描结果
type SecurityResult struct {
	File       string          `json:"file"`       // 文件名
	Total      int             `json:"total"`      // 总问题数
	Issues     []SecurityIssue `json:"issues"`     // 所有问题
	Summary    string          `json:"summary"`    // 摘要
	Statistics SecurityStats   `json:"statistics"` // 统计信息
}

// SecurityStats 安全统计
type SecurityStats struct {
	TotalIssues int `json:"total_issues"` // 总问题数
	Critical    int `json:"critical"`      // 严重问题
	High        int `json:"high"`          // 高危问题
	Medium      int `json:"medium"`        // 中危问题
	Low         int `json:"low"`           // 低危问题
}

// RuleContext 规则检测上下文
type RuleContext struct {
	FSet      *token.FileSet
	CurrentFunc *ast.FuncDecl
}

// RuleEngine 规则引擎
type RuleEngine struct {
	Rules []SecurityRule
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		Rules: make([]SecurityRule, 0),
	}
}

// Register 注册规则
func (re *RuleEngine) Register(rule SecurityRule) {
	re.Rules = append(re.Rules, rule)
}

// RegisterAllRules 注册所有默认规则
func (re *RuleEngine) RegisterAllRules() {
	re.Register(&HardCodedSecretRule{})
	re.Register(&SQLInjectionRule{})
	re.Register(&WeakRandomRule{})
	re.Register(&InfoDisclosureRule{})
	re.Register(&WeakEncryptionRule{})
	re.Register(&InsecureFilePermRule{})
	re.Register(&InsecureHTTPRule{})
}

// SecurityRule 安全规则接口
type SecurityRule interface {
	ID() string                     // 规则唯一标识
	Name() string                   // 规则名称
	Category() string               // 规则类别
	Severity() string               // 严重程度
	Description() string            // 规则描述
	Suggestion() string             // 修复建议
	Match(node ast.Node, ctx *RuleContext) bool
}

// 规则 1: 硬编码密钥检测
type HardCodedSecretRule struct{}

func (r *HardCodedSecretRule) ID() string             { return "G101" }
func (r *HardCodedSecretRule) Name() string           { return "Hardcoded Secrets" }
func (r *HardCodedSecretRule) Category() string       { return "Credentials" }
func (r *HardCodedSecretRule) Severity() string       { return "Critical" }
func (r *HardCodedSecretRule) Description() string    { return "检测到硬编码的密码/密钥/Token" }
func (r *HardCodedSecretRule) Suggestion() string     { return "使用环境变量或配置文件存储敏感信息（如 os.Getenv、viper）" }

var secretKeywords = []string{
	"password", "passwd", "secret", "api_key", "apikey",
	"access_token", "accesstoken", "private_key", "privatekey",
	"auth_token", "authtoken", "token", "credential",
}

func (r *HardCodedSecretRule) Match(node ast.Node, ctx *RuleContext) bool {
	if assign, ok := node.(*ast.AssignStmt); ok {
		// 检查赋值语句
		for _, lhs := range assign.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok {
				// 检查变量名是否包含敏感关键字
				varName := strings.ToLower(ident.Name)
				for _, keyword := range secretKeywords {
					if strings.Contains(varName, keyword) {
						// 检查右侧是否是字符串字面量
						if len(assign.Rhs) > 0 {
							if _, ok := assign.Rhs[0].(*ast.BasicLit); ok {
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

// 规则 2: SQL 注入检测
type SQLInjectionRule struct{}

func (r *SQLInjectionRule) ID() string          { return "G201" }
func (r *SQLInjectionRule) Name() string        { return "SQL Injection" }
func (r *SQLInjectionRule) Category() string    { return "Injection" }
func (r *SQLInjectionRule) Severity() string    { return "Critical" }
func (r *SQLInjectionRule) Description() string { return "SQL 注入风险：使用字符串拼接构造 SQL 语句" }
func (r *SQLInjectionRule) Suggestion() string  { return "使用参数化查询（Prepared Statement）或 ORM" }

var sqlKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "FROM", "WHERE",
	"DROP", "CREATE", "ALTER", "TRUNCATE", "EXEC", "EXECUTE",
}

func (r *SQLInjectionRule) Match(node ast.Node, ctx *RuleContext) bool {
	// 检测字符串拼接
	if binExpr, ok := node.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.ADD {
			// 检查左右是否包含字符串和变量
			hasStringLiteral := isStringLiteral(binExpr.X) || isStringLiteral(binExpr.Y)
			hasVariable := !isStringLiteral(binExpr.X) || !isStringLiteral(binExpr.Y)

			if hasStringLiteral && hasVariable {
				// 检查是否包含 SQL 关键字
				str := extractStringLiteral(binExpr.X) + extractStringLiteral(binExpr.Y)
				for _, keyword := range sqlKeywords {
					if strings.Contains(strings.ToUpper(str), keyword) {
						return true
					}
				}
			}
		}
	}
	return false
}

// 规则 3: 不安全随机数检测
type WeakRandomRule struct{}

func (r *WeakRandomRule) ID() string          { return "G401" }
func (r *WeakRandomRule) Name() string        { return "Use of Weak Random Number Generator" }
func (r *WeakRandomRule) Category() string    { return "Cryptography" }
func (r *WeakRandomRule) Severity() string    { return "High" }
func (r *WeakRandomRule) Description() string { return "使用不安全的随机数生成器（math/rand）" }
func (r *WeakRandomRule) Suggestion() string  { return "使用 crypto/rand 代替 math/rand 用于密码学场景" }

func (r *WeakRandomRule) Match(node ast.Node, ctx *RuleContext) bool {
	if selExpr, ok := node.(*ast.SelectorExpr); ok {
		// 检测 math/rand 调用
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "rand" || ident.Name == "math/rand" {
				// 检测 rand 的常用函数
				funcName := selExpr.Sel.Name
				weakFuncs := []string{"Int", "Intn", "Int31", "Int31n", "Int63", "Int63n", "Float32", "Float64", "Perm", "Shuffle"}
				for _, f := range weakFuncs {
					if funcName == f {
						return true
					}
				}
			}
		}
	}
	return false
}

// 规则 4: 敏感信息打印检测
type InfoDisclosureRule struct{}

func (r *InfoDisclosureRule) ID() string          { return "G104" }
func (r *InfoDisclosureRule) Name() string        { return "Information Disclosure" }
func (r *InfoDisclosureRule) Category() string    { return "Data Privacy" }
func (r *InfoDisclosureRule) Severity() string    { return "Medium" }
func (r *InfoDisclosureRule) Description() string { return "敏感信息打印到日志/控制台" }
func (r *InfoDisclosureRule) Suggestion() string  { return "避免打印密码、Token、个人隐私信息到日志" }

var sensitiveKeywords = []string{
	"password", "passwd", "secret", "token", "api_key",
	"private_key", "access_key", "credential", "auth",
	"ssn", "credit_card", "pin", "key",
}

func (r *InfoDisclosureRule) Match(node ast.Node, ctx *RuleContext) bool {
	if callExpr, ok := node.(*ast.CallExpr); ok {
		// 检测打印函数调用
		if isPrintFunction(callExpr) {
			// 检查参数中是否包含敏感变量
			for _, arg := range callExpr.Args {
				if ident, ok := arg.(*ast.Ident); ok {
					for _, keyword := range sensitiveKeywords {
						if strings.Contains(strings.ToLower(ident.Name), keyword) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// 规则 5: 弱加密算法检测
type WeakEncryptionRule struct{}

func (r *WeakEncryptionRule) ID() string          { return "G501" }
func (r *WeakEncryptionRule) Name() string        { return "Use of Weak Cryptographic Algorithm" }
func (r *WeakEncryptionRule) Category() string    { return "Cryptography" }
func (r *WeakEncryptionRule) Severity() string    { return "High" }
func (r *WeakEncryptionRule) Description() string { return "使用弱加密算法（MD5、SHA1、DES、RC4）" }
func (r *WeakEncryptionRule) Suggestion() string  { return "使用强加密算法（SHA256、SHA512、AES、ChaCha20）" }

func (r *WeakEncryptionRule) Match(node ast.Node, ctx *RuleContext) bool {
	if selExpr, ok := node.(*ast.SelectorExpr); ok {
		// 检测加密函数调用
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			weakAlgos := []string{"md5", "sha1", "md4", "des", "rc4"}
			for _, algo := range weakAlgos {
				if strings.ToLower(ident.Name) == algo || strings.ToLower(selExpr.Sel.Name) == algo+"New" || strings.ToLower(selExpr.Sel.Name) == algo+".New" {
					return true
				}
			}
		}
	}

	if callExpr, ok := node.(*ast.CallExpr); ok {
		if ident, ok := callExpr.Fun.(*ast.Ident); ok {
			weakAlgos := []string{"md5", "sha1", "md4"}
			for _, algo := range weakAlgos {
				if strings.ToLower(ident.Name) == algo+"New" {
					return true
				}
			}
		}
	}
	return false
}

// 规则 6: 不安全文件权限检测
type InsecureFilePermRule struct{}

func (r *InsecureFilePermRule) ID() string          { return "G302" }
func (r *InsecureFilePermRule) Name() string        { return "Insecure File Permissions" }
func (r *InsecureFilePermRule) Category() string    { return "File System" }
func (r *InsecureFilePermRule) Severity() string    { return "Medium" }
func (r *InsecureFilePermRule) Description() string { return "文件权限过于宽松（如 0777）" }
func (r *InsecureFilePermRule) Suggestion() string  { return "使用更严格的文件权限（如 0600、0644）" }

func (r *InsecureFilePermRule) Match(node ast.Node, ctx *RuleContext) bool {
	if callExpr, ok := node.(*ast.CallExpr); ok {
		// 检测 os.OpenFile 或 os.Create
		if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := selExpr.X.(*ast.Ident); ok {
				if ident.Name == "os" || ident.Name == "ioutil" {
					funcName := selExpr.Sel.Name
					if funcName == "OpenFile" || funcName == "Create" || funcName == "WriteFile" {
						// 检查第三个参数（权限）
						if len(callExpr.Args) >= 3 {
							if perm, ok := callExpr.Args[2].(*ast.BasicLit); ok {
								permStr := strings.Trim(perm.Value, `"`)
								if permStr == "0777" || permStr == "0666" {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// 规则 7: 不安全 HTTP 检测
type InsecureHTTPRule struct{}

func (r *InsecureHTTPRule) ID() string          { return "G107" }
func (r *InsecureHTTPRule) Name() string        { return "Insecure HTTP Request" }
func (r *InsecureHTTPRule) Category() string    { return "Network Security" }
func (r *InsecureHTTPRule) Severity() string    { return "Medium" }
func (r *InsecureHTTPRule) Description() string { return "使用 HTTP 而非 HTTPS" }
func (r *InsecureHTTPRule) Suggestion() string  { return "使用 HTTPS 进行安全通信" }

func (r *InsecureHTTPRule) Match(node ast.Node, ctx *RuleContext) bool {
	if callExpr, ok := node.(*ast.CallExpr); ok {
		// 检测 http.Get 或 http.Post
		if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := selExpr.X.(*ast.Ident); ok {
				if ident.Name == "http" {
					funcName := selExpr.Sel.Name
					if funcName == "Get" || funcName == "Post" || funcName == "Head" || funcName == "Do" {
						// 检查 URL 参数
						if len(callExpr.Args) > 0 {
							if urlArg, ok := callExpr.Args[0].(*ast.BasicLit); ok {
								url := strings.Trim(urlArg.Value, `"`)
								if strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// 辅助函数：判断是否是字符串字面量
func isStringLiteral(expr ast.Expr) bool {
	if lit, ok := expr.(*ast.BasicLit); ok {
		return lit.Kind == token.STRING
	}
	return false
}

// 辅助函数：提取字符串字面量
func extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value
	}
	return ""
}

// 辅助函数：判断是否是打印函数
func isPrintFunction(callExpr *ast.CallExpr) bool {
	if ident, ok := callExpr.Fun.(*ast.Ident); ok {
		printFuncs := []string{"Print", "Println", "Printf", "Fprint", "Fprintln", "Fprintf", "Sprint", "Sprintln", "Sprintf", "Log", "Logf", "Logln"}
		for _, f := range printFuncs {
			if ident.Name == f {
				return true
			}
		}
	}
	if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "fmt" || ident.Name == "log" {
				return true
			}
		}
	}
	return false
}

// 辅助函数：构建安全问题
func buildSecurityIssue(rule SecurityRule, node ast.Node, fset *token.FileSet, code string) SecurityIssue {
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

	return SecurityIssue{
		ID:          fmt.Sprintf("sec-%d", position.Offset),
		RuleID:      rule.ID(),
		Severity:    rule.Severity(),
		Category:    rule.Category(),
		Description: rule.Description(),
		File:        "",
		Line:        line,
		Function:    funcName,
		CodeSnippet: codeSnippet,
		Suggestion:  rule.Suggestion(),
	}
}

// 辅助函数：去重问题
func deduplicateIssues(issues []SecurityIssue) []SecurityIssue {
	seen := make(map[string]bool)
	result := []SecurityIssue{}

	for _, issue := range issues {
		key := fmt.Sprintf("%s-%d", issue.RuleID, issue.Line)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}

	return result
}

// 辅助函数：生成安全摘要
func generateSecuritySummary(issues []SecurityIssue) string {
	if len(issues) == 0 {
		return "✅ 未检测到安全问题"
	}

	// 统计各级别数量
	critical := 0
	high := 0
	medium := 0
	low := 0
	for _, issue := range issues {
		switch issue.Severity {
		case "Critical":
			critical++
		case "High":
			high++
		case "Medium":
			medium++
		case "Low":
			low++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("检测到 %d 个安全问题", len(issues)))

	parts := []string{}
	if critical > 0 {
		parts = append(parts, fmt.Sprintf("%d Critical", critical))
	}
	if high > 0 {
		parts = append(parts, fmt.Sprintf("%d High", high))
	}
	if medium > 0 {
		parts = append(parts, fmt.Sprintf("%d Medium", medium))
	}
	if low > 0 {
		parts = append(parts, fmt.Sprintf("%d Low", low))
	}

	if len(parts) > 0 {
		sb.WriteString("（")
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString("）")
	}

	return sb.String()
}

// 辅助函数：计算安全统计
func calculateSecurityStatistics(issues []SecurityIssue) SecurityStats {
	stats := SecurityStats{
		TotalIssues: len(issues),
	}

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
