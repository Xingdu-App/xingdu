package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/storage"
)

type APIKeyStore interface {
	APIKeys(context.Context) ([]storage.APIKey, error)
	CreateAPIKey(context.Context, string, string, string, []string, time.Time) (storage.APIKey, error)
	RevokeAPIKey(context.Context, string) error
	AuthenticateAPIKey(context.Context, string) (storage.APIKey, error)
}
type apiPrincipal struct{}

func apiScope(method, path string) string {
	p := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(p) >= 1 && p[0] == "subscriptions" {
		if len(p) == 1 {
			if method == "GET" {
				return "subscriptions:read"
			}
			if method == "POST" {
				return "subscriptions:write"
			}
		}
		if len(p) >= 2 && id.Valid("sub", p[1]) {
			if len(p) == 2 {
				if method == "GET" {
					return "subscriptions:read"
				}
				if method == "PUT" || method == "PATCH" || method == "DELETE" {
					return "subscriptions:write"
				}
			}
			if len(p) == 3 {
				if p[2] == "config" && method == "GET" || p[2] == "preview" && method == "POST" {
					return "subscriptions:export"
				}
			}
		}
		return ""
	}
	if path == "/api/v1/nodes" && method == "GET" {
		return "nodes:read"
	}
	if len(p) == 1 && p[0] == "hosts" {
		if method == "GET" {
			return "hosts:read"
		}
		if method == "POST" {
			return "hosts:write"
		}
	}
	if len(p) < 2 || p[0] != "hosts" || !id.Valid("srv", p[1]) {
		return ""
	}
	if len(p) == 2 && (method == "PUT" || method == "DELETE") {
		return "hosts:write"
	}
	if len(p) == 3 && p[2] == "deployments" {
		if method == "GET" {
			return "nodes:read"
		}
		if method == "POST" {
			return "nodes:write"
		}
	}
	if len(p) >= 4 && p[2] == "deployments" {
		if len(p) == 4 && p[3] == "preflight" && method == "POST" {
			return "nodes:write"
		}
		if !id.Valid("node", p[3]) {
			return ""
		}
		if len(p) == 4 && (method == "DELETE" || method == "PUT") {
			return "nodes:write"
		}
		if len(p) == 5 && method == "POST" {
			if p[4] == "probe" {
				return "nodes:probe"
			}
			if p[4] == "restart" {
				return "nodes:write"
			}
			if p[4] == "connection" {
				return "nodes:credentials"
			}
		}
	}
	return ""
}
func (a *api) apiKeyRequest(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	scope := apiScope(r.Method, r.URL.Path)
	if scope == "" || r.Header.Get("Cookie") != "" || r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		failure(w, 403, "api_key_forbidden", "API 密钥不能访问此接口或混用浏览器身份")
		return r, false
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer xd_key_") || len(auth) != len("Bearer xd_key_")+64 {
		failure(w, 401, "invalid_api_key", "API 密钥无效或已过期")
		return r, false
	}
	secret := strings.TrimPrefix(auth, "Bearer ")
	if _, err := hex.DecodeString(strings.TrimPrefix(secret, "xd_key_")); err != nil {
		failure(w, 401, "invalid_api_key", "API 密钥无效或已过期")
		return r, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	k, err := a.store.AuthenticateAPIKey(ctx, tokenHash(secret))
	if err != nil {
		if errors.Is(err, storage.ErrForbidden) || errors.Is(err, storage.ErrNotFound) {
			failure(w, 401, "invalid_api_key", "API 密钥无效或已过期")
		} else {
			storeError(w, err)
		}
		return r, false
	}
	if org := r.Header.Get("X-Xingdu-Organization"); org != "" && org != k.OrganizationID {
		failure(w, 403, "api_key_forbidden", "API 密钥不属于此组织")
		return r, false
	}
	allowed := false
	for _, s := range k.Scopes {
		if s == scope {
			allowed = true
		}
	}
	if !allowed {
		failure(w, 403, "insufficient_scope", "API 密钥缺少此操作的权限")
		return r, false
	}
	if !a.attempts.allowLimit("api-key:"+k.ID, 120) {
		w.Header().Set("Retry-After", "60")
		failure(w, 429, "rate_limited", "请求过于频繁")
		return r, false
	}
	if r.Method != "GET" {
		content, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || content != "application/json" {
			failure(w, 415, "invalid_content_type", "请使用 JSON 请求")
			return r, false
		}
	}
	return r.WithContext(context.WithValue(storage.WithAPIKeyTenant(r.Context(), k), apiPrincipal{}, k)), true
}
func (a *api) apiKeyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/api-keys", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		keys, err := a.store.APIKeys(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": keys})
	}))
	mux.HandleFunc("POST /api/v1/api-keys", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		var in struct {
			Name          string   `json:"name"`
			Scopes        []string `json:"scopes"`
			ExpiresInDays int      `json:"expires_in_days"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.ExpiresInDays < 1 || in.ExpiresInDays > 365 || !storage.ValidAPIKeyScopes(in.Scopes) {
			failure(w, 400, "invalid_input", "请选择权限及 1–365 天的有效期")
			return
		}
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			failure(w, 503, "unavailable", "无法生成密钥")
			return
		}
		secret := "xd_key_" + hex.EncodeToString(random[:])
		k, err := a.store.CreateAPIKey(r.Context(), in.Name, tokenHash(secret), secret[:15], in.Scopes, time.Now().Add(time.Duration(in.ExpiresInDays)*24*time.Hour))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 201, map[string]any{"data": map[string]any{"key": k, "secret": secret}})
	}))
	mux.HandleFunc("DELETE /api/v1/api-keys/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !id.Valid("key", r.PathValue("id")) {
			failure(w, 400, "invalid_id", "无效的密钥 ID")
			return
		}
		if err := a.store.RevokeAPIKey(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
}
