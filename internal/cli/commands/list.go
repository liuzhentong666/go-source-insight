package commands

import (
	"context"
	"encoding/json"
	"go-ai-study/internal/cli/output"
	"sort"
)

type ListCommand struct{ registry *CommandRegistry }

func NewListCommand(registry *CommandRegistry) *ListCommand { return &ListCommand{registry} }
func (c *ListCommand) Name() string                         { return "list" }
func (c *ListCommand) Description() string                  { return "列出可用命令" }
func (c *ListCommand) Run(ctx context.Context, args []string, formatter output.Formatter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	list := c.registry.List()
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	type item struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	items := make([]item, 0, len(list))
	for _, cmd := range list {
		items = append(items, item{cmd.Name(), cmd.Description()})
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return output.Print(formatter, string(data))
}
