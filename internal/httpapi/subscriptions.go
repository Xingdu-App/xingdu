package httpapi

import (
	"context"
	"errors"
	"mime"
	"net"
	"net/http"
	"strings"
	"unicode"
	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
)

type SubscriptionStore interface {
	RuleTemplates(context.Context) ([]storage.RuleTemplate, error)
	SaveRuleTemplate(context.Context, storage.RuleTemplate, bool) (storage.RuleTemplate, error)
	DeleteRuleTemplate(context.Context, string) error
	Subscription(context.Context, string) (storage.Subscription, error)
	SubscriptionConfig(context.Context, string, []string) (storage.Subscription, []storage.Deployment, error)
	Subscriptions(context.Context) ([]storage.Subscription, error)
	SaveSubscription(context.Context, storage.Subscription, string, bool) (storage.Subscription, error)
	RotateSubscription(context.Context, string, string, ...[]byte) error
	DeleteSubscription(context.Context, string) error
	SubscriptionContent(context.Context, string, string) (storage.Subscription, []storage.Deployment, error)
}

func subscriptionPath(id, token string) string {
	return "/api/v1/subscriptions/" + id + "/content?token=" + token + ""
}
func (a *api) subscriptionRoutes(mux *http.ServeMux) {
	a.ruleTemplateRoutes(mux)
	a.subscriptionConfigRoutes(mux)
	mux.HandleFunc("GET /api/v1/subscription-presets", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		reply(w, 200, map[string]any{"data": subscription.RoutingCatalog()})
	}))
	mux.HandleFunc("GET /api/v1/subscriptions", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		out, err := a.store.Subscriptions(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		w.Header().Set("Referrer-Policy", "no-referrer")
		for i := range out {
			a.revealSubscriptionLink(storage.TenantOrg(r.Context()), &out[i])
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	save := func(create bool) func(http.ResponseWriter, *http.Request, storage.User, string) {
		return func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
			configHeaders(w)
			if !create && !validID(w, r) {
				return
			}
			if !a.attempts.allow("subscription-write:" + u.ID) {
				failure(w, 429, "rate_limited", "请稍后重试")
				return
			}
			var in storage.Subscription
			if !decodeLimit(w, r, &in, 256*1024) {
				return
			}
			in.SubscriptionPath, in.LinkState = "", ""
			in.ID = r.PathValue("id")
			if !create {
				in.ExpectedRevision = in.Revision
				if _, apiKey := r.Context().Value(apiPrincipal{}).(storage.APIKey); apiKey && r.Header.Get("If-Match") == "" {
					failure(w, 428, "revision_required", "请先读取订阅，再携带 If-Match 版本修改")
					return
				}
				if r.Header.Get("If-Match") != "" {
					revision, ok := subscriptionMatch(w, r)
					if !ok {
						return
					}
					in.ExpectedRevision = revision
				}
				if in.ExpectedRevision < 0 {
					failure(w, 422, "invalid_revision", "版本号无效")
					return
				}
			}
			token := ""
			hash := ""
			if create {
				if a.vault == nil {
					failure(w, 503, "credential_key_required", "订阅链接加密不可用")
					return
				}
				in.ID = storage.NewID("sub")
				token = machine.Token()
				hash = machine.Hash(token)
				in.EncryptedToken = a.vault.Seal([]byte(token), subscriptionTokenAAD(storage.TenantOrg(r.Context()), in.ID))
			}
			out, err := a.store.SaveSubscription(r.Context(), in, hash, create)
			if err != nil {
				configConflict(w, err)
				return
			}
			if create {
				data := map[string]any{"subscription": out}
				if _, apiKey := r.Context().Value(apiPrincipal{}).(storage.APIKey); !apiKey {
					data["subscription_path"] = subscriptionPath(out.ID, token)
				}
				w.Header().Set("ETag", subscriptionETag(out.Revision))
				reply(w, 201, map[string]any{"data": data})
			} else {
				w.Header().Set("ETag", subscriptionETag(out.Revision))
				reply(w, 200, map[string]any{"data": out})
			}
		}
	}
	mux.HandleFunc("POST /api/v1/subscriptions", a.tenant(save(true)))
	mux.HandleFunc("PUT /api/v1/subscriptions/{id}", a.tenant(save(false)))
	mux.HandleFunc("POST /api/v1/subscriptions/{id}/rotate", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "订阅链接加密不可用")
			return
		}
		token := machine.Token()
		encrypted := a.vault.Seal([]byte(token), subscriptionTokenAAD(storage.TenantOrg(r.Context()), r.PathValue("id")))
		if err := a.store.RotateSubscription(r.Context(), r.PathValue("id"), machine.Hash(token), encrypted); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"subscription_path": subscriptionPath(r.PathValue("id"), token)}})
	}))
	mux.HandleFunc("DELETE /api/v1/subscriptions/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.DeleteSubscription(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("GET /api/v1/subscriptions/{id}/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !a.attempts.allowLimit("subscription-content:"+remoteIP(r), 120) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		token := r.URL.Query().Get("token")
		id := r.PathValue("id")
		if !resourceid.Valid("sub", id) || !machine.ValidToken(token) {
			failure(w, 404, "not_found", "订阅不存在或链接已失效")
			return
		}
		sub, deps, err := a.store.SubscriptionContent(r.Context(), id, machine.Hash(token))
		if err != nil {
			storeError(w, err)
			return
		}
		format := r.URL.Query().Get("format")
		if format == "" {
			format = sub.Format
		}
		if format == "clash" {
			format = "mihomo"
		}
		if !subscription.ValidFormat(format) {
			failure(w, 422, "unsupported_format", "不支持此客户端格式；Shadowrocket 的安全导入尚未验证")
			return
		}
		if format == "loon" && sub.Format != "loon" {
			failure(w, 422, "client_incompatible", "请由管理员在订阅设置中明确选择 Loon 的系统 CA 验证方式")
			return
		}
		if len(deps) == 0 {
			failure(w, 409, "no_available_nodes", "订阅没有可用的已部署节点")
			return
		}
		nodes := make([]subscription.Node, 0, len(deps))
		for _, d := range deps {
			var spec protocol.Spec
			if !a.openDeployment(w, d, d.OrgID, &spec) {
				return
			}
			nodes = append(nodes, subscription.Node{ID: d.ID, Name: d.Name, Server: d.Server, Spec: spec})
		}
		if warnings := subscription.IconWarnings(sub.Routing, format); len(warnings) > 0 {
			w.Header().Set("X-Xingdu-Config-Warnings", strings.Join(warnings, ","))
		}
		content, err := subscription.RenderRouting(format, sub.Name, nodes, sub.Rules, sub.FinalAction, sub.Routing, sub.NodeIDs)
		if err != nil {
			var compatibility *subscription.CompatibilityError
			if errors.As(err, &compatibility) {
				failure(w, 422, "client_incompatible", compatibility.Reason)
				return
			}
			failure(w, 409, "subscription_unavailable", "节点配置或证书不可用，请联系订阅管理员")
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		if format == "surge" || format == "loon" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		if format == "hysteria2_uri" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": subscriptionFilename(sub.Name, format)}))
		w.WriteHeader(200)
		_, _ = w.Write(content)
	})
}

func subscriptionFilename(name, format string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(strings.TrimSpace(name), ". ")
	if name == "" {
		name = "subscription"
	}
	ext := ".yaml"
	if format == "surge" || format == "loon" {
		ext = ".conf"
	} else if format == "hysteria2_uri" {
		ext = ".txt"
	}
	return name + ext
}

func remoteIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func subscriptionTokenAAD(org, id string) string {
	return "xingdu-subscription-token-v1:" + org + ":" + id
}
func (a *api) revealSubscriptionLink(org string, sub *storage.Subscription) {
	if len(sub.EncryptedToken) == 0 {
		return
	}
	defer func() { sub.EncryptedToken = nil; sub.TokenHash = "" }()
	sub.LinkState = "unavailable"
	if a.vault == nil {
		return
	}
	token, err := a.vault.Open(sub.EncryptedToken, subscriptionTokenAAD(org, sub.ID))
	if err != nil {
		return
	}
	defer clear(token)
	if !machine.ValidToken(string(token)) || machine.Hash(string(token)) != sub.TokenHash {
		return
	}
	sub.SubscriptionPath = subscriptionPath(sub.ID, string(token))
	sub.LinkState = "available"
}
