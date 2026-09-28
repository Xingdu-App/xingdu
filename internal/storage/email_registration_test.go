package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestEmailVerificationAttemptsExpiryAndConcurrency(t *testing.T) {
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
	email := "storageverify_" + NewID("obj")[4:12] + "@example.invalid"
	defer func() {
		db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by IN (SELECT id FROM users WHERE username=$1)", email)
		db.Pool.Exec(ctx, "DELETE FROM users WHERE username=$1", email)
		db.Pool.Exec(ctx, "DELETE FROM pending_registrations WHERE email=$1", email)
	}()
	next := func(ch string) PendingRegistration {
		return PendingRegistration{TokenHash: strings.Repeat(ch, 64), CodeHash: strings.Repeat("a", 64), Email: email, PasswordHash: "fixture-bcrypt", Organization: "Verify test"}
	}
	p := next("1")
	if err = s.BeginEmailRegistration(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = s.VerifyEmailRegistration(ctx, p.TokenHash, p.CodeHash); !errors.Is(err, ErrVerification) {
		t.Fatal("undelivered verified", err)
	}
	if err = s.DeliverEmailRegistration(ctx, p.TokenHash); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err = s.VerifyEmailRegistration(ctx, p.TokenHash, strings.Repeat("b", 64)); !errors.Is(err, ErrVerification) {
			t.Fatal("wrong code accepted", err)
		}
	}
	if _, err = s.VerifyEmailRegistration(ctx, p.TokenHash, p.CodeHash); !errors.Is(err, ErrVerification) {
		t.Fatal("attempt limit bypass", err)
	}
	db.Pool.Exec(ctx, "UPDATE pending_registrations SET created_at=now()-interval '61 seconds' WHERE email=$1", email)
	p = next("2")
	if err = s.BeginEmailRegistration(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.DeliverEmailRegistration(ctx, p.TokenHash)
	db.Pool.Exec(ctx, "UPDATE pending_registrations SET expires_at=now()-interval '1 second',created_at=now()-interval '61 seconds' WHERE token_hash=$1", p.TokenHash)
	if _, err = s.VerifyEmailRegistration(ctx, p.TokenHash, p.CodeHash); !errors.Is(err, ErrVerification) {
		t.Fatal("expired verified", err)
	}
	p = next("3")
	if err = s.BeginEmailRegistration(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.DeliverEmailRegistration(ctx, p.TokenHash)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.VerifyEmailRegistration(ctx, p.TokenHash, p.CodeHash); results <- e }()
	}
	wg.Wait()
	close(results)
	success, denied := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrVerification) {
			denied++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal("replay race", success, denied)
	}
	// Registering an existing email cannot replace its credentials or ownership.
	db.Pool.Exec(ctx, "UPDATE pending_registrations SET created_at=now()-interval '61 seconds' WHERE email=$1", email)
	duplicate := next("4")
	duplicate.PasswordHash = "attacker-hash"
	if err = s.BeginEmailRegistration(ctx, duplicate); err != nil {
		t.Fatal(err)
	}
	s.DeliverEmailRegistration(ctx, duplicate.TokenHash)
	if _, err = s.VerifyEmailRegistration(ctx, duplicate.TokenHash, duplicate.CodeHash); !errors.Is(err, ErrVerification) {
		t.Fatal("existing account recreated", err)
	}
	credentials, err := s.Credentials(ctx, email)
	if err != nil || credentials.PasswordHash != "fixture-bcrypt" {
		t.Fatal("existing password replaced", err)
	}
}
