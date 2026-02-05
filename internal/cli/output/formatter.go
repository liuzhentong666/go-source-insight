package output

// Formatter 输出格式化接口
type Formatter interface {
	Format(result string) string
}

// Options 格式化选项
type Options struct {
	Verbose bool
}
