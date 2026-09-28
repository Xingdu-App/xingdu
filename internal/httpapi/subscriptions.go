package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
)

type SubscriptionStore interface {
	Subscriptions(context.Context) ([]storage.Subscription, error)
	SaveSubscription(context.Context, storage.Subscription, string, bool) (storage.Subscription, error)
	RotateSubscription(context.Context, string, string) error
	DeleteSubscription(context.Context, string) error
	SubscriptionContent(context.Context, string, string) (storage.Subscription, []storage.Deployment, error)
}

func subscriptionPath(id, token string) string {
	return "/api/v1/subscriptions/" + id + "/content?token=" + token + ""
}
func (a *api) subscriptionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/subscriptions", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := a.store.Subscriptions(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	save := func(create bool) func(http.ResponseWriter, *http.Request, storage.User, string) {
		return func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
			if !create && !validID(w, r) {
				return
			}
			if !a.attempts.allow("subscription-write:" + u.ID) {
				failure(w, 429, "rate_limited", "请稍后重试")
				return
			}
			var in storage.Subscription
			if !decodeLimit(w, r, &in, 64*1024) {
				return
			}
			in.ID = r.PathValue("id")
			token := ""
			hash := ""
			if create {
				in.ID = storage.NewID()
				token = machine.Token()
				hash = machine.Hash(token)
			}
			out, err := a.store.SaveSubscription(r.Context(), in, hash, create)
			if err != nil {
				storeError(w, err)
				return
			}
			if create {
				reply(w, 201, map[string]any{"data": map[string]any{"subscription": out, "subscription_path": subscriptionPath(out.ID, token)}})
			} else {
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
		token := machine.Token()
		if err := a.store.RotateSubscription(r.Context(), r.PathValue("id"), machine.Hash(token)); err != nil {
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
		if !hosts.IDPattern.MatchString(id) || !machine.ValidToken(token) {
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
		content, err := subscription.Render(format, sub.Name, nodes, sub.Rules, sub.FinalAction)
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
		w.Header().Set("Content-Disposition", `attachment; filename="xingdu.yaml"`)
		if format == "surge" || format == "loon" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="xingdu.conf"`)
		}
		if format == "hysteria2_uri" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="xingdu.txt"`)
		}
		w.WriteHeader(200)
		_, _ = w.Write(content)
	})
}

func remoteIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
