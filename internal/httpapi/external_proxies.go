package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

type ExternalProxyStore interface {
	ExternalProxies(context.Context) ([]storage.ExternalProxy, error)
	ExternalProxySecret(context.Context, string) (storage.ExternalProxy, error)
	SaveExternalProxy(context.Context, storage.ExternalProxy, []byte, bool) (storage.ExternalProxy, error)
	DeleteExternalProxy(context.Context, string) error
}
type externalCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func externalAAD(org, proxy string) string { return "external-proxy:" + org + ":" + proxy }
func (a *api) externalProxyRoutes(mux *http.ServeMux) {
	s, ok := a.store.(ExternalProxyStore)
	if !ok {
		return
	}
	mux.HandleFunc("GET /api/v1/external-proxies", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := s.ExternalProxies(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	save := a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "配置加密不可用")
			return
		}
		if !a.attempts.allow("external-proxy:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		create := r.Method == "POST"
		proxy := r.PathValue("proxy")
		if create {
			proxy = storage.NewID("ext")
		} else if !id.Valid("ext", proxy) {
			failure(w, 404, "not_found", "出口不存在")
			return
		}
		var in struct {
			Name     string  `json:"name"`
			Address  string  `json:"address"`
			Port     int     `json:"port"`
			Protocol string  `json:"protocol"`
			Username *string `json:"username"`
			Password *string `json:"password"`
		}
		if !decodeLimit(w, r, &in, 4096) {
			return
		}
		if !protocol.ValidExternalEndpoint(in.Address, in.Port) || (in.Protocol != "socks" && in.Protocol != "http") || (in.Username == nil) != (in.Password == nil) || create && in.Username == nil || in.Username != nil && (!protocol.ValidExternalCredential(*in.Username) || !protocol.ValidExternalCredential(*in.Password)) {
			failure(w, 422, "invalid_external_proxy", "请填写公网 IP、有效端口、SOCKS5 或 HTTP CONNECT，以及完整账号密码")
			return
		}
		p := storage.ExternalProxy{ID: proxy, Name: in.Name, Address: in.Address, Port: in.Port, Protocol: in.Protocol}
		var previous []byte
		if !create {
			old, err := s.ExternalProxySecret(r.Context(), proxy)
			if err != nil {
				storeError(w, err)
				return
			}
			previous = old.Encrypted
		}
		if in.Username != nil {
			plain, _ := json.Marshal(externalCredentials{*in.Username, *in.Password})
			p.Encrypted = a.vault.Seal(plain, externalAAD(storage.TenantOrg(r.Context()), proxy))
			clear(plain)
		}
		out, err := s.SaveExternalProxy(r.Context(), p, previous, create)
		if err != nil {
			storeError(w, err)
			return
		}
		status := 200
		if create {
			status = 201
		}
		reply(w, status, map[string]any{"data": out})
	})
	mux.HandleFunc("POST /api/v1/external-proxies", save)
	mux.HandleFunc("PUT /api/v1/external-proxies/{proxy}", save)
	mux.HandleFunc("DELETE /api/v1/external-proxies/{proxy}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		proxy := r.PathValue("proxy")
		if !id.Valid("ext", proxy) {
			failure(w, 404, "not_found", "出口不存在")
			return
		}
		if err := s.DeleteExternalProxy(r.Context(), proxy); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	}))
}
func (a *api) externalPeer(ctx context.Context, proxy string) (*protocol.Peer, []byte, int, error) {
	if !id.Valid("ext", proxy) {
		return nil, nil, 0, storage.ErrInvalid
	}
	s, ok := a.store.(ExternalProxyStore)
	if !ok || a.vault == nil {
		return nil, nil, 0, storage.ErrConflict
	}
	p, err := s.ExternalProxySecret(ctx, proxy)
	if err != nil {
		return nil, nil, 0, err
	}
	plain, err := a.vault.Open(p.Encrypted, externalAAD(storage.TenantOrg(ctx), proxy))
	if err != nil {
		return nil, nil, 0, storage.ErrConflict
	}
	defer clear(plain)
	var credentials externalCredentials
	if json.Unmarshal(plain, &credentials) != nil {
		return nil, nil, 0, storage.ErrConflict
	}
	peer := &protocol.Peer{ExternalID: p.ID, Address: p.Address, Port: p.Port, Protocol: p.Protocol, Username: credentials.Username, Credential: credentials.Password}
	if peer.Validate() != nil {
		return nil, nil, 0, storage.ErrInvalid
	}
	return peer, p.Encrypted, p.Revision, nil
}
func externalScope(w http.ResponseWriter, r *http.Request) bool {
	if k, ok := r.Context().Value(apiPrincipal{}).(storage.APIKey); ok {
		for _, scope := range k.Scopes {
			if scope == "exits:write" {
				return true
			}
		}
		failure(w, 403, "insufficient_scope", "选择外部出口还需要 exits:write 权限")
		return false
	}
	return true
}
