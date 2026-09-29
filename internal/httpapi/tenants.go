package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/id"
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

func (a *api) tenant(next func(http.ResponseWriter, *http.Request, storage.User, string)) http.HandlerFunc {
	browser := a.require(func(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
		org := r.Header.Get("X-Xingdu-Organization")
		if !id.Valid("org", org) {
			failure(w, 400, "organization_required", "请选择组织")
			return
		}
		next(w, r.WithContext(storage.WithTenant(r.Context(), u.ID, org)), u, token)
	})
	return func(w http.ResponseWriter, r *http.Request) {
		if k, ok := r.Context().Value(apiPrincipal{}).(storage.APIKey); ok {
			next(w, r, storage.User{ID: k.CreatedBy}, "")
			return
		}
		browser(w, r)
	}
}
func (a *api) tenantRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/config", func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, map[string]any{"data": map[string]any{"mode": a.mode, "multi_organization": a.mode != "self_hosted", "registration_enabled": a.registration, "email_verification_required": true, "email_delivery_configured": a.emailReady(), "oauth_providers": a.oauthConfig()}})
	})
	mux.HandleFunc("POST /api/v1/auth/password-recovery", a.requestPasswordRecovery)
	mux.HandleFunc("POST /api/v1/auth/password-recovery/complete", a.completePasswordRecovery)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/register/verify", a.verifyRegistration)
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
		if a.mode == "self_hosted" {
			orgs, err := a.store.Organizations(r.Context(), u.ID)
			if err != nil {
				storeError(w, err)
				return
			}
			if len(orgs) > 0 {
				failure(w, 409, "single_organization", "自部署模式仅支持一个组织")
				return
			}
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
	mux.HandleFunc("POST /api/v1/invitations", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		var in struct {
			Role  string `json:"role"`
			Email string `json:"email"`
		}
		if !decode(w, r, &in) {
			return
		}
		var mailer emailverification.MessageSender
		if in.Email != "" {
			var valid bool
			in.Email, valid = registrationEmail(in.Email)
			if !valid {
				failure(w, 422, "invalid_email", "请输入有效邮箱")
				return
			}
			mailer, _ = a.emailSender.(emailverification.MessageSender)
			if mailer == nil || !mailer.Configured() {
				failure(w, 503, "email_unavailable", "邮件服务暂不可用")
				return
			}
		}
		if !a.attempts.allowLimit("invitation:"+u.ID, 10) {
			failure(w, 429, "rate_limited", "邀请过于频繁，请稍后再试")
			return
		}
		orgName := "Xingdu"
		if in.Email != "" {
			orgs, err := a.store.Organizations(r.Context(), u.ID)
			if err != nil {
				storeError(w, err)
				return
			}
			for _, o := range orgs {
				if o.ID == storage.TenantOrg(r.Context()) {
					orgName = o.Name
				}
			}
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			failure(w, 503, "unavailable", "服务暂不可用")
			return
		}
		token := hex.EncodeToString(b)
		out, err := a.store.CreateInvitation(r.Context(), in.Role, tokenHash(token))
		if err != nil {
			storeError(w, err)
			return
		}
		link := a.origin + "/app#invite=" + token
		status := "not_requested"
		if mailer != nil {
			status = "accepted"
			if err := mailer.SendMessage(r.Context(), in.Email, emailverification.Message{Kind: "invitation", URL: link, Organization: orgName, Role: in.Role}, "xingdu-invitation/"+out.ID); err != nil {
				status = "failed"
			}
		}
		// Preserve the created link even when delivery fails; never report creation as failed.
		reply(w, 201, map[string]any{"data": map[string]any{"invitation": out, "url": link, "email_delivery": status}})
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
