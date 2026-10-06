package httpapi

import (
	"context"
	"errors"
	"net/http"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
)

type nodeCompatibility struct {
	ID         string `json:"id"`
	Compatible bool   `json:"compatible"`
	Reason     string `json:"reason,omitempty"`
}
type compatibilityReport struct {
	Nodes  []nodeCompatibility `json:"nodes"`
	Issues []string            `json:"issues"`
}

func compatibilityReason(err error) string {
	var issue *subscription.CompatibilityError
	if errors.As(err, &issue) {
		return issue.Reason
	}
	return "节点配置不可用，请检查节点状态或重新部署"
}
func (a *api) compatibility(ctx context.Context, draft storage.Subscription, candidates []string) (compatibilityReport, error) {
	report := compatibilityReport{Nodes: []nodeCompatibility{}, Issues: []string{}}
	deps, err := a.store.SubscriptionCandidates(ctx, candidates)
	if err != nil {
		return report, err
	}
	byID := map[string]storage.Deployment{}
	for _, d := range deps {
		byID[d.ID] = d
	}
	for _, key := range candidates {
		item := nodeCompatibility{ID: key}
		if d, ok := byID[key]; ok {
			sub := storage.Subscription{Name: "Compatibility", Format: draft.Format, NodeIDs: []string{key}, FinalAction: "proxy"}
			content, _, e := a.renderSubscription(sub, []storage.Deployment{d}, draft.Format)
			clear(content)
			item.Compatible = e == nil
			if e != nil {
				item.Reason = compatibilityReason(e)
			}
		} else {
			item.Reason = "节点不可用或不属于当前组织"
		}
		report.Nodes = append(report.Nodes, item)
	}
	if len(draft.NodeIDs) > 0 {
		selected := []storage.Deployment{}
		for _, key := range draft.NodeIDs {
			if d, ok := byID[key]; ok {
				selected = append(selected, d)
			} else {
				report.Issues = append(report.Issues, "已选节点不可用，请取消选择或刷新节点列表")
				return report, nil
			}
		}
		draft.Name = "Compatibility"
		content, _, e := a.renderSubscription(draft, selected, draft.Format)
		clear(content)
		if e != nil {
			report.Issues = append(report.Issues, compatibilityReason(e))
		}
	}
	return report, nil
}
func (a *api) validateSubscriptionCompatibility(ctx context.Context, in storage.Subscription, create bool) error {
	if in.Format == "" {
		if !create {
			base, e := a.store.Subscription(ctx, in.ID)
			if e != nil {
				return e
			}
			in.Format = base.Format
		} else {
			in.Format = "stash"
		}
	}
	if err := in.Validate(); err != nil {
		return err
	}
	if !in.Enabled || len(in.NodeIDs) == 0 {
		return nil
	}
	report, err := a.compatibility(ctx, in, in.NodeIDs)
	if err != nil {
		return err
	}
	if len(report.Issues) > 0 {
		return &subscription.CompatibilityError{Format: in.Format, Reason: report.Issues[0]}
	}
	return nil
}
func (a *api) subscriptionCompatibilityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/subscriptions/compatibility", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		configHeaders(w)
		if !a.attempts.allowLimit("subscription-compatibility:"+u.ID, 120) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		var in struct {
			Format       string                `json:"format"`
			CandidateIDs []string              `json:"candidate_ids"`
			NodeIDs      []string              `json:"node_ids"`
			Rules        []subscription.Rule   `json:"rules"`
			FinalAction  string                `json:"final_action"`
			Routing      *subscription.Routing `json:"routing"`
		}
		if !decodeLimit(w, r, &in, 256*1024) {
			return
		}
		if !subscription.ValidFormat(in.Format) || len(in.CandidateIDs) > 1000 || len(in.NodeIDs) > 100 {
			failure(w, 422, "invalid_subscription", "配置格式或节点数量无效")
			return
		}
		seen := map[string]bool{}
		for _, key := range in.CandidateIDs {
			if !id.Valid("node", key) || seen[key] {
				failure(w, 422, "invalid_nodes", "节点列表无效")
				return
			}
			seen[key] = true
		}
		selected := map[string]bool{}
		for _, key := range in.NodeIDs {
			if !seen[key] || selected[key] {
				failure(w, 422, "invalid_nodes", "已选节点列表无效")
				return
			}
			selected[key] = true
		}
		draft := storage.Subscription{Format: in.Format, NodeIDs: in.NodeIDs, Rules: in.Rules, FinalAction: in.FinalAction, Routing: in.Routing}
		out, err := a.compatibility(r.Context(), draft, in.CandidateIDs)
		if err != nil {
			configConflict(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
}
