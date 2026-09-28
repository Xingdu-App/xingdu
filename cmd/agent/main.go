package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	version := flag.Bool("version", false, "print version")
	flag.Parse()
	if *version {
		fmt.Println("xingdu-agent 0.1.0-dev")
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("agent scaffold started; enrollment and host operations are not implemented")
	<-ctx.Done()
	slog.Info("agent stopped")
}
