package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccountPasswordAndSessionIsolation(t *testing.T) {
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
	var ids []string
	defer func() {
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	add := func() string {
		id, e := s.Register(ctx, "account_"+NewID()[:8], "old-hash", "Account test")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		return id
	}
	a, b := add(), add()
	ta, tb, tc := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	for token, id := range map[string]string{ta: a, tb: b, tc: a} {
		if err = s.NewVerifiedSession(ctx, token, id, "old-hash", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	sessions, err := s.AccountSessions(ctx, a, ta)
	if err != nil || len(sessions) != 2 {
		t.Fatal(sessions, err)
	}
	var currentID, otherID string
	for _, v := range sessions {
		if v.Current {
			currentID = v.ID
		} else {
			otherID = v.ID
		}
		if len(v.ID) != 36 {
			t.Fatal("exposed credential instead of opaque ID")
		}
	}
	if currentID == "" {
		t.Fatal("current session unidentified")
	}
	if err = s.RevokeAccountSession(ctx, b, otherID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-user revoke", err)
	}
	if err = s.RevokeAccountSession(ctx, a, otherID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, tc); !errors.Is(err, ErrNotFound) {
		t.Fatal("revoked session accepted", err)
	}
	if err = s.ChangeAccountPassword(ctx, a, "wrong-hash", "new-hash", ta); !errors.Is(err, ErrConflict) {
		t.Fatal("stale hash accepted", err)
	}
	if err = s.ChangeAccountPassword(ctx, a, "old-hash", "new-hash", tc); !errors.Is(err, ErrConflict) {
		t.Fatal("revoked session changed password", err)
	}
	if err = s.ChangeAccountPassword(ctx, a, "old-hash", "new-hash", ta); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ctx, ta); !errors.Is(err, ErrNotFound) {
		t.Fatal("password change did not revoke current session", err)
	}
	if _, err = s.Session(ctx, tb); err != nil {
		t.Fatal("other user invalidated", err)
	}
	if err = s.NewVerifiedSession(ctx, ta, a, "old-hash", time.Now().Add(time.Hour)); !errors.Is(err, ErrConflict) {
		t.Fatal("stale verified login accepted", err)
	}
	if err = s.NewVerifiedSession(ctx, ta, a, "new-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveAccountProfile(ctx, a, "Alice"); err != nil {
		t.Fatal(err)
	}
	if name, err := s.AccountProfile(ctx, a); err != nil || name != "Alice" {
		t.Fatal(name, err)
	}
	if name, err := s.AccountProfile(ctx, b); err != nil || name != "" {
		t.Fatal("profile leak", name, err)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, b, ""); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM user_profiles WHERE user_id=$1", a).Scan(&count); err != nil || count != 0 {
		t.Fatal("profile RLS leak", count, err)
	}
}
