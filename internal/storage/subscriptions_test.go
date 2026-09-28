package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/subscription"
)

func TestSubscriptionTenantCapabilityLifecycle(t *testing.T) {
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
		id, e := s.Register(ctx, "sub_"+NewID()[:8], "unused", "Subscription test")
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
	node := NewID()
	if _, err = admin.Pool.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,state,action,operation_id,encrypted,agent_hash,installed_at) VALUES($1,$2,$3,$4,'node','trojan',443,'node.example.invalid','succeeded','deploy',$5,$6,'test',now())`, node, org, h.ID, a, NewID(), []byte("encrypted-secret")); err != nil {
		t.Fatal(err)
	}
	in := Subscription{ID: NewID(), Name: "Personal", NodeIDs: []string{node}, Rules: []subscription.Rule{{Type: "domain_suffix", Value: "example.com", Target: "direct"}}, FinalAction: "proxy", Enabled: true}
	hash := machine.Hash(machine.Token())
	out, err := s.SaveSubscription(ca, in, hash, true)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID {
		t.Fatal(out)
	}
	if out.Format != "stash" {
		t.Fatal("new subscription must default to Stash")
	}
	for _, state := range []string{"queued", "running", "failed", "interrupted"} {
		if _, e := admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET action='restart',state=$2 WHERE id=$1", node, state); e != nil {
			t.Fatal(e)
		}
		if _, nodes, e := s.SubscriptionContent(ctx, in.ID, hash); e != nil || len(nodes) != 1 {
			t.Fatal("confirmed installed node lost during restart", state, e)
		}
		if _, e := s.SaveSubscription(ca, in, "", false); e != nil {
			t.Fatal("subscription edit rejected during restart", state, e)
		}
	}
	if _, e := admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET action='deploy',state='succeeded' WHERE id=$1", node); e != nil {
		t.Fatal(e)
	}
	in.Format = "mihomo"
	if changed, e := s.SaveSubscription(ca, in, "", false); e != nil || changed.Format != "mihomo" {
		t.Fatal("format update failed", e)
	}
	in.Format = ""
	if changed, e := s.SaveSubscription(ca, in, "", false); e != nil || changed.Format != "mihomo" {
		t.Fatal("omitted format must preserve existing value", e)
	}
	for _, format := range []string{"surge", "loon"} {
		in.Format = format
		if changed, e := s.SaveSubscription(ca, in, "", false); e != nil || changed.Format != format {
			t.Fatal("client format not persisted", format, e)
		}
	}
	in.Format = "hysteria2_uri"
	in.Rules = nil
	if _, e := s.SaveSubscription(ca, in, "", false); !errors.Is(e, ErrInvalid) {
		t.Fatal("unsupported Trojan share-link subscription accepted", e)
	}
	in.Format = "mihomo"
	if _, e := s.SaveSubscription(ca, in, "", false); e != nil {
		t.Fatal(e)
	}
	in.Format = "unsupported_test_format"
	if _, e := s.SaveSubscription(ca, in, "", false); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid format accepted")
	}
	in.Format = ""
	// Direct SQL with token scope can only read selected deployments; invalid tokens read nothing.
	unselected := NewID()
	if _, err = admin.Pool.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,state,action,operation_id,encrypted,agent_hash,installed_at) VALUES($1,$2,$3,$4,'unselected','trojan',444,'node.example.invalid','succeeded','deploy',$5,$6,'test',now())`, unselected, org, h.ID, a, NewID(), []byte("other-secret")); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		hash string
		want int
	}{{hash, 1}, {hash + "bad", 0}} {
		tx, e := s.Pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = setScope(ctx, tx, "", org); e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, `SELECT set_config('app.subscription_id',$1,true),set_config('app.subscription_hash',$2,true)`, in.ID, check.hash); e != nil {
			t.Fatal(e)
		}
		for _, table := range []string{"subscriptions", "subscription_nodes", "protocol_deployments", "hosts"} {
			var n int
			if e = tx.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); e != nil || n != check.want {
				t.Fatalf("token RLS %s: %d want %d (%v)", table, n, check.want, e)
			}
		}
		tx.Rollback(ctx)
	}
	meta, err := s.Subscriptions(WithTenant(ctx, viewer, org))
	if err != nil || len(meta) != 1 {
		t.Fatal(meta, err)
	}
	serialized, _ := json.Marshal(meta)
	if strings.Contains(string(serialized), hash) || strings.Contains(string(serialized), "encrypted") {
		t.Fatal("metadata leaks")
	}
	if _, err = s.SaveSubscription(WithTenant(ctx, viewer, org), in, "", false); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer mutate", err)
	}
	if list, e := s.Subscriptions(cb); e != nil || len(list) != 0 {
		t.Fatal("cross tenant list", list, e)
	}
	if err = s.RotateSubscription(cb, in.ID, machine.Hash(machine.Token())); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross tenant rotate", err)
	}
	foreign := in
	foreign.ID = NewID()
	if _, err = s.SaveSubscription(cb, foreign, hash+"x", true); !errors.Is(err, ErrInvalid) {
		t.Fatal("cross tenant node", err)
	}
	if _, _, err = s.SubscriptionContent(ctx, in.ID, hash+"x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("invalid capability", err)
	}
	content, deps, err := s.SubscriptionContent(ctx, in.ID, hash)
	if err != nil || len(deps) != 1 || content.ID != in.ID || deps[0].OrgID != org || string(deps[0].Encrypted) != "encrypted-secret" {
		t.Fatal("content", content, deps, err)
	}
	hash2 := machine.Hash(machine.Token())
	if err = s.RotateSubscription(ca, in.ID, hash2); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SubscriptionContent(ctx, in.ID, hash); !errors.Is(err, ErrNotFound) {
		t.Fatal("old token", err)
	}
	in.Enabled = false
	if _, err = s.SaveSubscription(ca, in, "", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SubscriptionContent(ctx, in.ID, hash2); !errors.Is(err, ErrNotFound) {
		t.Fatal("disabled", err)
	}
	in.Enabled = true
	if _, err = s.SaveSubscription(ca, in, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET state='removed',encrypted=NULL WHERE id=$1", node); err != nil {
		t.Fatal(err)
	}
	if _, deps, err = s.SubscriptionContent(ctx, in.ID, hash2); err != nil || len(deps) != 0 {
		t.Fatal("removed node exported", deps, err)
	}
	in.Enabled = false
	if _, err = s.SaveSubscription(ca, in, "", false); err != nil {
		t.Fatal("cannot disable removed selection", err)
	}
	if _, err = admin.Pool.Exec(ctx, "DELETE FROM hosts WHERE id=$1", h.ID); err != nil {
		t.Fatal(err)
	}
	in.NodeIDs = []string{}
	if _, err = s.SaveSubscription(ca, in, "", false); err != nil {
		t.Fatal("cannot edit empty existing subscription", err)
	}
	if err = s.DeleteSubscription(ca, in.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SubscriptionContent(ctx, in.ID, hash2); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted", err)
	}
}
