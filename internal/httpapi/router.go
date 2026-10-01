package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/bootstrap"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/socialauth"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type Store interface {
	APIKeyStore
	BillingStore
	OAuthStore
	EmailRegistrationStore
	AccountStore
	OperationsStore
	OwnershipStore
	AvatarStore
	TenantStore
	MachineStore
	DeploymentStore
	SubscriptionStore
	Ready(context.Context) error
	Hosts(context.Context) ([]storage.Host, error)
	Credentials(context.Context, string) (storage.User, error)
	Session(context.Context, string) (storage.User, error)
	NewSession(context.Context, string, string, time.Time) error
	DeleteSession(context.Context, string) error
	CreateHost(context.Context, hosts.Input) (storage.Host, error)
	UpdateHost(context.Context, string, hosts.Input) (storage.Host, error)
	DeleteHost(context.Context, string) error
}
type Options struct {
	Certificates        certificates.ManagedConfig
	Billing             billing.Gateway
	Mode                string
	BillingCloud        bool
	BillingPremium      bool
	BillingTest         bool
	OAuthProviders      map[string]socialauth.Provider
	EmailSender         emailverification.Sender
	PublicOrigin        string
	SecureCookies       bool
	RegistrationEnabled bool
	AgentOrigin         string
	ArtifactDir         string
	CredentialVault     *vault.Vault
	SSHConnector        *bootstrap.Connector
}
type api struct {
	billing         billing.Gateway
	billingCloud    bool
	billingPremium  bool
	billingTest     bool
	oauthProviders  map[string]socialauth.Provider
	emailSender     emailverification.Sender
	agentOrigin     string
	artifacts       string
	vault           *vault.Vault
	connector       *bootstrap.Connector
	connectionSlots chan struct{}
	mode            string
	registration    bool
	store           Store
	origin          string
	secure          bool
	dummyHash       []byte
	attempts        limiter
	hashSlots       chan struct{}
}

func New(store Store, opts Options) http.Handler {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("non-authenticating-dummy-password"), bcrypt.DefaultCost)
	a := &api{mode: opts.Mode, oauthProviders: opts.OAuthProviders, emailSender: opts.EmailSender, registration: opts.RegistrationEnabled, store: store, origin: opts.PublicOrigin, secure: opts.SecureCookies, dummyHash: dummy, attempts: limiter{entries: make(map[string]attempt)}, hashSlots: make(chan struct{}, 4)}
	if opts.AgentOrigin == "" {
		opts.AgentOrigin = opts.PublicOrigin
	}
	a.agentOrigin = opts.AgentOrigin
	a.artifacts = opts.ArtifactDir
	a.vault = opts.CredentialVault
	a.connector = opts.SSHConnector
	if a.connector == nil {
		a.connector = &bootstrap.Connector{}
	}
	a.connectionSlots = make(chan struct{}, 4)
	a.billing = opts.Billing
	a.billingCloud = opts.BillingCloud
	a.billingPremium = opts.BillingPremium
	a.billingTest = opts.BillingTest
	mux := http.NewServeMux()
	a.billingRoutes(mux)
	a.machineRoutes(mux)
	a.deploymentRoutes(mux)
	a.subscriptionRoutes(mux)
	a.certificateRoutes(mux, opts.Certificates)
	a.avatarRoutes(mux)
	a.tenantRoutes(mux)
	a.apiKeyRoutes(mux)
	a.oauthRoutes(mux)
	a.accountRoutes(mux)
	a.operationsRoutes(mux)
	mux.HandleFunc("POST /api/v1/organization/ownership", a.tenant(a.transferOwnership))
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ready(r.Context()); err != nil {
			reply(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		reply(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/logout", a.require(a.logout))
	mux.HandleFunc("GET /api/v1/auth/session", a.require(func(w http.ResponseWriter, r *http.Request, admin storage.User, token string) {
		reply(w, 200, map[string]any{"data": map[string]string{"id": admin.ID, "username": admin.Username, "csrf_token": csrfToken(token)}})
	}))
	mux.HandleFunc("GET /api/v1/system", a.require(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		reply(w, 200, map[string]any{"data": map[string]any{"name": "Xingdu", "version": "0.7.0-dev", "stage": "protocol-deployment", "database_ready": store.Ready(r.Context()) == nil, "capabilities": map[string]bool{"host_inventory": true, "host_enrollment": true, "deployment": true, "subscription_export": true}}})
	}))
	mux.HandleFunc("GET /api/v1/hosts", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		result, err := store.Hosts(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": result})
	}))
	mux.HandleFunc("POST /api/v1/hosts", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		var in hosts.Input
		if !decode(w, r, &in) {
			return
		}
		if err := in.Validate(); err != nil {
			failure(w, 422, "invalid_host", err.Error())
			return
		}
		host, err := store.CreateHost(r.Context(), in)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 201, map[string]any{"data": host})
	}))
	mux.HandleFunc("PUT /api/v1/hosts/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		var in hosts.Input
		if !decode(w, r, &in) {
			return
		}
		if err := in.Validate(); err != nil {
			failure(w, 422, "invalid_host", err.Error())
			return
		}
		host, err := store.UpdateHost(r.Context(), r.PathValue("id"), in)
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": host})
	}))
	mux.HandleFunc("DELETE /api/v1/hosts/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := store.DeleteHost(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		machineRequest := r.URL.Path == "/api/v1/agent/update/applied" || r.URL.Path == "/api/v1/agent/update/claim" || r.URL.Path == "/api/v1/agent/update/check" || r.URL.Path == "/api/v1/agent/update/failed" || r.URL.Path == "/api/v1/agent/enroll" || r.URL.Path == "/api/v1/agent/heartbeat" || r.URL.Path == "/api/v1/agent/deployments/claim" || r.URL.Path == "/api/v1/agent/deployments/result" || r.URL.Path == "/api/v1/agent/deployments/status"
		// Build artifacts are public. Existing Agents send their machine bearer
		// when fetching a runtime; it must not be interpreted as a user API key.
		artifactRequest := (r.Method == "GET" || r.Method == "HEAD") && (r.URL.Path == "/api/v1/agent/runtime/trusttunnel/amd64" || r.URL.Path == "/api/v1/agent/runtime/trusttunnel/arm64" || r.URL.Path == "/api/v1/agent/runtime/amd64" || r.URL.Path == "/api/v1/agent/runtime/arm64" || r.URL.Path == "/api/v1/agent/download/amd64" || r.URL.Path == "/api/v1/agent/download/arm64")
		keyRequest := !machineRequest && !artifactRequest && r.Header.Get("Authorization") != ""
		if keyRequest {
			var ok bool
			r, ok = a.apiKeyRequest(w, r)
			if !ok {
				return
			}
		}
		if machineRequest {
			if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" {
				failure(w, 403, "agent_only", "机器接口不接受浏览器身份")
				return
			}
			if contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); r.Method == "POST" && (err != nil || contentType != "application/json") {
				failure(w, 415, "invalid_content_type", "请使用 JSON 请求")
				return
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && !machineRequest && !keyRequest && r.URL.Path != stripeWebhookPath {
			if a.origin == "" || r.Header.Get("Origin") != a.origin || r.Header.Get("X-Xingdu-Request") != "1" {
				failure(w, 403, "origin_rejected", "请求来源不受信任")
				return
			}
			if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
				failure(w, 403, "origin_rejected", "请求来源不受信任")
				return
			}
			contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				failure(w, 415, "invalid_content_type", "请使用 JSON 请求")
				return
			}
		}
		requestTimeout := 5 * time.Second
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/auth/oauth/") && strings.HasSuffix(r.URL.Path, "/callback") {
			requestTimeout = 8 * time.Second
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/billing") {
			requestTimeout = 25 * time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func validID(w http.ResponseWriter, r *http.Request) bool {
	prefix := "srv"
	if strings.HasPrefix(r.URL.Path, "/api/v1/rule-templates/") {
		prefix = "obj"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/subscriptions/") {
		prefix = "sub"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/members/") {
		prefix = "usr"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/invitations/") {
		prefix = "inv"
	}
	if !id.Valid(prefix, r.PathValue("id")) {
		failure(w, 400, "invalid_id", "记录 ID 无效")
		return false
	}
	return true
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	return decodeLimit(w, r, out, 16*1024)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, out any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		failure(w, 400, "invalid_json", "请求内容格式不正确")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		failure(w, 400, "invalid_json", "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}
func storeError(w http.ResponseWriter, err error) {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "P0006" {
		failure(w, 409, "single_organization", "自部署模式仅支持一个组织")
		return
	}
	if errors.As(err, &databaseError) && databaseError.Code == "P0005" {
		failure(w, 402, "payment_required", "请先为当前组织开通云端套餐，再新增资源")
		return
	}
	if errors.As(err, &databaseError) && databaseError.Code == "P0004" {
		failure(w, 409, "quota_exceeded", "当前组织已达到资源配额，请联系服务管理员")
		return
	}
	switch {
	case errors.Is(err, storage.ErrForbidden):
		failure(w, 403, "forbidden", "你没有此组织的操作权限")
	case errors.Is(err, storage.ErrInvalid):
		failure(w, 422, "invalid_input", "输入内容不正确")
	case errors.Is(err, storage.ErrNotFound):
		failure(w, 404, "not_found", "记录不存在或已删除")
	case errors.Is(err, storage.ErrConflict):
		failure(w, 409, "conflict", "记录已存在或与当前状态冲突")
	default:
		failure(w, 503, "unavailable", "服务暂时不可用，请稍后重试")
	}
}
func failure(w http.ResponseWriter, status int, code, message string) {
	reply(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
