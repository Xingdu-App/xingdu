package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"net/url"
	"os"
	"sync"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
)

func TestOrganizationQuotaAndIsolation(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("options", "-crole=xingdu_app")
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.Register(ctx, "quota_"+NewID("obj")[4:12], "unused", "Quota test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
	}()
	orgs, err := s.Organizations(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	org := orgs[0].ID
	scoped := WithTenant(ctx, id, org)
	_, err = admin.Pool.Exec(ctx, "INSERT INTO organization_limits(organization_id,hosts) VALUES($1,1)", org)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			_, e := s.CreateHost(scoped, hosts.Input{Name: "Quota host", Address: "example.com", SSHPort: 22, SSHUser: "root", Tags: []string{}})
			results <- e
		})
	}
	wg.Wait()
	close(results)
	successes, limited := 0, 0
	for e := range results {
		var p *pgconn.PgError
		if e == nil {
			successes++
		} else if errors.As(e, &p) && p.Code == "P0004" {
			limited++
		} else {
			t.Fatal(e)
		}
	}
	if successes != 1 || limited != 1 {
		t.Fatalf("quota race: successes=%d limited=%d", successes, limited)
	}
	ops, err := s.OrganizationOperations(scoped)
	if err != nil {
		t.Fatal(err)
	}
	if ops.Usage["hosts"] != (ResourceUsage{1, 1}) {
		t.Fatal(ops.Usage)
	}
	if _, err = s.OrganizationOperations(WithTenant(ctx, id, NewID("obj"))); !errors.Is(err, ErrForbidden) {
		t.Fatalf("tenant escape: %v", err)
	}
	_, err = s.Pool.Exec(scoped, "UPDATE organization_limits SET hosts=999")
	if err == nil {
		t.Fatal("tenant edited limits")
	}
}
