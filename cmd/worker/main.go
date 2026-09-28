package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/config"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	if !run() {
		os.Exit(1)
	}
}
func run() bool {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("worker requires DATABASE_URL")
		return false
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := storage.OpenRuntime(startup, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("worker database connection failed")
		return false
	}
	defer store.Close()
	slog.Info("worker scaffold started; deployment execution is not implemented")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopped")
			return true
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := store.Ready(check)
			cancel()
			if err != nil {
				slog.Warn("worker database unavailable")
			}
		}
	}
}
