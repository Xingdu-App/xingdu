package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"xingdu.app/xingdu/internal/storage"
)

func TestAPIKeyScopeAllowlist(t *testing.T) {
	sub := storage.NewID("sub")
	host := storage.NewID("srv")
	node := storage.NewID("node")
	cases := []struct{ method, path, want string }{
		{"GET", "/api/v1/subscriptions/" + sub, "subscriptions:read"},
		{"PATCH", "/api/v1/subscriptions/" + sub, "subscriptions:write"},
		{"GET", "/api/v1/subscriptions/" + sub + "/config", "subscriptions:export"},
		{"POST", "/api/v1/subscriptions/" + sub + "/preview", "subscriptions:export"},
		{"GET", "/api/v1/subscriptions/" + sub + "/content", ""},
		{"POST", "/api/v1/subscriptions/" + sub + "/rotate", ""},
		{"GET", "/api/v1/hosts", "hosts:read"}, {"POST", "/api/v1/hosts", "hosts:write"},
		{"PUT", "/api/v1/hosts/" + host, "hosts:write"}, {"GET", "/api/v1/nodes", "nodes:read"},
		{"POST", "/api/v1/hosts/" + host + "/deployments", "nodes:write"},
		{"POST", "/api/v1/hosts/" + host + "/deployments/preflight", "nodes:write"},
		{"POST", "/api/v1/hosts/" + host + "/deployments/" + node + "/restart", "nodes:write"},
		{"DELETE", "/api/v1/hosts/" + host + "/deployments/" + node, "nodes:write"},
		{"POST", "/api/v1/hosts/" + host + "/deployments/" + node + "/connection", "nodes:credentials"},
		{"PATCH", "/api/v1/api-keys/" + storage.NewID("key"), ""},
		{"GET", "/api/v1/api-keys", ""}, {"POST", "/api/v1/auth/logout", ""},
		{"POST", "/api/v1/hosts/" + host + "/ssh", ""}, {"GET", "/api/v1/hosts/../api-keys", ""},
		{"GET", "/api/v1/hosts/", ""}, {"POST", "/api/v1/agent/enroll", ""},
	}
	for _, c := range cases {
		if got := apiScope(c.method, c.path); got != c.want {
			t.Errorf("%s %s got %q want %q", c.method, c.path, got, c.want)
		}
	}
}
func TestAPIKeyManagementAndIsolation(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated test database")
	}
	ctx := context.Background()
	owner, err := storage.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err = owner.Migrate(ctx); err != nil {
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
	users := []string{}
	defer func() {
		for _, user := range users {
			owner.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", user)
		}
		for _, user := range users {
			owner.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", user)
		}
	}()
	newUser := func() (string, string) {
		user, e := s.Register(ctx, "test_"+storage.NewID("obj"), "test-only-hash", "Key test")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, user)
		orgs, e := s.Organizations(ctx, user)
		if e != nil {
			t.Fatal(e)
		}
		return user, orgs[0].ID
	}
	user, org := newUser()
	other, otherOrg := newUser()
	session := strings.Repeat("a", 64)
	if err = s.NewSession(ctx, tokenHash(session), user, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	h := New(s, Options{PublicOrigin: "https://console.example"})
	call := func(method, path, body, bearer string, browser bool, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if browser {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: session})
			r.Header.Set("Origin", "https://console.example")
			r.Header.Set("X-Xingdu-Request", "1")
			r.Header.Set("X-CSRF-Token", csrfToken(session))
			r.Header.Set("X-Xingdu-Organization", org)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	assert := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("HTTP %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	create := func(scopes string) (storage.APIKey, string) {
		w := call("POST", "/api/v1/api-keys", `{"name":"Automation","scopes":`+scopes+`,"expires_in_days":90}`, "", true, nil)
		assert(w, 201)
		var out struct {
			Data struct {
				Key    storage.APIKey `json:"key"`
				Secret string         `json:"secret"`
			} `json:"data"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out.Data.Key, out.Data.Secret
	}
	key, secret := create(`["hosts:read","nodes:read"]`)
	var hash string
	if err = owner.Pool.QueryRow(ctx, "SELECT token_hash FROM api_keys WHERE id=$1", key.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash != tokenHash(secret) || strings.Contains(hash, secret) {
		t.Fatal("secret was not hashed")
	}
	w := call("GET", "/api/v1/api-keys", "", "", true, nil)
	assert(w, 200)
	if strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), hash) {
		t.Fatal("list disclosed credential")
	}
	assert(call("GET", "/api/v1/hosts", "", secret, false, nil), 200)
	assert(call("GET", "/api/v1/nodes", "", secret, false, nil), 200)
	assert(call("POST", "/api/v1/hosts", "{}", secret, false, nil), 403)
	assert(call("GET", "/api/v1/hosts", "", secret, false, map[string]string{"X-Xingdu-Organization": otherOrg}), 403)
	assert(call("GET", "/api/v1/api-keys", "", secret, false, nil), 403)
	assert(call("GET", "/api/v1/auth/session", "", secret, false, nil), 403)
	assert(call("GET", "/api/v1/hosts", "", secret, true, nil), 403)
	assert(call("GET", "/api/v1/hosts", "", secret, false, map[string]string{"Origin": "https://console.example"}), 403)
	assert(call("GET", "/api/v1/hosts", "", "xd_key_"+strings.Repeat("0", 64), false, nil), 401)
	assert(call("POST", "/api/v1/api-keys", "{}", "", true, map[string]string{"X-CSRF-Token": ""}), 403)
	// The runtime role cannot enumerate keys without a tenant or credential hash.
	var count int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM api_keys").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unscoped RLS %d %v", count, err)
	}
	if err = s.RevokeAPIKey(storage.WithTenant(ctx, other, otherOrg), key.ID); err == nil {
		t.Fatal("cross tenant revoke succeeded")
	}
	// Scope changes preserve the secret and apply to subsequent requests.
	path := "/api/v1/api-keys/" + key.ID
	assert(call("PATCH", path, `{"scopes":[]}`, "", true, nil), 400)
	assert(call("PATCH", path, `{"scopes":["unknown"]}`, "", true, nil), 400)
	assert(call("PATCH", path, `{"scopes":["hosts:read","hosts:read"]}`, "", true, nil), 400)
	assert(call("PATCH", path, `{"scopes":["hosts:read"]}`, "", true, map[string]string{"X-CSRF-Token": ""}), 403)
	assert(call("PATCH", path, `{"scopes":["hosts:read"]}`, secret, false, nil), 403)
	if _, err = s.UpdateAPIKeyScopes(storage.WithTenant(ctx, other, otherOrg), key.ID, []string{"hosts:read"}); err != storage.ErrNotFound {
		t.Fatalf("cross tenant scope update: %v", err)
	}
	w = call("PATCH", path, `{"scopes":["hosts:read"]}`, "", true, nil)
	assert(w, 200)
	if strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), hash) {
		t.Fatal("update disclosed credential")
	}
	assert(call("GET", "/api/v1/nodes", "", secret, false, nil), 403)
	assert(call("GET", "/api/v1/hosts", "", secret, false, nil), 200)
	if _, err = s.Hosts(storage.WithAPIKeyTenant(ctx, key)); err != storage.ErrForbidden {
		t.Fatalf("stale scopes accepted: %v", err)
	}
	assert(call("PATCH", path, `{"scopes":["hosts:read","nodes:read"]}`, "", true, nil), 200)
	assert(call("GET", "/api/v1/nodes", "", secret, false, nil), 200)
	writer, writeSecret := create(`["hosts:read","hosts:write","nodes:write"]`)
	payload := `{"name":"API host","address":"vps.example.com","ssh_port":22,"ssh_user":"root","tags":[],"notes":""}`
	w = call("POST", "/api/v1/hosts", payload, writeSecret, false, nil)
	assert(w, 201)
	var host struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &host)
	assert(call("PUT", "/api/v1/hosts/"+host.Data.ID, payload, writeSecret, false, nil), 200)
	assert(call("GET", "/api/v1/hosts/"+host.Data.ID+"/deployments", "", writeSecret, false, nil), 403)
	assert(call("POST", "/api/v1/hosts/"+host.Data.ID+"/deployments/"+storage.NewID("node")+"/connection", "{}", writeSecret, false, nil), 403)
	assert(call("DELETE", "/api/v1/hosts/"+host.Data.ID, "{}", writeSecret, false, nil), 204)
	assert(call("DELETE", "/api/v1/api-keys/"+writer.ID, "", "", true, nil), 204)
	assert(call("GET", "/api/v1/hosts", "", writeSecret, false, nil), 401)
	assert(call("PATCH", "/api/v1/api-keys/"+writer.ID, `{"scopes":["hosts:read"]}`, "", true, nil), 404)
	// An already authenticated context must also fail once revocation commits.
	if _, err = s.Hosts(storage.WithAPIKeyTenant(ctx, writer)); err != storage.ErrForbidden {
		t.Fatalf("stale principal accepted: %v", err)
	}
	// Downgrading or removing the issuing administrator invalidates their key.
	if _, err = owner.Pool.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'admin')", org, other); err != nil {
		t.Fatal(err)
	}
	adminKey, err := s.CreateAPIKey(storage.WithTenant(ctx, other, org), "Admin", tokenHash("admin-token"), "test", []string{"hosts:read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AuthenticateAPIKey(ctx, tokenHash("admin-token")); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeMember(storage.WithTenant(ctx, user, org), other, "member", false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AuthenticateAPIKey(ctx, tokenHash("admin-token")); err != storage.ErrForbidden {
		t.Fatalf("downgraded principal: %v", err)
	}
	if _, err = s.Hosts(storage.WithAPIKeyTenant(ctx, adminKey)); err != storage.ErrForbidden {
		t.Fatalf("stale downgraded principal: %v", err)
	}
	if _, err = s.UpdateAPIKeyScopes(storage.WithTenant(ctx, other, org), key.ID, []string{"hosts:read"}); err != storage.ErrForbidden {
		t.Fatalf("member updated scopes: %v", err)
	}
	if _, err = s.CreateAPIKey(storage.WithTenant(ctx, other, org), "Denied", tokenHash("denied"), "test", []string{"hosts:read"}, time.Now().Add(time.Hour)); err != storage.ErrForbidden {
		t.Fatalf("member created key: %v", err)
	}
	if _, err = owner.Pool.Exec(ctx, "UPDATE api_keys SET expires_at=now()-interval '1 minute' WHERE id=$1", key.ID); err != nil {
		t.Fatal(err)
	}
	assert(call("PATCH", path, `{"scopes":["hosts:read"]}`, "", true, nil), 404)
	assert(call("GET", "/api/v1/hosts", "", secret, false, nil), 401)
	var used *time.Time
	if err = owner.Pool.QueryRow(ctx, "SELECT last_used_at FROM api_keys WHERE id=$1", key.ID).Scan(&used); err != nil || used == nil {
		t.Fatalf("last used missing: %v", err)
	}
}

type keyBoundaryStore struct {
	fakeStore
	key     storage.APIKey
	authErr error
}

func (s keyBoundaryStore) AuthenticateAPIKey(context.Context, string) (storage.APIKey, error) {
	return s.key, s.authErr
}
func (s keyBoundaryStore) Hosts(context.Context) ([]storage.Host, error) {
	return []storage.Host{}, nil
}
func TestAPIKeyRateLimitAndUnavailableStore(t *testing.T) {
	s := keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), CreatedBy: storage.NewID("usr"), OrganizationID: storage.NewID("org"), Scopes: []string{"hosts:read"}}}
	h := New(s, Options{})
	send := func(handler http.Handler) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
		r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for i := 0; i < 120; i++ {
		if w := send(h); w.Code != 200 {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if w := send(h); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("missing rate limit")
	}
	s.authErr = context.DeadlineExceeded
	if w := send(New(s, Options{})); w.Code != 503 {
		t.Fatalf("database error classified as %d", w.Code)
	}
}
