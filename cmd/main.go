package main

import (
	"context"
	"os"
	"os/signal"

	"go-ai-study/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
