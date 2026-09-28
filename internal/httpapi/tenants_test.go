package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

func TestOrganizationsAndInvitations(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated test database")
	}
	ctx := context.Background()
	db, err := storage.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	uri, _ := url.Parse(raw)
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	runtime, err := storage.Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	h := New(runtime, Options{PublicOrigin: "http://127.0.0.1:15173", RegistrationEnabled: true})
	type client struct {
		cookie    *http.Cookie
		csrf, org string
	}
	request := func(c *client, method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(body)
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Origin", "http://127.0.0.1:15173")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("X-CSRF-Token", c.csrf)
		r.Header.Set("X-Xingdu-Organization", c.org)
		if c.cookie != nil {
			r.AddCookie(c.cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: want %d got %d %s", method, path, want, w.Code, w.Body.String())
		}
		return w
	}
	type session struct {
		Data struct {
			ID   string `json:"id"`
			CSRF string `json:"csrf_token"`
		}
	}
	signup := func() (*client, string) {
		t.Helper()
		c := &client{}
		name := "api_" + storage.NewID()[:8]
		body := map[string]string{"username": name, "password": "integration-test-password", "organization": "API test"}
		request(c, "POST", "/api/v1/auth/register", body, 201)
		delete(body, "organization")
		w := request(c, "POST", "/api/v1/auth/login", body, 200)
		var s session
		json.Unmarshal(w.Body.Bytes(), &s)
		c.cookie = w.Result().Cookies()[0]
		c.csrf = s.Data.CSRF
		t.Cleanup(func() {
			db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", s.Data.ID)
			db.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", s.Data.ID)
		})
		w = request(c, "GET", "/api/v1/organizations", nil, 200)
		var orgs struct{ Data []storage.Organization }
		json.Unmarshal(w.Body.Bytes(), &orgs)
		if len(orgs.Data) != 1 {
			t.Fatal("unexpected organizations")
		}
		c.org = orgs.Data[0].ID
		return c, s.Data.ID
	}
	a, aid := signup()
	b, bid := signup()
	ownB := b.org
	b.org = a.org
	request(b, "GET", "/api/v1/hosts", nil, 403)
	request(b, "GET", "/api/v1/members", nil, 403)
	request(b, "GET", "/api/v1/invitations", nil, 403)
	request(a, "POST", "/api/v1/invitations", map[string]string{"role": "owner"}, 422)
	w := request(a, "POST", "/api/v1/invitations", map[string]string{"role": "viewer"}, 201)
	var invite struct {
		Data struct {
			URL string `json:"url"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &invite)
	if !strings.Contains(invite.Data.URL, "/app#invite=") {
		t.Fatalf("invitation must target console: %s", invite.Data.URL)
	}
	token := strings.Split(invite.Data.URL, "#invite=")[1]
	request(b, "POST", "/api/v1/invitations/accept", map[string]string{"token": token}, 200)
	request(b, "POST", "/api/v1/invitations/accept", map[string]string{"token": token}, 404)
	request(b, "GET", "/api/v1/members", nil, 200)
	request(b, "POST", "/api/v1/invitations", map[string]string{"role": "member"}, 403)
	body := map[string]any{"name": "test", "address": "api.example.invalid", "ssh_port": 22, "ssh_user": "root", "tags": []string{}, "notes": ""}
	request(b, "POST", "/api/v1/hosts", body, 403)
	request(a, "PUT", "/api/v1/members/"+bid, map[string]string{"role": "member"}, 204)
	request(b, "POST", "/api/v1/hosts", body, 201)
	request(b, "PUT", "/api/v1/members/"+aid, map[string]string{"role": "viewer"}, 403)
	request(a, "DELETE", "/api/v1/members/"+aid, nil, 403)
	request(a, "DELETE", "/api/v1/members/"+bid, nil, 204)
	request(b, "GET", "/api/v1/hosts", nil, 403)
	b.org = ownB
	w = request(b, "GET", "/api/v1/hosts", nil, 200)
	if strings.Contains(w.Body.String(), "api.example.invalid") {
		t.Fatal("inventory crossed tenants")
	}
}
