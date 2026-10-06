package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
)

type subscriptionPatch struct {
	Name         *string              `json:"name"`
	Format       *string              `json:"format"`
	NodeIDs      *[]string            `json:"node_ids"`
	Rules        *[]subscription.Rule `json:"rules"`
	FinalAction  *string              `json:"final_action"`
	Enabled      *bool                `json:"enabled"`
	Routing      json.RawMessage      `json:"routing"`
	GroupUpdates []groupIconPatch     `json:"group_updates"`
}
type groupIconPatch struct {
	ID   string          `json:"id"`
	Icon json.RawMessage `json:"icon"`
}

func applySubscriptionPatch(base storage.Subscription, p subscriptionPatch) (storage.Subscription, error) {
	// Deep-copy the snapshot; preview and failed edits must not mutate stored data.
	b, _ := json.Marshal(base)
	var out storage.Subscription
	_ = json.Unmarshal(b, &out)
	if p.Name != nil {
		out.Name = *p.Name
	}
	if p.Format != nil {
		out.Format = *p.Format
	}
	if p.NodeIDs != nil {
		out.NodeIDs = *p.NodeIDs
	}
	if p.Rules != nil {
		out.Rules = *p.Rules
	}
	if p.FinalAction != nil {
		out.FinalAction = *p.FinalAction
	}
	if p.Enabled != nil {
		out.Enabled = *p.Enabled
	}
	if len(p.Routing) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(p.Routing))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&out.Routing); err != nil {
			return out, storage.ErrInvalid
		}
	}
	if len(p.GroupUpdates) > 0 && (out.Routing == nil || len(p.Routing) > 0 || len(p.GroupUpdates) > 20) {
		return out, storage.ErrInvalid
	}
	seen := map[string]bool{}
	for _, patch := range p.GroupUpdates {
		if seen[patch.ID] || len(patch.Icon) == 0 {
			return out, storage.ErrInvalid
		}
		seen[patch.ID] = true
		icon := ""
		if string(patch.Icon) != "null" {
			if err := json.Unmarshal(patch.Icon, &icon); err != nil {
				return out, storage.ErrInvalid
			}
		}
		found := false
		for i := range out.Routing.Groups {
			if out.Routing.Groups[i].ID == patch.ID {
				out.Routing.Groups[i].Icon = icon
				found = true
				break
			}
		}
		if !found {
			return out, storage.ErrInvalid
		}
	}
	if err := out.Validate(); err != nil {
		return out, err
	}
	return out, nil
}
func subscriptionETag(revision int64) string { return `"` + strconv.FormatInt(revision, 10) + `"` }
func subscriptionMatch(w http.ResponseWriter, r *http.Request) (int64, bool) {
	h := r.Header.Get("If-Match")
	if h == "" {
		failure(w, 428, "revision_required", "请先读取订阅，再携带 If-Match 版本修改")
		return 0, false
	}
	v, err := strconv.ParseInt(strings.Trim(h, `"`), 10, 64)
	if err != nil || v < 1 || h != subscriptionETag(v) {
		failure(w, 400, "invalid_revision", "If-Match 版本格式无效")
		return 0, false
	}
	return v, true
}
func configHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}
func configConflict(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrConflict) {
		failure(w, 409, "subscription_conflict", "订阅已修改或所选节点不可用，请重新读取后重试")
	} else {
		storeError(w, err)
	}
}
func (a *api) subscriptionConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/subscriptions/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		if !validID(w, r) {
			return
		}
		out, err := a.store.Subscription(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		w.Header().Set("ETag", subscriptionETag(out.Revision))
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("PATCH /api/v1/subscriptions/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		if !validID(w, r) {
			return
		}
		revision, ok := subscriptionMatch(w, r)
		if !ok {
			return
		}
		var patch subscriptionPatch
		if !decodeLimit(w, r, &patch, 256*1024) {
			return
		}
		base, err := a.store.Subscription(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		if base.Revision != revision {
			configConflict(w, storage.ErrConflict)
			return
		}
		out, err := applySubscriptionPatch(base, patch)
		if err != nil {
			storeError(w, err)
			return
		}
		if err := a.validateSubscriptionCompatibility(r.Context(), out, false); err != nil {
			a.configRenderError(w, err)
			return
		}
		out.ExpectedRevision = revision
		out, err = a.store.SaveSubscription(r.Context(), out, "", false)
		if err != nil {
			configConflict(w, err)
			return
		}
		w.Header().Set("ETag", subscriptionETag(out.Revision))
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("GET /api/v1/subscriptions/{id}/config", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		if !validID(w, r) {
			return
		}
		sub, deps, err := a.store.SubscriptionConfig(r.Context(), r.PathValue("id"), nil)
		if err != nil {
			configConflict(w, err)
			return
		}
		format := r.URL.Query().Get("format")
		if format == "" {
			format = sub.Format
		}
		if format == "clash" {
			format = "mihomo"
		}
		content, warnings, err := a.renderSubscription(sub, deps, format)
		if err != nil {
			a.configRenderError(w, err)
			return
		}
		if len(warnings) > 0 {
			w.Header().Set("X-Xingdu-Config-Warnings", strings.Join(warnings, ","))
		}
		w.Header().Set("X-Xingdu-Subscription-Revision", strconv.FormatInt(sub.Revision, 10))
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		if format == "singbox" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		if format == "surge" || format == "loon" || subscription.LinkOnly(format) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": subscriptionFilename(sub.Name, format)}))
		w.WriteHeader(200)
		_, _ = w.Write(content)
	}))
	mux.HandleFunc("POST /api/v1/subscriptions/{id}/preview", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		if !validID(w, r) {
			return
		}
		var patch subscriptionPatch
		if !decodeLimit(w, r, &patch, 256*1024) {
			return
		}
		base, err := a.store.Subscription(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		draft, err := applySubscriptionPatch(base, patch)
		if err != nil {
			storeError(w, err)
			return
		}
		current, deps, err := a.store.SubscriptionConfig(r.Context(), base.ID, draft.NodeIDs)
		if err != nil {
			configConflict(w, err)
			return
		}
		if current.Revision != base.Revision {
			configConflict(w, storage.ErrConflict)
			return
		}
		content, warnings, err := a.renderSubscription(draft, deps, draft.Format)
		if err != nil {
			a.configRenderError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]any{"format": draft.Format, "revision": base.Revision, "content": string(content), "warnings": warnings}})
	}))
}
func (a *api) renderSubscription(sub storage.Subscription, deps []storage.Deployment, format string) ([]byte, []string, error) {
	if !subscription.ValidFormat(format) {
		return nil, nil, &subscription.CompatibilityError{Reason: "不支持此客户端格式"}
	}
	if format == "loon" && sub.Format != "loon" {
		return nil, nil, &subscription.CompatibilityError{Reason: "请在订阅设置中明确选择 Loon 证书验证方式"}
	}
	nodes := []subscription.Node{}
	for _, d := range deps {
		var spec protocol.Spec /* Decode without writing secrets into errors. */
		if a.vault == nil {
			return nil, nil, storage.ErrConflict
		}
		plain, err := a.vault.Open(d.Encrypted, deploymentAAD(d.OrgID, d.HostID, d.ID))
		if err != nil {
			return nil, nil, storage.ErrConflict
		}
		decodeErr := json.Unmarshal(plain, &spec)
		clear(plain)
		if decodeErr != nil {
			return nil, nil, storage.ErrConflict
		}
		nodes = append(nodes, subscription.Node{ID: d.ID, Name: d.Name, Server: d.Server, Spec: spec})
	}
	warnings := subscription.IconWarnings(sub.Routing, format)
	content, err := subscription.RenderRouting(format, sub.Name, nodes, sub.Rules, sub.FinalAction, sub.Routing, sub.NodeIDs)
	return content, warnings, err
}
func (a *api) configRenderError(w http.ResponseWriter, err error) {
	var compatibility *subscription.CompatibilityError
	if errors.As(err, &compatibility) {
		failure(w, 422, "client_incompatible", compatibility.Reason)
		return
	}
	configConflict(w, err)
}
