package main

import (
	"context"
	"log/slog"
	"os"
	"time"
	"xingdu.app/xingdu/internal/config"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("migration configuration failed")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed")
		os.Exit(1)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		slog.Error("migration failed; check database availability and migration schema")
		os.Exit(1)
	}
	if password := os.Getenv("XINGDU_APP_DATABASE_PASSWORD"); password != "" {
		if err := store.ConfigureRuntime(ctx, password); err != nil {
			slog.Error("runtime database role provisioning failed")
			os.Exit(1)
		}
	}
	if password := os.Getenv("XINGDU_WORKER_DATABASE_PASSWORD"); password != "" {
		if err := store.ConfigureWorker(ctx, password); err != nil {
			slog.Error("worker database role provisioning failed")
			os.Exit(1)
		}
	}
	slog.Info("migrations applied")
}
