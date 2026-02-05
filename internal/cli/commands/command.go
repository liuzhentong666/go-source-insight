package commands

import (
	"context"
	"go-ai-study/internal/cli/output"
)

// Command 命令接口
type Command interface {
	// Name 命令名称
	Name() string

	// Description 命令描述
	Description() string

	// Run 执行命令
	Run(ctx context.Context, args []string, formatter output.Formatter) error
}

// CommandRegistry 命令注册表
type CommandRegistry struct {
	commands map[string]Command
}

// NewCommandRegistry 创建命令注册表
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
	}
}

// Register 注册命令
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// Get 获取命令
func (r *CommandRegistry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// List 列出所有命令
func (r *CommandRegistry) List() []Command {
	var list []Command
	for _, cmd := range r.commands {
		list = append(list, cmd)
	}
	return list
}
