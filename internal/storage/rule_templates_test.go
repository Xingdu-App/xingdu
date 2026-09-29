package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/subscription"
)

func TestRuleTemplateIsolationAndSnapshot(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("options", "-crole=xingdu_app")
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Exercise the full lifecycle on the free one-server cloud tier.
	s.CloudBilling = true
	users := []string{}
	defer func() {
		for _, id := range users {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range users {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	register := func() (string, string) {
		id, e := s.Register(ctx, "sub_"+NewID("obj")[4:12], "unused", "Subscription test")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, id)
		orgs, e := s.Organizations(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		return id, orgs[0].ID
	}
	a, org := register()
	b, other := register()
	viewer, _ := register()
	ca, cb := WithTenant(ctx, a, org), WithTenant(ctx, b, other)
	if _, err = admin.Pool.Exec(ctx, `INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'viewer')`, org, viewer); err != nil {
		t.Fatal(err)
	}
	h, err := s.CreateHost(ca, hosts.Input{Name: "subscription", Address: "node.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	node := NewID("node")
	if _, err = admin.Pool.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,state,action,operation_id,encrypted,agent_hash,installed_at) VALUES($1,$2,$3,$4,'node','trojan',443,'node.example.invalid','succeeded','deploy',$5,$6,'test',now())`, node, org, h.ID, a, NewID("op"), []byte("encrypted-secret")); err != nil {
		t.Fatal(err)
	}
	template, err := s.SaveRuleTemplate(ca, RuleTemplate{Name: "Daily", Rules: []subscription.Rule{{Type: "domain_suffix", Value: "example.com", Target: "direct"}}, FinalAction: "proxy"}, true)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.RuleTemplates(cb)
	if err != nil || len(rows) != 0 {
		t.Fatal("cross-tenant template visibility", err)
	}
	rows, err = s.RuleTemplates(WithTenant(ctx, viewer, org))
	if err != nil || len(rows) != 1 {
		t.Fatal("member cannot read templates", err)
	}
	if _, err = s.SaveRuleTemplate(WithTenant(ctx, viewer, org), template, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer write", err)
	}
	if err = s.DeleteRuleTemplate(WithTenant(ctx, viewer, org), template.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer delete", err)
	}
	if _, err = s.SaveRuleTemplate(cb, template, false); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant edit", err)
	}
	if err = s.DeleteRuleTemplate(cb, template.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant delete", err)
	}
	sub, err := s.SaveSubscription(ca, Subscription{ID: NewID("sub"), Name: "Applied", Format: "stash", NodeIDs: []string{node}, Rules: template.Rules, FinalAction: template.FinalAction, Enabled: true}, machine.Hash(machine.Token()), true)
	if err != nil {
		t.Fatal(err)
	}

	// Full configuration is a subscription snapshot under the same RLS scope.
	sub.Routing = &subscription.Routing{Preset: "balanced-v1", Groups: []subscription.RoutingGroup{{ID: "proxy", Name: "Default", Type: "url-test"}, {ID: "work", Name: "Work", Type: "fallback", NodeIDs: []string{node}}}, Targets: map[string]string{"proxy": "group:work"}, Final: "group:proxy"}
	sub, err = s.SaveSubscription(ca, sub, "", false)
	if err != nil || sub.Routing == nil || sub.Routing.Groups[1].NodeIDs[0] != node {
		t.Fatal("routing round trip", err)
	}
	visible, err := s.Subscriptions(cb)
	if err != nil || len(visible) != 0 {
		t.Fatal("cross-tenant routing visible", err)
	}
	if _, err = s.SaveSubscription(cb, sub, "", false); err == nil {
		t.Fatal("cross-tenant routing update")
	}
	if _, err = s.SaveSubscription(WithTenant(ctx, viewer, org), sub, "", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer routing write", err)
	}
	sub.Routing.Groups[1].NodeIDs = []string{NewID("node")}
	if _, err = s.SaveSubscription(ca, sub, "", false); !errors.Is(err, ErrInvalid) {
		t.Fatal("foreign group membership", err)
	}
	sub.Routing.Groups[1].NodeIDs = []string{node}
	sub.Format = "hysteria2_uri"
	if _, err = s.SaveSubscription(ca, sub, "", false); !errors.Is(err, ErrInvalid) {
		t.Fatal("URI routing accepted", err)
	}
	sub.Format = "stash"
	template.Rules = []subscription.Rule{{Type: "domain", Value: "changed.example.com", Target: "reject"}}
	template.FinalAction = "direct"
	template, err = s.SaveRuleTemplate(ca, template, false)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = s.RuleTemplates(ca)
	if err != nil || len(rows) != 1 || rows[0].Rules[0].Value != "changed.example.com" {
		t.Fatal("edit not persisted", err)
	}
	if err = s.DeleteRuleTemplate(ca, template.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Subscriptions(ca)
	if err != nil || len(saved) != 1 || saved[0].ID != sub.ID || saved[0].Rules[0].Value != "example.com" || saved[0].FinalAction != "proxy" || saved[0].Routing == nil || saved[0].Routing.Targets["proxy"] != "group:work" {
		t.Fatal("template edit/delete changed subscription snapshot", err)
	}
	if _, err = admin.Pool.Exec(ctx, "DELETE FROM subscription_nodes WHERE subscription_id=$1 AND node_id=$2", sub.ID, node); err != nil {
		t.Fatal(err)
	}
	saved, err = s.Subscriptions(ca)
	if err != nil || len(saved[0].Routing.Groups[1].NodeIDs) != 0 {
		t.Fatal("deleted node retained stale group membership", err)
	}
	bad := RuleTemplate{Name: "Bad", Rules: []subscription.Rule{{Type: "domain", Value: "bad\n.example", Target: "direct"}}, FinalAction: "proxy"}
	if _, err = s.SaveRuleTemplate(ca, bad, true); !errors.Is(err, ErrInvalid) {
		t.Fatal("invalid rule accepted", err)
	}
	// Direct SQL still observes FORCE RLS, independently of the store checks.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, viewer, org); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO rule_templates(id,organization_id,name,rules,final_action) VALUES($1,$2,'Denied','[]','proxy')", NewID("obj"), org); err == nil {
		t.Fatal("RLS allowed viewer insert")
	}
}
