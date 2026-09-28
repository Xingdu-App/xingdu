package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"xingdu.app/xingdu/internal/storage"
)

const cookieName = "xingdu_session"
const sessionLifetime = 24 * time.Hour

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func csrfToken(token string) string { return tokenHash("xingdu-csrf:" + token) }

type attempt struct {
	start time.Time
	count int
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]attempt
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, v := range l.entries {
		if now.Sub(v.start) >= time.Minute {
			delete(l.entries, k)
		}
	}
	v, exists := l.entries[key]
	if !exists {
		if len(l.entries) >= 1024 {
			return false
		}
		v = attempt{start: now}
	}
	if v.count >= 10 {
		return false
	}
	v.count++
	l.entries[key] = v
	return true
}
func (a *api) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allow(ip) {
		w.Header().Set("Retry-After", "60")
		failure(w, 429, "rate_limited", "尝试过于频繁，请稍后再试")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Username) > 32 || len(in.Password) > 72 {
		failure(w, 401, "invalid_credentials", "用户名或密码不正确")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "尝试过于频繁，请稍后再试")
		return
	}
	admin, err := a.store.Credentials(r.Context(), in.Username)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		failure(w, 503, "unavailable", "服务暂时不可用")
		return
	}
	hash := admin.PasswordHash
	if errors.Is(err, storage.ErrNotFound) {
		hash = string(a.dummyHash)
	}
	valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) == nil
	if err != nil || !valid {
		failure(w, 401, "invalid_credentials", "用户名或密码不正确")
		return
	}
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	token := hex.EncodeToString(bytes)
	expires := time.Now().Add(sessionLifetime)
	if err := a.store.NewSession(r.Context(), tokenHash(token), admin.ID, expires); err != nil {
		failure(w, 503, "unavailable", "暂时无法登录，请重试")
		return
	}
	// A login always issues a fresh token, replacing any existing browser session.
	if old, err := r.Cookie(cookieName); err == nil {
		_ = a.store.DeleteSession(r.Context(), tokenHash(old.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds()), Expires: expires})
	reply(w, 200, map[string]any{"data": map[string]string{"id": admin.ID, "username": admin.Username, "csrf_token": csrfToken(token)}})
}
func (a *api) require(next func(http.ResponseWriter, *http.Request, storage.User, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || len(cookie.Value) != 64 {
			failure(w, 401, "unauthenticated", "请先登录")
			return
		}
		admin, err := a.store.Session(r.Context(), tokenHash(cookie.Value))
		if errors.Is(err, storage.ErrNotFound) {
			failure(w, 401, "unauthenticated", "登录已过期，请重新登录")
			return
		}
		if err != nil {
			failure(w, 503, "unavailable", "服务暂时不可用")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrfToken(cookie.Value))) != 1 {
			failure(w, 403, "csrf_failed", "请求验证失败，请刷新页面")
			return
		}
		next(w, r, admin, cookie.Value)
	}
}
func (a *api) logout(w http.ResponseWriter, r *http.Request, _ storage.User, token string) {
	if err := a.store.DeleteSession(r.Context(), tokenHash(token)); err != nil {
		failure(w, 503, "unavailable", "退出失败，请重试")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.WriteHeader(http.StatusNoContent)
}
