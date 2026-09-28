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

type oauthFixture struct {
	ctx          context.Context
	admin, store *Store
	users        []string
}

func newOAuthFixture(t *testing.T) *oauthFixture {
	t.Helper()
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = admin.Migrate(ctx); err != nil {
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
	f := &oauthFixture{ctx: ctx, admin: admin, store: s}
	t.Cleanup(func() {
		for _, id := range f.users {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range f.users {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
		s.Close()
		admin.Close()
	})
	return f
}
func oauthHash() string { return strings.ReplaceAll(NewID()+NewID(), "-", "") }
func (f *oauthFixture) local(t *testing.T) (User, string) {
	t.Helper()
	email := "oauth_" + NewID()[:8] + "@example.invalid"
	id, err := f.store.Register(f.ctx, email, "fixture-hash", "OAuth test")
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, id)
	hash := oauthHash()
	if err = f.store.NewSession(f.ctx, hash, id, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return User{ID: id, Username: email}, hash
}
func TestOAuthStateBrowserProviderExpirySingleUse(t *testing.T) {
	f := newOAuthFixture(t)
	v := OAuthState{Hash: oauthHash(), BrowserHash: oauthHash(), Provider: "google", Mode: "login", Encrypted: []byte("encrypted")}
	defer f.admin.Pool.Exec(f.ctx, "DELETE FROM oauth_states WHERE state_hash=$1", v.Hash)
	if err := f.store.SaveOAuthState(f.ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ConsumeOAuthState(f.ctx, v.Hash, oauthHash(), "google"); !errors.Is(err, ErrOAuthState) {
		t.Fatal("wrong browser accepted", err)
	}
	if _, err := f.store.ConsumeOAuthState(f.ctx, v.Hash, v.BrowserHash, "github"); !errors.Is(err, ErrOAuthState) {
		t.Fatal("provider mixup accepted", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.store.ConsumeOAuthState(f.ctx, v.Hash, v.BrowserHash, v.Provider)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	accepted, denied := 0, 0
	for e := range results {
		if e == nil {
			accepted++
		} else if errors.Is(e, ErrOAuthState) {
			denied++
		} else {
			t.Fatal(e)
		}
	}
	if accepted != 1 || denied != 1 {
		t.Fatal("state race", accepted, denied)
	}
	var n int
	f.admin.Pool.QueryRow(f.ctx, "SELECT octet_length(encrypted) FROM oauth_states WHERE state_hash=$1", v.Hash).Scan(&n)
	if n != 0 {
		t.Fatal("consumed PKCE retained")
	}
	f.admin.Pool.Exec(f.ctx, "UPDATE oauth_states SET used_at=NULL,expires_at=now()-interval '1 second' WHERE state_hash=$1", v.Hash)
	if _, err := f.store.ConsumeOAuthState(f.ctx, v.Hash, v.BrowserHash, v.Provider); !errors.Is(err, ErrOAuthState) {
		t.Fatal("expired state accepted", err)
	}
}
func TestOAuthUnverifiedUsernameDoesNotAutoLinkAndStableIdentity(t *testing.T) {
	f := newOAuthFixture(t)
	local, _ := f.local(t)
	subject := NewID()
	state := OAuthState{Provider: "google", Mode: "login"}
	if _, err := f.store.CompleteOAuth(f.ctx, state, subject, local.Username, oauthHash(), time.Now().Add(time.Hour), true); !errors.Is(err, ErrOAuthEmailExists) {
		t.Fatal("auto-linked existing email", err)
	}
	email := "social_" + NewID()[:8] + "@example.invalid"
	if _, err := f.store.CompleteOAuth(f.ctx, state, subject, email, oauthHash(), time.Now().Add(time.Hour), false); !errors.Is(err, ErrOAuthRegistrationDisabled) {
		t.Fatal("registration bypass", err)
	}
	hash := oauthHash()
	social, err := f.store.CompleteOAuth(f.ctx, state, subject, email, hash, time.Now().Add(time.Hour), true)
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, social.ID)
	credentials, e := f.store.Credentials(f.ctx, email)
	if e != nil || credentials.PasswordLoginEnabled || credentials.PasswordHash != "" {
		t.Fatal("social password method enabled", e)
	}
	if e = f.store.NewVerifiedSession(f.ctx, oauthHash(), social.ID, "", time.Now().Add(time.Hour)); !errors.Is(e, ErrConflict) {
		t.Fatal("disabled password issued session", e)
	}
	if e = f.store.ChangeAccountPassword(f.ctx, social.ID, "", "replacement-hash", hash); !errors.Is(e, ErrConflict) {
		t.Fatal("disabled password changed", e)
	}
	methods, err := f.store.LoginMethods(f.ctx, social.ID)
	if err != nil || methods.HasPassword || len(methods.Identities) != 1 {
		t.Fatal("social method metadata", methods, err)
	}
	again, err := f.store.CompleteOAuth(f.ctx, state, subject, "changed@example.invalid", oauthHash(), time.Now().Add(time.Hour), false)
	if err != nil || again.ID != social.ID {
		t.Fatal("subject identity not stable", again, err)
	}
	link := OAuthState{Provider: "google", Mode: "link", UserID: local.ID, SessionHash: oauthHash()}
	if _, err = f.store.CompleteOAuth(f.ctx, link, subject, email, "", time.Time{}, true); !errors.Is(err, ErrOAuthIdentityInUse) {
		t.Fatal("identity stolen", err)
	}
	if err = f.store.UnlinkOAuthIdentity(f.ctx, social.ID, "google", hash, []string{"google", "github"}); !errors.Is(err, ErrLastLoginMethod) {
		t.Fatal("last method removed", err)
	}
	tx, err := f.store.Pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	setScope(f.ctx, tx, local.ID, "")
	var count int
	err = tx.QueryRow(f.ctx, "SELECT count(*) FROM oauth_identities WHERE user_id=$1", social.ID).Scan(&count)
	if err != nil || count != 0 {
		t.Fatal("identity RLS leak", count, err)
	}
}
func TestOAuthLinkSessionAndConcurrentUnlink(t *testing.T) {
	f := newOAuthFixture(t)
	state := OAuthState{Provider: "google", Mode: "login"}
	hash := oauthHash()
	u, err := f.store.CompleteOAuth(f.ctx, state, NewID(), "social_"+NewID()[:8]+"@example.invalid", hash, time.Now().Add(time.Hour), true)
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, u.ID)
	link := OAuthState{Provider: "github", Mode: "link", UserID: u.ID, SessionHash: oauthHash()}
	subject := NewID()
	if _, err = f.store.CompleteOAuth(f.ctx, link, subject, "github@example.invalid", "", time.Time{}, false); !errors.Is(err, ErrOAuthLinkSession) {
		t.Fatal("dead link session accepted", err)
	}
	link.SessionHash = hash
	if _, err = f.store.CompleteOAuth(f.ctx, link, subject, "github@example.invalid", "", time.Time{}, false); err != nil {
		t.Fatal(err)
	}
	if err = f.store.UnlinkOAuthIdentity(f.ctx, u.ID, "google", hash, []string{"google"}); !errors.Is(err, ErrLastLoginMethod) {
		t.Fatal("disabled fallback counted", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, provider := range []string{"google", "github"} {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			results <- f.store.UnlinkOAuthIdentity(f.ctx, u.ID, p, hash, []string{"google", "github"})
		}(provider)
	}
	wg.Wait()
	close(results)
	success, guarded := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrLastLoginMethod) {
			guarded++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || guarded != 1 {
		t.Fatal("concurrent unlink lost last method", success, guarded)
	}
	methods, err := f.store.LoginMethods(f.ctx, u.ID)
	if err != nil || len(methods.Identities) != 1 {
		t.Fatal(methods, err)
	}
}

func TestOAuthVerifiedEmailAutoLink(t *testing.T) {
	f := newOAuthFixture(t)
	local, _ := f.local(t)
	// Legacy username-only users must not be treated as verified email owners.
	if _, err := f.admin.Pool.Exec(f.ctx, `UPDATE users SET email=$1,email_verified_at=now() WHERE id=$2`, local.Username, local.ID); err != nil {
		t.Fatal(err)
	}
	state := OAuthState{Provider: "google", Mode: "login"}
	subject := NewID()
	hash := oauthHash()
	user, err := f.store.CompleteOAuth(f.ctx, state, subject, "  "+strings.ToUpper(local.Username)+"  ", hash, time.Now().Add(time.Hour), false)
	if err != nil || user.ID != local.ID {
		t.Fatal("verified email must link while registration is closed", user, err)
	}
	methods, err := f.store.LoginMethods(f.ctx, local.ID)
	if err != nil || !methods.HasPassword || len(methods.Identities) != 1 || methods.Identities[0].Email != local.Username {
		t.Fatal(methods, err)
	}
	var owner string
	if err = f.admin.Pool.QueryRow(f.ctx, `SELECT user_id::text FROM sessions WHERE token_hash=$1`, hash).Scan(&owner); err != nil || owner != local.ID {
		t.Fatal("wrong session account", err)
	}
	var organizations int
	if err = f.admin.Pool.QueryRow(f.ctx, `SELECT count(*) FROM organizations WHERE created_by=$1`, local.ID).Scan(&organizations); err != nil || organizations != 1 {
		t.Fatal("existing organization changed", organizations, err)
	}
	if _, err = f.store.CompleteOAuth(f.ctx, state, NewID(), local.Username, oauthHash(), time.Now().Add(time.Hour), false); !errors.Is(err, ErrOAuthIdentityInUse) {
		t.Fatal("replaced existing provider identity", err)
	}
	other, _ := f.local(t)
	if _, err = f.admin.Pool.Exec(f.ctx, `UPDATE users SET email=$1,email_verified_at=now() WHERE id=$2`, other.Username, other.ID); err != nil {
		t.Fatal(err)
	}
	again, err := f.store.CompleteOAuth(f.ctx, state, subject, other.Username, oauthHash(), time.Now().Add(time.Hour), false)
	if err != nil || again.ID != local.ID {
		t.Fatal("email change transferred stable identity", again, err)
	}
}

func TestOAuthConcurrentProvidersShareOneAccount(t *testing.T) {
	f := newOAuthFixture(t)
	email := "concurrent_" + NewID()[:8] + "@example.invalid"
	type result struct {
		user User
		err  error
	}
	results := make(chan result, 2)
	for _, provider := range []string{"google", "github"} {
		go func(provider string) {
			u, e := f.store.CompleteOAuth(f.ctx, OAuthState{Provider: provider, Mode: "login"}, NewID(), email, oauthHash(), time.Now().Add(time.Hour), true)
			results <- result{u, e}
		}(provider)
	}
	first, second := <-results, <-results
	if first.user.ID != "" {
		f.users = append(f.users, first.user.ID)
	}
	if second.user.ID != "" && second.user.ID != first.user.ID {
		f.users = append(f.users, second.user.ID)
	}
	if first.err != nil || second.err != nil || first.user.ID != second.user.ID {
		t.Fatal("concurrent providers created separate accounts", first, second)
	}
	methods, err := f.store.LoginMethods(f.ctx, first.user.ID)
	if err != nil || methods.HasPassword || len(methods.Identities) != 2 {
		t.Fatal(methods, err)
	}
	var count int
	if err = f.admin.Pool.QueryRow(f.ctx, `SELECT count(*) FROM organizations WHERE created_by=$1`, first.user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate organizations", count, err)
	}
}
