package storage

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"xingdu.app/xingdu/internal/hosts"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	Pool         *pgxpool.Pool
	CloudBilling bool
}
type Host = hosts.Host

func Open(ctx context.Context, url string, mode ...string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	if len(mode) > 0 {
		cfg.ConnConfig.RuntimeParams["app.deployment_mode"] = mode[0]
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database connection failed")
	}
	return &Store{Pool: pool}, nil
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) Ready(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var exists bool
		if err := s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("database migrations pending")
		}
	}
	return nil
}

// Migrate applies embedded migrations transactionally under a database-wide lock.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(94643801)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
