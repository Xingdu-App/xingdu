package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"regexp"
	"strings"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/storage"
)

type TenantStore interface {
	Register(context.Context, string, string, string) (string, error)
	Organizations(context.Context, string) ([]storage.Organization, error)
	CreateOrganization(context.Context, string, string) (storage.Organization, error)
	Members(context.Context) ([]storage.Member, error)
	ChangeMember(context.Context, string, string, bool) error
	CreateInvitation(context.Context, string, string) (storage.Invitation, error)
	Invitations(context.Context) ([]storage.Invitation, error)
	RevokeInvitation(context.Context, string) error
	AcceptInvitation(context.Context, string, string) (string, error)
}

func (a *api) register(w http.ResponseWriter, r *http.Request) {
	if !a.registration {
		failure(w, 403, "registration_disabled", "注册未开放")
		return
	}
	if !a.attempts.allow("register") {
		failure(w, 429, "rate_limited", "注册过于频繁，请稍后再试")
		return
	}
	var in struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		Organization string `json:"organization"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`).MatchString(in.Username) || len(in.Password) < 12 || len(in.Password) > 72 {
		failure(w, 422, "invalid_account", "用户名需为 3–32 位字母、数字、下划线或连字符，密码需为 12–72 字节")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "请稍后再试")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		storeError(w, err)
		return
	}
	_, err = a.store.Register(r.Context(), in.Username, string(hash), in.Organization)
	if err != nil {
		storeError(w, err)
		return
	}
	reply(w, 201, map[string]any{"data": map[string]bool{"registered": true}})
}
func (a *api) tenant(next func(http.ResponseWriter, *http.Request, storage.User, string)) http.HandlerFunc {
	return a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
		org := r.Header.Get("X-Xingdu-Organization")
		if !hosts.IDPattern.MatchString(org) {
			failure(w, 400, "organization_required", "请选择组织")
			return
		}
		next(w, r.WithContext(storage.WithTenant(r.Context(), u.ID, org)), u, token)
	})
}
func (a *api) tenantRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/config", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"data": map[string]bool{"registration_enabled": a.registration}})
	})
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("GET /api/v1/organizations", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		out, err := a.store.Organizations(r.Context(), u.ID)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("POST /api/v1/organizations", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		var in struct {
			Name string `json:"name"`
		}
		if !decode(w, r, &in) {
			return
		}
		out, err := a.store.CreateOrganization(r.Context(), u.ID, in.Name)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 201, map[string]any{"data": out})
	}))
	mux.HandleFunc("GET /api/v1/members", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := a.store.Members(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("PUT /api/v1/members/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		var in struct {
			Role string `json:"role"`
		}
		if !decode(w, r, &in) {
			return
		}
		if err := a.store.ChangeMember(r.Context(), r.PathValue("id"), in.Role, false); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("DELETE /api/v1/members/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.ChangeMember(r.Context(), r.PathValue("id"), "", true); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("GET /api/v1/invitations", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := a.store.Invitations(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("POST /api/v1/invitations", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		var in struct {
			Role string `json:"role"`
		}
		if !decode(w, r, &in) {
			return
		}
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		token := hex.EncodeToString(b)
		out, err := a.store.CreateInvitation(r.Context(), in.Role, tokenHash(token))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 201, map[string]any{"data": map[string]any{"invitation": out, "url": a.origin + "/app#invite=" + token}})
	}))
	mux.HandleFunc("DELETE /api/v1/invitations/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.RevokeInvitation(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/invitations/accept", a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		var in struct {
			Token string `json:"token"`
		}
		if !decode(w, r, &in) {
			return
		}
		if len(in.Token) != 64 {
			failure(w, 422, "invalid_invitation", "邀请链接无效")
			return
		}
		org, err := a.store.AcceptInvitation(r.Context(), u.ID, tokenHash(in.Token))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"organization_id": org}})
	}))
}
