package main

import (
	"context"
	"flag"
	"fmt"
	"go-ai-study/internal/cli"
	"os"
)

const version = "1.0.0"

func main() {
	// 解析全局参数
	configFile := flag.String("c", "", "配置文件路径")
	outputFormat := flag.String("f", "text", "输出格式 (json|text)")
	outputFile := flag.String("o", "", "输出文件路径")
	verbose := flag.Bool("v", false, "详细输出")
	showVersion := flag.Bool("version", false, "显示版本信息")

	// 日志配置参数
	logLevel := flag.String("log-level", "", "日志级别 (debug|info|warn|error)")
	logFormat := flag.String("log-format", "", "日志格式 (text|json)")
	logOutput := flag.String("log-output", "", "日志输出 (stdout|stderr|file)")
	logFilePath := flag.String("log-file", "", "日志文件路径 (当 log-output=file 时使用)")

	flag.Parse()

	// 显示版本
	if *showVersion {
		fmt.Printf("go-ai-insight v%s\n", version)
		os.Exit(0)
	}

	// 创建 CLI
	cli, err := cli.NewCLI(*configFile, *outputFormat, *outputFile, *verbose,
		*logLevel, *logFormat, *logOutput, *logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 执行命令
	ctx := context.Background()
	args := flag.Args()

	if err := cli.Run(ctx, args); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
