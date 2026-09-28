package httpapi

import (
	"context"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/storage"
)

type AccountStore interface {
	AccountProfile(context.Context, string) (string, error)
	SaveAccountProfile(context.Context, string, string) error
	AccountSessions(context.Context, string, string) ([]storage.AccountSession, error)
	RevokeAccountSession(context.Context, string, string) error
	ChangeAccountPassword(context.Context, string, string, string, string) error
	NewVerifiedSession(context.Context, string, string, string, time.Time) error
}

func (a *api) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/account/profile", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		name, err := a.store.AccountProfile(r.Context(), u.ID)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"display_name": name}})
	}))
	mux.HandleFunc("PUT /api/v1/account/profile", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		var in struct {
			DisplayName string `json:"display_name"`
		}
		if !decode(w, r, &in) {
			return
		}
		in.DisplayName = strings.TrimSpace(in.DisplayName)
		if n := utf8.RuneCountInString(in.DisplayName); n < 1 || n > 64 || strings.IndexFunc(in.DisplayName, unicode.IsControl) >= 0 {
			failure(w, 422, "invalid_profile", "显示名称需为 1–64 个字符，不能包含控制字符")
			return
		}
		if err := a.store.SaveAccountProfile(r.Context(), u.ID, in.DisplayName); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/account/sessions", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
		sessions, err := a.store.AccountSessions(r.Context(), u.ID, tokenHash(token))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": sessions})
	}))
	mux.HandleFunc("DELETE /api/v1/account/sessions/{id}", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !id.Valid("ses", r.PathValue("id")) {
			failure(w, 400, "invalid_id", "记录 ID 无效")
			return
		}
		if err := a.store.RevokeAccountSession(r.Context(), u.ID, r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/account/password", a.require(a.changePassword))
}
func (a *api) changePassword(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
	if !a.attempts.allowLimit("password:"+u.ID, 5) {
		failure(w, 429, "rate_limited", "尝试过于频繁，请稍后再试")
		return
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.CurrentPassword) > 72 || len(in.NewPassword) < 12 || len(in.NewPassword) > 72 || in.NewPassword == in.CurrentPassword {
		failure(w, 422, "invalid_password", "新密码需为 12–72 字节，且不能与当前密码相同")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "请稍后再试")
		return
	}
	credentials, err := a.store.Credentials(r.Context(), u.Username)
	if err != nil {
		storeError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(in.CurrentPassword)) != nil {
		failure(w, 422, "invalid_password", "当前密码不正确")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		failure(w, 503, "unavailable", "暂时无法修改密码")
		return
	}
	if err = a.store.ChangeAccountPassword(r.Context(), u.ID, credentials.PasswordHash, string(hash), tokenHash(token)); err != nil {
		storeError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.WriteHeader(204)
}
