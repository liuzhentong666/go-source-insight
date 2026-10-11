package main

import (
	"context"
	"flag"
	"fmt"
	"go-ai-study/internal/localfiles"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:19100", "source reader listen address")
	tokenFile := flag.String("token-file", "", "restricted bearer token file")
	flag.Parse()
	token, err := os.ReadFile(*tokenFile)
	if err != nil || len(strings.TrimSpace(string(token))) < 32 {
		fmt.Fprintln(os.Stderr, "source reader requires a strong token file")
		os.Exit(1)
	}
	server := &http.Server{Addr: *listen, Handler: localfiles.Handler(strings.TrimSpace(string(token))), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "source reader could not start")
		os.Exit(1)
	}
}
