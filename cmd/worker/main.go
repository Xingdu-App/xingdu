package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/bootstrap"
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
	connector, credentialVault, origin, err := bootstrap.Configuration(cfg.PublicOrigin)
	if err != nil {
		slog.Error("invalid machine configuration")
		return false
	}
	slog.Info("machine installation worker started")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopped")
			return true
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, 110*time.Second)
			err := bootstrap.WorkOnce(check, store, credentialVault, connector, origin)
			cancel()
			if err != nil {
				slog.Warn("machine job could not finish; check worker and database availability")
			}
		}
	}
}
