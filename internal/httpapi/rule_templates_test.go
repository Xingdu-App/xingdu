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

func TestRuleTemplateHTTP(t *testing.T) {
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
	_, otherOrg := newUser()
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
	catalog := call("GET", "/api/v1/subscription-presets", "", "", true, nil)
	assert(catalog, 200)
	if !strings.Contains(catalog.Body.String(), "streaming-v1") || !strings.Contains(catalog.Body.String(), "ChinaDomain.list") {
		t.Fatal("missing template catalog")
	}
	assert(call("GET", "/api/v1/subscription-presets", "", "", false, nil), 401)
	body := `{"name":"Daily","rules":[{"type":"domain_suffix","value":"example.com","target":"direct"}],"final_action":"proxy"}`
	assert(call("POST", "/api/v1/rule-templates", body, "", false, nil), 403)
	w := call("POST", "/api/v1/rule-templates", body, "", true, nil)
	assert(w, 201)
	var out struct {
		Data storage.RuleTemplate `json:"data"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/rule-templates/" + out.Data.ID
	assert(call("GET", "/api/v1/rule-templates", "", "", true, nil), 200)
	assert(call("PUT", path, body, "", true, nil), 200)
	assert(call("PUT", path, body, "", true, map[string]string{"X-Xingdu-Organization": otherOrg}), 403)
	assert(call("PUT", path, `{"name":"Bad","rules":[],"final_action":"reject"}`, "", true, nil), 422)
	assert(call("DELETE", path, "", "", true, nil), 204)
	assert(call("DELETE", path, "", "", true, nil), 404)
}
