// Package socialauth verifies provider identities. Provider tokens and secrets
// remain in memory and are never returned to the browser or stored as identity data.
package socialauth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var ErrProvider = errors.New("provider identity verification failed")

type Identity struct{ Subject, Email string }
type Provider interface {
	AuthorizationURL(state, verifier, nonce string) string
	Verify(context.Context, string, string, string) (Identity, error)
}
type Config struct{ GoogleClientID, GoogleClientSecret, GitHubClientID, GitHubClientSecret, Origin string }

func credential(value string) bool {
	if len(strings.TrimSpace(value)) < 4 {
		return false
	}
	lower := strings.ToLower(value)
	for _, v := range []string{"placeholder", "replace_with", "your_client", "example", "xxxxxxxx"} {
		if strings.Contains(lower, v) {
			return false
		}
	}
	return !strings.ContainsAny(value, "\r\n")
}
func New(cfg Config) map[string]Provider {
	out := map[string]Provider{}
	origin, err := url.Parse(cfg.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || strings.Trim(origin.Path, "/") != "" || (origin.Scheme != "https" && (origin.Scheme != "http" || (origin.Hostname() != "localhost" && origin.Hostname() != "127.0.0.1" && origin.Hostname() != "::1"))) {
		return out
	}
	base := strings.TrimSuffix(cfg.Origin, "/")
	client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if credential(cfg.GoogleClientID) && credential(cfg.GoogleClientSecret) {
		config := oauth2.Config{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret, RedirectURL: base + "/api/v1/auth/oauth/google/callback", Scopes: []string{"openid", "email"}, Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams}}
		ctx := oidc.ClientContext(context.Background(), client)
		out["google"] = &googleProvider{config: config, client: client, verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(ctx, "https://www.googleapis.com/oauth2/v3/certs"), &oidc.Config{ClientID: cfg.GoogleClientID, SupportedSigningAlgs: []string{"RS256"}})}
	}
	if credential(cfg.GitHubClientID) && credential(cfg.GitHubClientSecret) {
		out["github"] = &githubProvider{config: oauth2.Config{ClientID: cfg.GitHubClientID, ClientSecret: cfg.GitHubClientSecret, RedirectURL: base + "/api/v1/auth/oauth/github/callback", Scopes: []string{"read:user", "user:email"}, Endpoint: oauth2.Endpoint{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", AuthStyle: oauth2.AuthStyleInParams}}, client: client}
	}
	return out
}
func VerifiedEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	a, e := mail.ParseAddress(value)
	if e != nil || a.Address != value || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	for _, r := range value {
		if r > 127 {
			return "", false
		}
	}
	return value, true
}

type googleProvider struct {
	config   oauth2.Config
	client   *http.Client
	verifier *oidc.IDTokenVerifier
}

func (p *googleProvider) AuthorizationURL(state, verifier, nonce string) string {
	return p.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce), oauth2.SetAuthURLParam("prompt", "select_account"))
}
func (p *googleProvider) Verify(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, ErrProvider
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, ErrProvider
	}
	verified, err := p.verifier.Verify(ctx, raw)
	if err != nil || len(verified.Subject) == 0 || len(verified.Subject) > 255 || subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(nonce)) != 1 {
		return Identity{}, ErrProvider
	}
	var claims struct {
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
		AuthorizedParty string `json:"azp"`
	}
	if verified.Claims(&claims) != nil || !claims.EmailVerified || (len(verified.Audience) > 1 && claims.AuthorizedParty != p.config.ClientID) || (claims.AuthorizedParty != "" && claims.AuthorizedParty != p.config.ClientID) {
		return Identity{}, ErrProvider
	}
	email, valid := VerifiedEmail(claims.Email)
	if !valid {
		return Identity{}, ErrProvider
	}
	return Identity{Subject: verified.Subject, Email: email}, nil
}

type githubProvider struct {
	config oauth2.Config
	client *http.Client
}

func (p *githubProvider) AuthorizationURL(state, verifier, _ string) string {
	return p.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}
func (p *githubProvider) Verify(ctx context.Context, code, verifier, _ string) (Identity, error) {
	token, err := p.config.Exchange(context.WithValue(ctx, oauth2.HTTPClient, p.client), code, oauth2.VerifierOption(verifier))
	if err != nil || token.AccessToken == "" || !strings.EqualFold(token.Type(), "Bearer") {
		return Identity{}, ErrProvider
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if p.get(ctx, "https://api.github.com/user", token.AccessToken, &user) != nil || user.ID <= 0 {
		return Identity{}, ErrProvider
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if p.get(ctx, "https://api.github.com/user/emails?per_page=100", token.AccessToken, &emails) != nil {
		return Identity{}, ErrProvider
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			email, valid := VerifiedEmail(e.Email)
			if valid {
				return Identity{Subject: strconv.FormatInt(user.ID, 10), Email: email}, nil
			}
		}
	}
	return Identity{}, ErrProvider
}
func (p *githubProvider) get(ctx context.Context, path, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return ErrProvider
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := p.client.Do(req)
	if err != nil {
		return ErrProvider
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return ErrProvider
	}
	if json.NewDecoder(io.LimitReader(res.Body, 256*1024)).Decode(out) != nil {
		return ErrProvider
	}
	return nil
}
