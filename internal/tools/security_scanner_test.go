package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// 测试硬编码密钥检测
func TestSecurityScanner_HardCodedSecrets(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

func Login() bool {
	password := "admin123"
	apiKey := "sk-1234567890"
	token := "secret_token_123"
	return true
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到硬编码密钥")
	}

	// 检查是否有 Critical 级别的问题
	hasCritical := false
	for _, issue := range analysis.Issues {
		if issue.Severity == "Critical" && issue.RuleID == "G101" {
			hasCritical = true
			break
		}
	}

	if !hasCritical {
		t.Fatal("应该检测到 Critical 级别的硬编码密钥")
	}
}

// 测试 SQL 注入检测
func TestSecurityScanner_SQLInjection(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

func QueryUser(id string) {
	query := "SELECT * FROM users WHERE id=" + id
	db.Exec(query)
}

func InsertUser(name, email string) {
	sql := "INSERT INTO users (name, email) VALUES ('" + name + "', '" + email + "')"
	db.Exec(sql)
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到 SQL 注入风险")
	}

	// 检查是否有 Critical 级别的问题
	hasSQLInjection := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G201" {
			hasSQLInjection = true
			break
		}
	}

	if !hasSQLInjection {
		t.Fatal("应该检测到 SQL 注入风险")
	}
}

// 测试不安全随机数检测
func TestSecurityScanner_WeakRandom(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import "math/rand"

func GenerateToken() int {
	return rand.Intn(1000000)
}

func RandomFloat() float64 {
	return rand.Float64()
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到不安全随机数")
	}

	// 检查是否有 High 级别的问题
	hasWeakRandom := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G401" && issue.Severity == "High" {
			hasWeakRandom = true
			break
		}
	}

	if !hasWeakRandom {
		t.Fatal("应该检测到 High 级别的不安全随机数")
	}
}

// 测试敏感信息打印检测
func TestSecurityScanner_InfoDisclosure(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import "fmt"

func ProcessLogin(username, password string) {
	fmt.Println("Username:", username)
	fmt.Println("Password:", password)
	fmt.Printf("Token: %s\n", authToken)
}

var authToken = "secret123"
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到敏感信息打印")
	}

	// 检查是否有 Medium 级别的问题
	hasDisclosure := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G104" && issue.Severity == "Medium" {
			hasDisclosure = true
			break
		}
	}

	if !hasDisclosure {
		t.Fatal("应该检测到 Medium 级别的敏感信息打印")
	}
}

// 测试弱加密算法检测
func TestSecurityScanner_WeakEncryption(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import (
	"crypto/md5"
	"crypto/sha1"
)

func HashMD5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return string(h.Sum(nil))
}

func HashSHA1(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return string(h.Sum(nil))
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到弱加密算法")
	}

	// 检查是否有 High 级别的问题
	hasWeakEncryption := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G501" && issue.Severity == "High" {
			hasWeakEncryption = true
			break
		}
	}

	if !hasWeakEncryption {
		t.Fatal("应该检测到 High 级别的弱加密算法")
	}
}

// 测试不安全文件权限检测
func TestSecurityScanner_InsecureFilePerm(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import "os"

func WriteFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0777)
}

func OpenFile(filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
	return err
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到不安全文件权限")
	}

	// 检查是否有 Medium 级别的问题
	hasInsecurePerm := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G302" && issue.Severity == "Medium" {
			hasInsecurePerm = true
			break
		}
	}

	if !hasInsecurePerm {
		t.Fatal("应该检测到 Medium 级别的不安全文件权限")
	}
}

// 测试不安全 HTTP 检测
func TestSecurityScanner_InsecureHTTP(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import "net/http"

func FetchData() error {
	resp, err := http.Get("http://example.com/api/data")
	return err
}

func PostData(data string) error {
	_, err := http.Post("http://example.com/api", "application/json", strings.NewReader(data))
	return err
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到不安全 HTTP")
	}

	// 检查是否有 Medium 级别的问题
	hasInsecureHTTP := false
	for _, issue := range analysis.Issues {
		if issue.RuleID == "G107" && issue.Severity == "Medium" {
			hasInsecureHTTP = true
			break
		}
	}

	if !hasInsecureHTTP {
		t.Fatal("应该检测到 Medium 级别的不安全 HTTP")
	}
}

// 测试安全代码（无问题）
func TestSecurityScanner_SafeCode(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"os"
)

func SafeHash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return string(h.Sum(nil))
}

func SafeRandom() int {
	b := make([]byte, 4)
	rand.Read(b)
	return int(b[0])
}

func SafeQuery(db *sql.DB, id string) {
	db.Query("SELECT * FROM users WHERE id = ?", id)
}

func SafePrint(username string) {
	fmt.Println("Username:", username)
}

func SafeFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0600)
}

func SafeHTTP() error {
	resp, err := http.Get("https://example.com/api")
	return err
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total != 0 {
		t.Fatalf("安全代码不应检测到问题，实际检测到 %d 个", analysis.Total)
	}

	if !strings.Contains(analysis.Summary, "✅") {
		t.Fatal("摘要应该表示安全")
	}
}

// 测试空代码
func TestSecurityScanner_EmptyCode(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	_, err := scanner.Run(ctx, "")
	if err == nil {
		t.Fatal("空代码应该返回错误")
	}

	if !strings.Contains(err.Error(), "解析") {
		t.Fatalf("错误信息应该包含'解析': %v", err)
	}
}

// 测试语法错误
func TestSecurityScanner_SyntaxError(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	_, err := scanner.Run(ctx, "this is not valid go code {")
	if err == nil {
		t.Fatal("无效代码应该返回错误")
	}

	if !strings.Contains(err.Error(), "解析") {
		t.Fatalf("错误信息应该包含'解析': %v", err)
	}
}

// 测试多个问题同时存在
func TestSecurityScanner_MultipleIssues(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import (
	"fmt"
	"math/rand"
	"net/http"
)

func Login(username, password string) bool {
	apiKey := "sk-123456"
	query := "SELECT * FROM users WHERE username='" + username + "'"
	fmt.Println("Password:", password)
	rand.Intn(100)
	http.Get("http://example.com")
	return true
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 应该检测到多个问题
	if analysis.Total < 3 {
		t.Fatalf("应该检测到至少 3 个问题，实际 %d", analysis.Total)
	}

	stats := analysis.Statistics
	if stats.TotalIssues < 3 {
		t.Fatalf("统计信息错误")
	}
}

// 测试 JSON 输出格式
func TestSecurityScanner_JSONFormat(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

func Example() string {
	password := "secret123"
	return password
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	// 验证是有效的 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("输出不是有效的 JSON: %v", err)
	}

	// 验证必要字段
	requiredFields := []string{"total", "issues", "summary", "statistics"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			t.Fatalf("缺少必要字段: %s", field)
		}
	}
}

// 测试与 ToolManager 集成
func TestSecurityScanner_ToolManagerIntegration(t *testing.T) {
	logger := NewNoopLogger()
	tm := NewToolManager(logger)

	scanner := NewSecurityScanner()
	config := DefaultToolConfig("security_scanner")

	err := tm.Register(scanner, config)
	if err != nil {
		t.Fatalf("注册工具失败: %v", err)
	}

	code := `package main

func Example() string {
	password := "secret123"
	return password
}
`

	result, err := tm.Run(context.Background(), "security_scanner", code)
	if err != nil {
		t.Fatalf("执行工具失败: %v", err)
	}

	if !result.Success {
		t.Fatalf("工具应该执行成功: %s", result.Error)
	}

	// 验证输出
	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result.Result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	if analysis.Total == 0 {
		t.Fatal("应该检测到安全问题")
	}
}

// 演示测试 - 展示实际输出
func TestSecurityScanner_Demo(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import (
	"database/sql"
	"fmt"
	"math/rand"
)

func Login(username, password string) bool {
	// 问题 1: 硬编码密码
	adminPassword := "admin123"

	// 问题 2: SQL 注入
	query := "SELECT * FROM users WHERE username='" + username + "'"
	fmt.Println("Query:", query)

	// 问题 3: 打印密码
	fmt.Println("Password:", password)

	// 问题 4: 不安全随机数
	token := rand.Intn(1000000)

	return password == adminPassword && token > 0
}

func main() {
	Login("admin", "password123")
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	t.Log("=== 安全扫描结果 ===")
	t.Log(result)
}

// 格式化输出演示
func TestSecurityScanner_FormattedOutput(t *testing.T) {
	scanner := NewSecurityScanner()
	ctx := context.Background()

	code := `package main

import "fmt"

func Login(username, password string) bool {
	adminPassword := "admin123"
	query := "SELECT * FROM users WHERE username='" + username + "'"
	fmt.Println("Password:", password)
	return password == adminPassword
}
`

	result, err := scanner.Run(ctx, code)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	// 解析 JSON
	var analysis SecurityResult
	if err := json.Unmarshal([]byte(result), &analysis); err != nil {
		t.Fatalf("解析结果失败: %v", err)
	}

	// 格式化输出
	t.Log("\n========== 安全扫描报告 ==========")
	t.Logf("\n📊 总体信息")
	t.Logf("  - 文件: %s", analysis.File)
	t.Logf("  - 总问题数: %d", analysis.Total)
	t.Logf("  - %s", analysis.Summary)

	t.Logf("\n⚠️  统计信息")
	stats := analysis.Statistics
	t.Logf("  - Critical: %d", stats.Critical)
	t.Logf("  - High: %d", stats.High)
	t.Logf("  - Medium: %d", stats.Medium)
	t.Logf("  - Low: %d", stats.Low)

	if analysis.Total > 0 {
		t.Logf("\n📋 问题详情")
		for i, issue := range analysis.Issues {
			t.Logf("\n  问题 #%d:", i+1)
			t.Logf("    ID: %s", issue.ID)
			t.Logf("    规则: %s - %s", issue.RuleID, issue.Category)
			t.Logf("    严重程度: %s", issue.Severity)
			t.Logf("    位置: 第 %d 行 (%s)", issue.Line, issue.Function)
			t.Logf("    代码: %s", issue.CodeSnippet)
			t.Logf("    描述: %s", issue.Description)
			t.Logf("    建议: %s", issue.Suggestion)
		}
	} else {
		t.Log("\n✅ 未检测到安全问题")
	}
	t.Log("\n=====================================")
}
