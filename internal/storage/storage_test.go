package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

// Requires a dedicated disposable database; never point this at a production database.
func TestMigrateIdempotent(t *testing.T) {
	url := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set XINGDU_TEST_DATABASE_URL for database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for range 2 {
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hosts(ctx); err != ErrForbidden {
		t.Fatal(err)
	}
}
