package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"go-ai-study/internal/cli/output"
)

const Version = "1.0.0"

// Execute is shared by the CLI entry points and can be tested without os.Exit.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-ai-insight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var configPath, format, outputPath, logLevel, logFormat, logOutput, logFile string
	var verbose, version bool
	flags.StringVar(&configPath, "c", "", "配置文件")
	flags.StringVar(&configPath, "config", "", "配置文件")
	flags.StringVar(&format, "f", "", "输出格式")
	flags.StringVar(&format, "format", "", "输出格式")
	flags.StringVar(&outputPath, "o", "", "报告文件")
	flags.StringVar(&outputPath, "output", "", "报告文件")
	flags.BoolVar(&verbose, "v", false, "详细输出")
	flags.BoolVar(&verbose, "verbose", false, "详细输出")
	flags.BoolVar(&version, "version", false, "版本")
	flags.StringVar(&logLevel, "log-level", "", "日志级别")
	flags.StringVar(&logFormat, "log-format", "", "日志格式")
	flags.StringVar(&logOutput, "log-output", "", "日志输出")
	flags.StringVar(&logFile, "log-file", "", "日志文件")
	flags.Usage = func() { fmt.Fprintln(stderr, helpText) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if version {
		fmt.Fprintf(stdout, "go-ai-insight v%s\n", Version)
		return 0
	}
	app, err := NewCLI(configPath, format, outputPath, verbose, logLevel, logFormat, logOutput, logFile)
	if err != nil {
		fmt.Fprintf(stderr, "初始化失败: %v\n", err)
		return 1
	}
	app.writer = stdout
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "v" || f.Name == "verbose" {
			app.config.Verbose = verbose
			if app.config.DefaultFormat == "text" {
				app.formatter = output.NewTextFormatter(output.Options{Verbose: verbose})
			}
		}
	})
	if app.config.DefaultOutput == "stderr" {
		app.writer = stderr
	}
	if err := app.Run(ctx, flags.Args()); err != nil {
		fmt.Fprintf(stderr, "错误: %v\n", err)
		return 1
	}
	return 0
}
