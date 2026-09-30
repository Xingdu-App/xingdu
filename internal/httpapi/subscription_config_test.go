package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
	"xingdu.app/xingdu/internal/vault"
)

func TestSubscriptionPatchPreservesSnapshot(t *testing.T) {
	base := storage.Subscription{Name: "Example", Format: "stash", FinalAction: "proxy", Enabled: true, NodeIDs: []string{}, Rules: []subscription.Rule{}, Routing: &subscription.Routing{Preset: "balanced-v1", Groups: []subscription.RoutingGroup{{ID: "proxy", Name: "Default", Type: "select", Icon: "https://assets.example.com/old.png"}}, Final: "proxy"}}
	var patch subscriptionPatch
	if err := json.Unmarshal([]byte(`{"group_updates":[{"id":"proxy","icon":"https://assets.example.com/new.png"}]}`), &patch); err != nil {
		t.Fatal(err)
	}
	out, err := applySubscriptionPatch(base, patch)
	if err != nil {
		t.Fatal(err)
	}
	if out.Routing.Groups[0].Icon == base.Routing.Groups[0].Icon || out.Name != base.Name || !out.Enabled {
		t.Fatal("patch lost settings or mutated source")
	}
	for _, body := range []string{`{"group_updates":[{"id":"missing","icon":null}]}`, `{"group_updates":[{"id":"proxy"}]}`, `{"group_updates":[{"id":"proxy","icon":"http://assets.example.com/a.png"}]}`, `{"group_updates":[{"id":"proxy","icon":null},{"id":"proxy","icon":null}]}`} {
		var bad subscriptionPatch
		_ = json.Unmarshal([]byte(body), &bad)
		if _, err := applySubscriptionPatch(base, bad); err == nil {
			t.Fatal("invalid patch accepted", body)
		}
	}
	var clearIcon subscriptionPatch
	_ = json.Unmarshal([]byte(`{"group_updates":[{"id":"proxy","icon":null}]}`), &clearIcon)
	out, err = applySubscriptionPatch(base, clearIcon)
	if err != nil || out.Routing.Groups[0].Icon != "" {
		t.Fatal("icon deletion", err)
	}
}
func TestSubscriptionConfigAPIIsolationAndRevision(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := storage.Open(ctx, raw)
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
	s, err := storage.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.Register(ctx, "config_"+storage.NewID("obj"), "fixture", "Config test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", user)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", user)
	}()
	orgs, err := s.Organizations(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	org := orgs[0].ID
	tenant := storage.WithTenant(ctx, user, org)
	host, err := s.CreateHost(tenant, hosts.Input{Name: "Example", Address: "192.0.2.10", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := vault.New(strings.Repeat("a", 64))
	spec, err := protocol.NewSpec(protocol.Input{Name: "Example", Protocol: "shadowsocks", Port: 8388})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(spec)
	node := storage.NewID("node")
	_, err = admin.Pool.Exec(ctx, `INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,state,action,operation_id,encrypted,agent_hash,installed_at) VALUES($1,$2,$3,$4,'Example','shadowsocks',8388,'','succeeded','deploy',$5,$6,'fixture',now())`, node, org, host.ID, user, storage.NewID("op"), v.Seal(plain, deploymentAAD(org, host.ID, node)))
	if err != nil {
		t.Fatal(err)
	}
	sub, err := s.SaveSubscription(tenant, storage.Subscription{ID: storage.NewID("sub"), Name: "Example", Format: "stash", Enabled: true, NodeIDs: []string{node}, FinalAction: "proxy", Routing: &subscription.Routing{Preset: "balanced-v1", Groups: []subscription.RoutingGroup{{ID: "proxy", Name: "Default", Type: "select"}}, Final: "proxy"}}, machine.Hash(machine.Token()), true)
	if err != nil {
		t.Fatal(err)
	}
	key := func(scopes ...string) string {
		secret := "xd_key_" + machine.Token()
		_, e := s.CreateAPIKey(tenant, "Fixture", tokenHash(secret), secret[:15], scopes, time.Now().Add(time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		return secret
	}
	read, write, export := key("subscriptions:read"), key("subscriptions:write"), key("subscriptions:export")
	h := New(s, Options{CredentialVault: v})
	path := "/api/v1/subscriptions/" + sub.ID
	call := func(method, path, body, secret, match string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+secret)
		r.Header.Set("Content-Type", "application/json")
		if match != "" {
			r.Header.Set("If-Match", match)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	metadata := call("GET", path, "", read, "")
	assert(metadata, 200)
	etag := metadata.Header().Get("ETag")
	if etag != `"1"` || strings.Contains(metadata.Body.String(), "subscription_path") || strings.Contains(metadata.Body.String(), spec.Credential) {
		t.Fatal("metadata leak or missing revision")
	}
	assert(call("GET", path+"/config", "", read, ""), 403)
	assert(call("PATCH", path, `{"name":"Renamed"}`, read, etag), 403)
	assert(call("PATCH", path, `{"name":"Renamed"}`, write, ""), 428)
	body := `{"group_updates":[{"id":"proxy","icon":"https://assets.example.com/icon.png"}]}`
	preview := call("POST", path+"/preview", body, export, "")
	assert(preview, 200)
	if !strings.Contains(preview.Body.String(), "icon.png") || preview.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preview omitted icon or cache header")
	}
	unchanged, e := s.Subscription(tenant, sub.ID)
	if e != nil || unchanged.Revision != 1 || unchanged.Routing.Groups[0].Icon != "" {
		t.Fatal("preview persisted changes", e)
	}
	changed := call("PATCH", path, body, write, etag)
	assert(changed, 200)
	if changed.Header().Get("ETag") != `"2"` {
		t.Fatal("revision not advanced")
	}
	assert(call("PATCH", path, `{"name":"stale"}`, write, etag), 409)
	assert(call("PATCH", path, `{"arbitrary_yaml":"unsafe"}`, write, `"2"`), 400)
	config := call("GET", path+"/config", "", export, "")
	assert(config, 200)
	if !strings.Contains(config.Body.String(), "icon.png") || !strings.Contains(config.Body.String(), spec.Credential) || config.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("config missing icon, credentials or no-store")
	}
	warning := call("GET", path+"/config?format=surge", "", export, "")
	assert(warning, 200)
	if warning.Header().Get("X-Xingdu-Config-Warnings") != "group_icons_not_exported_for_format" {
		t.Fatal("missing compatibility warning")
	}
	// Export and node selection must retain the last confirmed config after rollback.
	_, err = admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET action='update',state='failed',pending_revision=NULL WHERE id=$1", node)
	if err != nil {
		t.Fatal(err)
	}
	nodes, e := s.Nodes(tenant)
	if e != nil || len(nodes) != 1 || !nodes[0].SubscriptionSelectable {
		t.Fatal("restored node not selectable", e)
	}
	assert(call("GET", path+"/config", "", export, ""), 200)
	// A key from another tenant cannot read or export this subscription.
	other, e := s.Register(ctx, "config_other_"+storage.NewID("obj"), "fixture", "Other")
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", other)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", other)
	}()
	otherOrgs, _ := s.Organizations(ctx, other)
	secret := "xd_key_" + machine.Token()
	_, e = s.CreateAPIKey(storage.WithTenant(ctx, other, otherOrgs[0].ID), "Other", tokenHash(secret), secret[:15], []string{"subscriptions:read", "subscriptions:write", "subscriptions:export"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	assert(call("GET", path, "", secret, ""), 404)
	assert(call("GET", path+"/config", "", secret, ""), 404)
	assert(call("PATCH", path, body, secret, `"2"`), 404)
	assert(call("GET", path+"/content?token=bad", "", export, ""), 403)
	assert(call("PUT", path, `{}`, write, ""), 428)
	created := call("POST", "/api/v1/subscriptions", `{"name":"API created","format":"stash","enabled":true,"node_ids":["`+node+`"],"rules":[],"final_action":"proxy"}`, write, "")
	assert(created, 201)
	if strings.Contains(created.Body.String(), "subscription_path") || strings.Contains(created.Body.String(), "token=") {
		t.Fatal("write-only creation revealed bearer link")
	}
	// Both writers read the same revision; exactly one may commit.
	fresh := call("GET", path, "", read, "")
	assert(fresh, 200)
	match := fresh.Header().Get("ETag")
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"Writer A", "Writer B"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			results <- call("PATCH", path, `{"name":"`+name+`"}`, write, match).Code
		}(name)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for code := range results {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("lost update protection", counts)
	}
}
