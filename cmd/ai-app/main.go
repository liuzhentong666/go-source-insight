// Deprecated: use ./cmd. This compatibility entry delegates to the same CLI.
package main

import (
	"context"
	"go-ai-study/internal/cli"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
