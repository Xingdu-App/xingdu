package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestAccountInvitationMemberQuota(t *testing.T) {
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
		id, e := s.Register(ctx, "quota_"+NewID()[:8], "hash", "Member quota test")
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
	owner, org := add()
	a, _ := add()
	b, _ := add()
	ca := WithTenant(ctx, owner, org)
	if _, err = db.Pool.Exec(ctx, "INSERT INTO organization_limits(organization_id,members) VALUES($1,1)", org); err != nil {
		t.Fatal(err)
	}
	tokenA, tokenB := strings.Repeat("1", 64), strings.Repeat("2", 64)
	for _, hash := range []string{tokenA, tokenB} {
		if _, err = s.CreateInvitation(ca, "member", hash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.AcceptInvitation(ctx, a, tokenA); !quotaFailure(err) {
		t.Fatal("not-yet-member bypassed custom quota", err)
	}
	// Failure must leave the invitation available for a later capacity increase.
	if _, err = db.Pool.Exec(ctx, "UPDATE organization_limits SET members=2 WHERE organization_id=$1", org); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for user, hash := range map[string]string{a: tokenA, b: tokenB} {
		wg.Add(1)
		go func(user, hash string) { defer wg.Done(); _, e := s.AcceptInvitation(ctx, user, hash); results <- e }(user, hash)
	}
	wg.Wait()
	close(results)
	success, limited := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if quotaFailure(e) {
			limited++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || limited != 1 {
		t.Fatal("concurrent quota result", success, limited)
	}
	var members int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE organization_id=$1", org).Scan(&members); err != nil || members != 2 {
		t.Fatal("quota exceeded", members, err)
	}
	// Operators without tenant session scope must also be checked against NEW.org.
	outsider, _ := add()
	if _, err = db.Pool.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'member')", org, outsider); !quotaFailure(err) {
		t.Fatal("unscoped insert bypassed quota", err)
	}
}
func quotaFailure(err error) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == "P0004"
}
