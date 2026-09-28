package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"os"
	"testing"
)

func TestDeploymentOrganizationLimit(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated test database")
	}
	ctx := context.Background()
	s, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"self_hosted", "cloud"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := s.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			// Isolate registry contents transactionally; no existing tenant is modified.
			if _, err = tx.Exec(ctx, "DELETE FROM deployment_organizations"); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, "SET LOCAL ROLE xingdu_app"); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, "SELECT set_config('app.deployment_mode',$1,true)", mode); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				user, org := NewID(), NewID()
				if err = setScope(ctx, tx, user, org); err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, "INSERT INTO users(id,username,password_hash) VALUES($1,$2,'unused')", user, user); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(ctx, "INSERT INTO organizations(id,name,created_by) VALUES($1,'Mode test',$2)", org, user)
				if mode == "self_hosted" && i == 1 {
					var pg *pgconn.PgError
					if !errors.As(err, &pg) || pg.Code != "P0006" {
						t.Fatalf("second organization was not rejected: %v", err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
