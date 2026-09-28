package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
)

func TestTenantIsolation(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated test database")
	}
	ctx := context.Background()
	owner, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err = owner.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if unsafe, err := OpenRuntime(ctx, raw); err == nil {
		unsafe.Close()
		t.Fatal("privileged runtime accepted")
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
	ids := []string{}
	defer func() {
		for _, id := range ids {
			owner.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
			owner.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	user := func() string {
		id, err := s.Register(ctx, "test_"+NewID()[:8], "unused-test-hash", "Test org")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		return id
	}
	a, b := user(), user()
	oa, _ := s.Organizations(ctx, a)
	ob, _ := s.Organizations(ctx, b)
	if len(oa) != 1 || len(ob) != 1 {
		t.Fatal("organization list not scoped")
	}
	ca, cb := WithTenant(ctx, a, oa[0].ID), WithTenant(ctx, b, ob[0].ID)
	in := hosts.Input{Name: "host", Address: "same.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}}
	ha, err := s.CreateHost(ca, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateHost(cb, in); err != nil {
		t.Fatal("tenant unique index:", err)
	}
	if _, err = s.UpdateHost(cb, ha.ID, in); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant update", err)
	}
	if err = s.DeleteHost(cb, ha.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant delete", err)
	}
	if _, err = s.Hosts(WithTenant(ctx, b, oa[0].ID)); !errors.Is(err, ErrForbidden) {
		t.Fatal("forged org accepted", err)
	}
	// Query directly with the actual low-privilege role: no WHERE filter and no app guard.
	tx, _ := s.Pool.Begin(ctx)
	defer tx.Rollback(ctx)
	setScope(ctx, tx, a, oa[0].ID)
	var n int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM hosts").Scan(&n); err != nil || n != 1 {
		t.Fatal("RLS read failed", n, err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO hosts(id,name,organization_id) VALUES($1,'escape',$2)", NewID(), ob[0].ID); err == nil {
		t.Fatal("RLS write allowed foreign tenant")
	}
	tx.Rollback(ctx)
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM hosts").Scan(&n); err != nil || n != 0 {
		t.Fatal("scope leaked after rollback", n, err)
	}
	// Even a forged organization context cannot expose another organization's members/invitations.
	tx, _ = s.Pool.Begin(ctx)
	setScope(ctx, tx, b, oa[0].ID)
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE organization_id=$1", oa[0].ID).Scan(&n); err != nil || n != 0 {
		t.Fatal("membership RLS leaked", n, err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'admin')", oa[0].ID, b); err == nil {
		t.Fatal("direct membership escalation succeeded")
	}
	tx.Rollback(ctx)
	invite, err := s.CreateInvitation(ca, "viewer", "test-"+NewID())
	if err != nil {
		t.Fatal(err)
	}
	// Resolve the digest only via privileged test fixture, never expose it through HTTP.
	var hash string
	owner.Pool.QueryRow(ctx, "SELECT token_hash FROM invitations WHERE id=$1", invite.ID).Scan(&hash)
	if _, err = s.AcceptInvitation(ctx, b, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptInvitation(ctx, b, hash); !errors.Is(err, ErrNotFound) {
		t.Fatal("invitation replay", err)
	}
	viewer := WithTenant(ctx, b, oa[0].ID)
	if _, err = s.Hosts(viewer); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateHost(viewer, in); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer write", err)
	}
	if _, err = s.CreateInvitation(viewer, "member", "x"); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer invite", err)
	}
	tx, _ = s.Pool.Begin(ctx)
	setScope(ctx, tx, b, oa[0].ID)
	result, err := tx.Exec(ctx, "UPDATE hosts SET name='illegal' WHERE id=$1", ha.ID)
	if err != nil || result.RowsAffected() != 0 {
		t.Fatal("viewer RLS allowed write", err)
	}
	tx.Rollback(ctx)
	if err = s.ChangeMember(ca, b, "member", false); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeMember(viewer, a, "viewer", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("member changed owner", err)
	}
	if err = s.ChangeMember(ca, a, "viewer", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("owner removed", err)
	}

	if err = s.ChangeMember(ca, b, "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateInvitation(viewer, "admin", "blocked-"+NewID()); !errors.Is(err, ErrForbidden) {
		t.Fatal("admin invited admin", err)
	}
	existingHash := "existing-" + NewID()
	existing, err := s.CreateInvitation(ca, "viewer", existingHash)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptInvitation(ctx, b, existingHash); !errors.Is(err, ErrConflict) {
		t.Fatal("existing membership overwritten", err)
	}
	if err = s.RevokeInvitation(ca, existing.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeMember(ca, b, "", true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Hosts(viewer); !errors.Is(err, ErrForbidden) {
		t.Fatal("removed member still reads", err)
	}
	revokedHash := "test-" + NewID()
	revoked, err := s.CreateInvitation(ca, "member", revokedHash)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RevokeInvitation(ca, revoked.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AcceptInvitation(ctx, b, revokedHash); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked invitation accepted", err)
	}
	expiredHash := "test-" + NewID()
	expired, _ := s.CreateInvitation(ca, "member", expiredHash)
	owner.Pool.Exec(ctx, "UPDATE invitations SET expires_at=now()-interval '1 second' WHERE id=$1", expired.ID)
	if _, err = s.AcceptInvitation(ctx, b, expiredHash); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired invitation accepted", err)
	}
	// Two different users racing to consume one token: exactly one membership commits.
	c := user()
	raceHash := "race-" + NewID()
	if _, err = s.CreateInvitation(ca, "member", raceHash); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, who := range []string{b, c} {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _, e := s.AcceptInvitation(ctx, id, raceHash); results <- e }(who)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatal("invitation consumed multiple times", success)
	}

}

func TestLegacyTenantMigration(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable database admin")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "xingdu_migration_" + strings.ReplaceAll(NewID(), "-", "")
	if _, err = admin.Pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	uri, _ := url.Parse(raw)
	uri.Path = "/" + name
	s, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Pool.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"001_hosts.sql", "002_auth_inventory.sql"} {
		body, _ := migrations.ReadFile("migrations/" + file)
		if _, err = s.Pool.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
		s.Pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", file)
	}
	user, host := NewID(), NewID()
	if _, err = s.Pool.Exec(ctx, "INSERT INTO admins(id,username,password_hash) VALUES($1,'legacy','preserved-hash')", user); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO hosts(id,name,address) VALUES($1,'legacy host','legacy.example.invalid')", host); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,admin_id,expires_at) VALUES('legacy-session',$1,now()+interval '1 hour')", user); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.Credentials(ctx, "legacy")
	if err != nil || account.ID != user || account.PasswordHash != "preserved-hash" {
		t.Fatal("account changed", err)
	}
	session, err := s.Session(ctx, "legacy-session")
	if err != nil || session.ID != user {
		t.Fatal("session lost", err)
	}
	orgs, err := s.Organizations(ctx, user)
	if err != nil || len(orgs) != 1 || orgs[0].Role != "owner" {
		t.Fatal("owner not migrated", err)
	}
	records, err := s.Hosts(WithTenant(ctx, user, orgs[0].ID))
	if err != nil || len(records) != 1 || records[0].ID != host {
		t.Fatal("legacy host lost", err)
	}
}
