package socialauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func signedToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	b, _ := json.Marshal(claims)
	input := h + "." + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func TestGoogleVerifiedClaimsAndPKCE(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	providers := New(Config{Origin: "https://xingdu.example.invalid", GoogleClientID: "test-client", GoogleClientSecret: "test-secret"})
	p := providers["google"].(*googleProvider)
	p.verifier = oidc.NewVerifier("https://accounts.google.com", &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "test-client", SupportedSigningAlgs: []string{"RS256"}})
	verifier := oauth2.GenerateVerifier()
	auth, _ := url.Parse(p.AuthorizationURL("state", verifier, "nonce"))
	if auth.Query().Get("code_challenge_method") != "S256" || auth.Query().Get("nonce") != "nonce" || auth.Query().Get("state") != "state" || auth.Query().Get("redirect_uri") != "https://xingdu.example.invalid/api/v1/auth/oauth/google/callback" {
		t.Fatal("unsafe authorization url")
	}
	cases := []struct {
		name     string
		change   func(map[string]any)
		valid    bool
		wrongKey bool
	}{
		{name: "valid", valid: true}, {name: "issuer", change: func(c map[string]any) { c["iss"] = "https://attacker.invalid" }},
		{name: "audience", change: func(c map[string]any) { c["aud"] = "another-client" }},
		{name: "expired", change: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "nonce", change: func(c map[string]any) { c["nonce"] = "another-nonce" }},
		{name: "unverified", change: func(c map[string]any) { c["email_verified"] = false }},
		{name: "subject", change: func(c map[string]any) { c["sub"] = "" }},
		{name: "azp", change: func(c map[string]any) { c["azp"] = "another-client" }},
		{name: "multi-aud-no-azp", change: func(c map[string]any) { c["aud"] = []string{"test-client", "other"} }},
		{name: "multi-aud-valid-azp", change: func(c map[string]any) { c["aud"] = []string{"test-client", "other"}; c["azp"] = "test-client" }, valid: true},
		{name: "signature", wrongKey: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{"iss": "https://accounts.google.com", "sub": "stable-subject", "aud": "test-client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": "nonce", "email": "Google@Example.Invalid", "email_verified": true}
			if tc.change != nil {
				tc.change(claims)
			}
			signing := key
			if tc.wrongKey {
				signing = other
			}
			raw := signedToken(t, signing, claims)
			p.client = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
				r.ParseForm()
				if r.URL.String() != "https://oauth2.googleapis.com/token" || r.Form.Get("code_verifier") != verifier || r.Form.Get("client_secret") != "test-secret" {
					t.Fatal("token exchange missing PKCE/client secret")
				}
				body, _ := json.Marshal(map[string]any{"access_token": "memory-only", "token_type": "Bearer", "id_token": raw})
				return jsonResponse(string(body)), nil
			})}
			id, e := p.Verify(context.Background(), "one-time-code", verifier, "nonce")
			if tc.valid {
				if e != nil || id.Subject != "stable-subject" || id.Email != "google@example.invalid" {
					t.Fatal(id, e)
				}
			} else if e != ErrProvider {
				t.Fatal("invalid claims accepted", id, e)
			}
		})
	}
}
func TestGitHubPrimaryVerifiedEmailAndStableID(t *testing.T) {
	p := New(Config{Origin: "https://xingdu.example.invalid", GitHubClientID: "test-client", GitHubClientSecret: "test-secret"})["github"].(*githubProvider)
	verifier := oauth2.GenerateVerifier()
	auth, _ := url.Parse(p.AuthorizationURL("state", verifier, "ignored"))
	if auth.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("PKCE absent")
	}
	for _, tc := range []struct {
		emails string
		valid  bool
	}{{`[{"email":"Primary@Example.Invalid","primary":true,"verified":true}]`, true}, {`[{"email":"private@example.invalid","primary":true,"verified":false},{"email":"other@example.invalid","primary":false,"verified":true}]`, false}, {`[]`, false}} {
		p.client = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/login/oauth/access_token":
				r.ParseForm()
				if r.Form.Get("code_verifier") != verifier {
					t.Fatal("PKCE missing")
				}
				return jsonResponse(`{"access_token":"memory-only","token_type":"bearer"}`), nil
			case "/user":
				if r.Header.Get("Authorization") != "Bearer memory-only" {
					t.Fatal("missing token")
				}
				return jsonResponse(`{"id":123456789,"login":"mutable-name","email":"untrusted@example.invalid"}`), nil
			case "/user/emails":
				return jsonResponse(tc.emails), nil
			default:
				t.Fatal("unexpected URL")
				return nil, nil
			}
		})}
		id, err := p.Verify(context.Background(), "code", verifier, "")
		if tc.valid {
			if err != nil || id.Subject != "123456789" || id.Email != "primary@example.invalid" {
				t.Fatal(id, err)
			}
		} else if err != ErrProvider {
			t.Fatal("unverified email accepted", id, err)
		}
	}
}
func TestDisabledProviderConfiguration(t *testing.T) {
	for _, origin := range []string{"https://xingdu.example.invalid", "http://untrusted.example.invalid"} {
		p := New(Config{Origin: origin, GoogleClientID: "PLACEHOLDER_CLIENT_ID", GoogleClientSecret: "PLACEHOLDER_SECRET", GitHubClientID: "", GitHubClientSecret: ""})
		if len(p) != 0 {
			t.Fatal("placeholder provider enabled")
		}
	}
}
