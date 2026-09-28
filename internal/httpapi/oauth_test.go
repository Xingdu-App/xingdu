package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/socialauth"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type mockOAuthProvider struct {
	subject, email  string
	fail            bool
	nonce, verifier string
}

func (p *mockOAuthProvider) AuthorizationURL(state, verifier, nonce string) string {
	p.nonce = nonce
	p.verifier = verifier
	return "https://provider.example.invalid/authorize?state=" + state
}
func (p *mockOAuthProvider) Verify(_ context.Context, code, verifier, nonce string) (socialauth.Identity, error) {
	if p.fail || code != "fixture-code" || verifier != p.verifier || nonce != p.nonce {
		return socialauth.Identity{}, errors.New("private provider token response")
	}
	return socialauth.Identity{Subject: p.subject, Email: p.email}, nil
}
func TestOAuthHTTPBrowserBindingLinkAndUnlink(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	db, err := storage.Open(ctx, raw)
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
	s, err := storage.Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	email := "httpoauth_" + storage.NewID()[:16] + "@example.invalid"
	var ids, states []string
	defer func() {
		for _, hash := range states {
			db.Pool.Exec(ctx, "DELETE FROM oauth_states WHERE state_hash=$1", hash)
		}
		for _, id := range ids {
			db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range ids {
			db.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	google := &mockOAuthProvider{subject: storage.NewID(), email: email}
	github := &mockOAuthProvider{subject: storage.NewID(), email: "github@example.invalid"}
	v, _ := vault.New(strings.Repeat("ad", 32))
	h := New(s, Options{PublicOrigin: "https://xingdu.example.invalid", SecureCookies: true, RegistrationEnabled: true, CredentialVault: v, OAuthProviders: map[string]socialauth.Provider{"google": google, "github": github}})
	request := func(method, path, body string, cookies []*http.Cookie, csrf string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", "https://xingdu.example.invalid")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s got %d want %d %s", path, w.Code, want, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private provider") {
			t.Fatal("secret error leaked")
		}
		return w
	}
	start := func(provider, mode string, cookies []*http.Cookie, csrf string) (string, *http.Cookie) {
		t.Helper()
		w := request("POST", "/api/v1/auth/oauth/"+provider+"/start", `{"mode":"`+mode+`"}`, cookies, csrf, 200)
		var out struct {
			Data struct {
				URL string `json:"authorization_url"`
			}
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		u, _ := url.Parse(out.Data.URL)
		state := u.Query().Get("state")
		states = append(states, tokenHash(state))
		c := w.Result().Cookies()[0]
		if c.SameSite != http.SameSiteLaxMode || !c.HttpOnly || !c.Secure {
			t.Fatal("unsafe oauth cookie")
		}
		return state, c
	}
	state, browser := start("google", "login", nil, "")
	callback := "/api/v1/auth/oauth/google/callback?state=" + state + "&code=fixture-code"
	wrong := *browser
	wrong.Value = strings.Repeat("b", 64)
	w := request("GET", callback, "", []*http.Cookie{&wrong}, "", 303)
	if w.Header().Get("Location") != "/login?oauth_error=invalid_state" {
		t.Fatal("browser binding missing")
	}
	w = request("GET", callback, "", []*http.Cookie{browser}, "", 303)
	if w.Header().Get("Location") != "/app" {
		t.Fatal("login failed", w.Header())
	}
	var session *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName {
			session = c
		}
	}
	if session == nil || session.SameSite != http.SameSiteStrictMode {
		t.Fatal("login session cookie missing")
	}
	user, err := s.Session(ctx, tokenHash(session.Value))
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, user.ID)
	w = request("GET", callback, "", []*http.Cookie{browser}, "", 303)
	if w.Header().Get("Location") != "/login?oauth_error=invalid_state" {
		t.Fatal("replay accepted")
	}
	request("POST", "/api/v1/auth/oauth/github/start", `{"mode":"link"}`, []*http.Cookie{session}, "invalid", 403)
	linked, bound := start("github", "link", []*http.Cookie{session}, csrfToken(session.Value))
	// Simulate cross-site callback: the Strict session cookie is deliberately absent.
	w = request("GET", "/api/v1/auth/oauth/github/callback?state="+linked+"&code=fixture-code", "", []*http.Cookie{bound}, "", 303)
	if w.Header().Get("Location") != "/app/security?oauth=linked" {
		t.Fatal("bound-session link failed", w.Header())
	}
	response := request("GET", "/api/v1/account/identities", "", []*http.Cookie{session}, "", 200)
	var methods struct{ Data storage.LoginMethods }
	json.Unmarshal(response.Body.Bytes(), &methods)
	if methods.Data.HasPassword || len(methods.Data.Identities) != 2 {
		t.Fatal("method metadata", response.Body.String())
	}
	if strings.Contains(response.Body.String(), google.subject) {
		t.Fatal("unnecessary provider subject exposed")
	}
	request("DELETE", "/api/v1/account/identities/google", "", []*http.Cookie{session}, csrfToken(session.Value), 204)
	request("DELETE", "/api/v1/account/identities/github", "", []*http.Cookie{session}, csrfToken(session.Value), 409)
	linked, bound = start("google", "link", []*http.Cookie{session}, csrfToken(session.Value))
	if err = s.DeleteSession(ctx, tokenHash(session.Value)); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/api/v1/auth/oauth/google/callback?state="+linked+"&code=fixture-code", "", []*http.Cookie{bound}, "", 303)
	if w.Header().Get("Location") != "/app/security?oauth_error=link_session_expired" {
		t.Fatal("revoked link session accepted", w.Header())
	}
	// Login with a verified email collision must not auto-link the local account.
	localEmail := "local_" + storage.NewID()[:16] + "@example.invalid"
	localID, err := s.Register(ctx, localEmail, "fixture-hash", "Existing")
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, localID)
	google.subject = storage.NewID()
	google.email = localEmail
	state, browser = start("google", "login", nil, "")
	w = request("GET", "/api/v1/auth/oauth/google/callback?state="+state+"&code=fixture-code", "", []*http.Cookie{browser}, "", 303)
	if w.Header().Get("Location") != "/login?oauth_error=account_exists" {
		t.Fatal("email auto-linked", w.Header())
	}
	state, browser = start("google", "login", nil, "")
	w = request("GET", "/api/v1/auth/oauth/google/callback?state="+state+"&error=attacker&error_description=private-provider-secret", "", []*http.Cookie{browser}, "", 303)
	if w.Header().Get("Location") != "/login?oauth_error=access_denied" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("unsafe error redirect")
	}
}
func TestOAuthConfigRequiresCredentialsAndVault(t *testing.T) {
	h := New(fakeStore{}, Options{PublicOrigin: "https://xingdu.example.invalid", OAuthProviders: map[string]socialauth.Provider{"google": &mockOAuthProvider{}}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/auth/config", nil))
	var result struct {
		Data struct {
			Providers map[string]bool `json:"oauth_providers"`
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Data.Providers["google"] || result.Data.Providers["github"] {
		t.Fatal("OAuth enabled without encryption key")
	}
}
