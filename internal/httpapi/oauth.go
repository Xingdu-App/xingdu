package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/oauth2"
	"net"
	"net/http"
	"regexp"
	"time"
	"xingdu.app/xingdu/internal/socialauth"
	"xingdu.app/xingdu/internal/storage"
)

type OAuthStore interface {
	SaveOAuthState(context.Context, storage.OAuthState) error
	ConsumeOAuthState(context.Context, string, string, string) (storage.OAuthState, error)
	CompleteOAuth(context.Context, storage.OAuthState, string, string, string, time.Time, bool) (storage.User, error)
	LoginMethods(context.Context, string) (storage.LoginMethods, error)
	UnlinkOAuthIdentity(context.Context, string, string, string, []string) error
}

func (a *api) oauthEnabled(provider string) bool {
	return a.vault != nil && a.oauthProviders[provider] != nil
}
func (a *api) oauthConfig() map[string]bool {
	return map[string]bool{"google": a.oauthEnabled("google"), "github": a.oauthEnabled("github")}
}
func oauthAAD(state storage.OAuthState) string {
	return "xingdu-oauth-state-v1:" + state.Hash + ":" + state.BrowserHash + ":" + state.Provider + ":" + state.Mode + ":" + state.UserID + ":" + state.SessionHash
}
func randomOAuthToken() string {
	data := make([]byte, 32)
	rand.Read(data)
	return hex.EncodeToString(data)
}
func (a *api) oauthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/oauth/{provider}/start", a.startOAuth)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/callback", a.oauthCallback)
	mux.HandleFunc("GET /api/v1/account/identities", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		methods, err := a.store.LoginMethods(r.Context(), u.ID)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": methods})
	}))
	mux.HandleFunc("DELETE /api/v1/account/identities/{provider}", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
		provider := r.PathValue("provider")
		if provider != "google" && provider != "github" {
			failure(w, 404, "not_found", "登录方式不存在")
			return
		}
		enabled := []string{}
		for _, name := range []string{"google", "github"} {
			if a.oauthEnabled(name) {
				enabled = append(enabled, name)
			}
		}
		if err := a.store.UnlinkOAuthIdentity(r.Context(), u.ID, provider, tokenHash(token), enabled); err != nil {
			if errors.Is(err, storage.ErrLastLoginMethod) {
				failure(w, 409, "last_login_method", "至少保留一种可用的登录方式")
			} else {
				storeError(w, err)
			}
			return
		}
		w.WriteHeader(204)
	}))
}
func (a *api) startOAuth(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if !a.oauthEnabled(provider) {
		failure(w, 503, "provider_disabled", "此登录方式尚未配置")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allowLimit("oauth-start:"+ip, 20) {
		failure(w, 429, "rate_limited", "请求过于频繁，请稍后再试")
		return
	}
	var in struct {
		Mode string `json:"mode"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Mode != "login" && in.Mode != "link" {
		failure(w, 422, "invalid_oauth_mode", "请选择登录或绑定")
		return
	}
	start := func(w http.ResponseWriter, r *http.Request, u storage.User, session string) {
		stateToken, browser := randomOAuthToken(), randomOAuthToken()
		state := storage.OAuthState{Hash: tokenHash(stateToken), BrowserHash: tokenHash(browser), Provider: provider, Mode: in.Mode}
		if in.Mode == "link" {
			state.UserID = u.ID
			state.SessionHash = tokenHash(session)
		}
		secret := struct {
			Verifier string `json:"verifier"`
			Nonce    string `json:"nonce"`
		}{oauth2.GenerateVerifier(), randomOAuthToken()}
		raw, _ := json.Marshal(secret)
		state.Encrypted = a.vault.Seal(raw, oauthAAD(state))
		clear(raw)
		if err := a.store.SaveOAuthState(r.Context(), state); err != nil {
			storeError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "xingdu_oauth_" + provider, Value: browser, Path: "/api/v1/auth/oauth/" + provider, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: 600, Expires: time.Now().Add(10 * time.Minute)})
		reply(w, 200, map[string]any{"data": map[string]string{"authorization_url": a.oauthProviders[provider].AuthorizationURL(stateToken, secret.Verifier, secret.Nonce)}})
	}
	if in.Mode == "link" {
		a.require(start)(w, r)
		return
	}
	start(w, r, storage.User{}, "")
}

// Only constant application paths and error codes are ever redirected. Provider
// error descriptions, authorization codes and tokens never reach application URLs.
func oauthReturn(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func (a *api) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	w.Header().Set("Referrer-Policy", "no-referrer")
	provider := r.PathValue("provider")
	if !a.oauthEnabled(provider) {
		oauthReturn(w, r, "/login?oauth_error=provider_disabled")
		return
	}
	stateToken := r.URL.Query().Get("state")
	cookie, err := r.Cookie("xingdu_oauth_" + provider)
	if err != nil || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(stateToken) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(cookie.Value) {
		oauthReturn(w, r, "/login?oauth_error=invalid_state")
		return
	}
	state, err := a.store.ConsumeOAuthState(r.Context(), tokenHash(stateToken), tokenHash(cookie.Value), provider)
	if err != nil {
		oauthReturn(w, r, "/login?oauth_error=invalid_state")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "xingdu_oauth_" + provider, Value: "", Path: "/api/v1/auth/oauth/" + provider, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	failurePath := "/login?oauth_error="
	if state.Mode == "link" {
		failurePath = "/app/security?oauth_error="
	}
	fail := func(code string) { oauthReturn(w, r, failurePath+code) }
	if r.URL.Query().Get("error") != "" {
		fail("access_denied")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 4096 {
		fail("provider_failed")
		return
	}
	raw, err := a.vault.Open(state.Encrypted, oauthAAD(state))
	if err != nil {
		fail("invalid_state")
		return
	}
	defer clear(raw)
	var secret struct {
		Verifier string `json:"verifier"`
		Nonce    string `json:"nonce"`
	}
	if json.Unmarshal(raw, &secret) != nil || len(secret.Verifier) < 43 || len(secret.Nonce) != 64 {
		fail("invalid_state")
		return
	}
	identity, err := a.oauthProviders[provider].Verify(r.Context(), code, secret.Verifier, secret.Nonce)
	if err != nil {
		fail("provider_failed")
		return
	}
	// Enforce identity invariants at the integration boundary as well.
	email, valid := socialauth.VerifiedEmail(identity.Email)
	if !valid || identity.Subject == "" || len(identity.Subject) > 255 {
		fail("provider_failed")
		return
	}
	sessionToken := randomOAuthToken()
	expires := time.Now().Add(sessionLifetime)
	_, err = a.store.CompleteOAuth(r.Context(), state, identity.Subject, email, tokenHash(sessionToken), expires, a.registration)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrOAuthEmailExists):
			fail("account_exists")
		case errors.Is(err, storage.ErrOAuthIdentityInUse):
			fail("identity_in_use")
		case errors.Is(err, storage.ErrOAuthRegistrationDisabled):
			fail("registration_disabled")
		case errors.Is(err, storage.ErrOAuthLinkSession):
			fail("link_session_expired")
		default:
			fail("unavailable")
		}
		return
	}
	if state.Mode == "link" {
		oauthReturn(w, r, "/app/security?oauth=linked")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: sessionToken, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds()), Expires: expires})
	// /app is a public SPA shell; its subsequent same-origin session request sends
	// the Strict cookie even when the provider's cross-site redirect did not.
	oauthReturn(w, r, "/app")
}
