package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"xingdu.app/xingdu/internal/config"
	"xingdu.app/xingdu/internal/httpapi"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := storage.Open(startup, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(store), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	slog.Info("xingdu API starting", "address", cfg.HTTPAddr, "stage", "scaffold")
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
