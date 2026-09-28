package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAccountOwnershipTransfer(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	db, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	uri, _ := url.Parse(raw)
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	s, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var ids []string
	defer func() {
		for _, id := range ids {
			db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range ids {
			db.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	add := func() (string, string) {
		id, e := s.Register(ctx, "owner_"+NewID("obj")[4:12], "hash", "Owner test")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		o, e := s.Organizations(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		return id, o[0].ID
	}
	a, org := add()
	b, _ := add()
	c, _ := add()
	outsider, _ := add()
	ca := WithTenant(ctx, a, org)
	for _, id := range []string{b, c} {
		if _, err = db.Pool.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'member')", org, id); err != nil {
			t.Fatal(err)
		}
	}
	token := strings.Repeat("f", 64)
	if err = s.NewSession(ctx, token, a, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.TransferOwnership(WithTenant(ctx, b, org), c, "hash", token); !errors.Is(err, ErrForbidden) {
		t.Fatal("member transferred", err)
	}
	for _, target := range []string{a, outsider} {
		if err = s.TransferOwnership(ca, target, "hash", token); !errors.Is(err, ErrConflict) {
			t.Fatal("invalid target transferred", err)
		}
	}
	if err = s.TransferOwnership(ca, b, "stale", token); !errors.Is(err, ErrConflict) {
		t.Fatal("stale hash transferred", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, target := range []string{b, c} {
		wg.Add(1)
		go func(target string) { defer wg.Done(); results <- s.TransferOwnership(ca, target, "hash", token) }(target)
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else if !errors.Is(e, ErrForbidden) {
			t.Fatal(e)
		}
	}
	if successes != 1 {
		t.Fatal("concurrent transfer count", successes)
	}
	var owners, oldAdmins, audits int
	db.Pool.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE organization_id=$1 AND role='owner'", org).Scan(&owners)
	db.Pool.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE organization_id=$1 AND user_id=$2 AND role='admin'", org, a).Scan(&oldAdmins)
	db.Pool.QueryRow(ctx, "SELECT count(*) FROM organization_audit WHERE organization_id=$1 AND event='ownership_transferred'", org).Scan(&audits)
	if owners != 1 || oldAdmins != 1 || audits != 1 {
		t.Fatal("non-atomic ownership", owners, oldAdmins, audits)
	}
}
