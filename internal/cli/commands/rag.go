package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"go-ai-study/internal/source"
	"os"
	"path"
	"strings"

	"go-ai-study/internal/ai"
	"go-ai-study/internal/cli/output"
)

type RAGClient interface {
	Index(context.Context, string) (*ai.IndexSummary, error)
	Ask(context.Context, string, string) (*ai.AnswerResult, error)
	Close() error
}
type RAGFactory func(context.Context, string) (RAGClient, error)

type IndexCommand struct{ factory RAGFactory }

func NewIndexCommand(factory RAGFactory) *IndexCommand { return &IndexCommand{factory} }
func (c *IndexCommand) Name() string                   { return "index" }
func (c *IndexCommand) Description() string {
	return "扫描并索引 Go 源码，更新当前代码库的向量索引"
}
func (c *IndexCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	opts, err := parseRAGArgs(args, false)
	if err != nil {
		return err
	}
	if opts.help {
		return output.Print(formatter, "使用: index [--collection NAME] PATH")
	}
	if len(opts.operands) != 1 {
		return fmt.Errorf("使用: index [--collection NAME] PATH")
	}
	info, err := os.Stat(opts.operands[0])
	if err != nil {
		return fmt.Errorf("访问索引路径失败: %w", err)
	}
	if !info.IsDir() && !source.IsGoFile(opts.operands[0]) {
		return fmt.Errorf("index 只接受目录或 Go 源文件")
	}
	svc, err := c.factory(ctx, opts.collection)
	if err != nil {
		return err
	}
	defer svc.Close()
	result, err := svc.Index(ctx, opts.operands[0])
	if err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return output.Print(formatter, string(data))
}

type AskCommand struct{ factory RAGFactory }

func NewAskCommand(factory RAGFactory) *AskCommand { return &AskCommand{factory} }
func (c *AskCommand) Name() string                 { return "ask" }
func (c *AskCommand) Description() string {
	return "检索已索引代码，使用 Ollama 回答问题并返回来源"
}
func (c *AskCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	opts, err := parseRAGArgs(args, true)
	if err != nil {
		return err
	}
	if opts.help {
		return output.Print(formatter, "使用: ask [--collection NAME] [--file RELATIVE_PATH] QUESTION")
	}
	question := strings.TrimSpace(strings.Join(opts.operands, " "))
	if question == "" {
		return fmt.Errorf("使用: ask [--collection NAME] [--file RELATIVE_PATH] QUESTION")
	}
	svc, err := c.factory(ctx, opts.collection)
	if err != nil {
		return err
	}
	defer svc.Close()
	result, err := svc.Ask(ctx, question, opts.file)
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("问答服务没有返回结果")
	}
	if result.Answer == "" {
		result.Answer = "未找到相关源码，请确认已执行 index 或检查文件过滤条件。"
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return output.Print(formatter, string(data))
}

type ragArgs struct {
	collection, file string
	operands         []string
	help             bool
}

func parseRAGArgs(args []string, allowFile bool) (ragArgs, error) {
	var opts ragArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.operands = append(opts.operands, args[i+1:]...)
			break
		}
		if arg == "--help" || arg == "-h" {
			opts.help = true
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "--collection" || (allowFile && name == "--file") {
			if !hasValue {
				i++
				if i >= len(args) {
					return opts, fmt.Errorf("%s 需要参数", name)
				}
				value = args[i]
			}
			if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "--") {
				return opts, fmt.Errorf("%s 需要非空参数", name)
			}
			if name == "--collection" {
				opts.collection = value
			} else {
				opts.file = value
			}
		} else if strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("未知参数: %s", arg)
		} else {
			opts.operands = append(opts.operands, arg)
		}
	}
	if opts.file != "" {
		opts.file = strings.ReplaceAll(opts.file, "\\", "/")
		opts.file = path.Clean(opts.file)
		if path.IsAbs(opts.file) || opts.file == ".." || strings.HasPrefix(opts.file, "../") || strings.Contains(opts.file, ":") {
			return opts, fmt.Errorf("--file 必须是代码库内的相对路径")
		}
	}
	return opts, nil
}
