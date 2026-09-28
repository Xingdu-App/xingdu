package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/storage"
)

func TestAuthenticatedInventory(t *testing.T) {
	databaseURL := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set XINGDU_TEST_DATABASE_URL for database integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := storage.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	username := "test_" + strings.ReplaceAll(storage.NewID("obj"), "-", "")[:12]
	password := "test-only-password-2026"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err := s.CreateAdmin(ctx, username, string(hash)); err != nil {
		t.Fatal("dedicated test database must not contain an administrator:", err)
	}
	defer func() {
		s.Pool.Exec(context.Background(), "DELETE FROM organizations WHERE created_by=(SELECT id FROM users WHERE username=$1)", username)
		s.Pool.Exec(context.Background(), "DELETE FROM users WHERE username=$1", username)
	}()
	admin, _ := s.Credentials(ctx, username)
	organizations, _ := s.Organizations(ctx, admin.ID)
	runtimeURL, _ := url.Parse(databaseURL)
	query := runtimeURL.Query()
	query.Set("options", "-crole=xingdu_app")
	runtimeURL.RawQuery = query.Encode()
	runtime, err := storage.Open(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	h := New(runtime, Options{PublicOrigin: "http://127.0.0.1:15173", SecureCookies: true})
	var cookie *http.Cookie
	csrf := ""
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&b).Encode(body)
		}
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("X-Xingdu-Organization", organizations[0].ID)
		r.Header.Set("Origin", "http://127.0.0.1:15173")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("got %d want %d: %s", w.Code, status, w.Body.String())
		}
	}
	expect(request("GET", "/api/v1/hosts", nil), 401)
	expect(request("POST", "/api/v1/auth/login", map[string]string{"username": username, "password": "wrong"}), 401)
	login := request("POST", "/api/v1/auth/login", map[string]string{"username": username, "password": password})
	expect(login, 200)
	cookie = login.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || len(cookie.Value) != 64 {
		t.Fatal("unsafe session cookie")
	}
	var session struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		}
	}
	_ = json.Unmarshal(login.Body.Bytes(), &session)
	csrf = session.Data.CSRF
	// Only token digests are stored, not bearer tokens.
	var stored string
	if err := s.Pool.QueryRow(ctx, "SELECT token_hash FROM sessions WHERE user_id=(SELECT id FROM users WHERE username=$1)", username).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == cookie.Value || stored != tokenHash(cookie.Value) {
		t.Fatal("unexpected session storage")
	}
	expect(request("GET", "/api/v1/auth/session", nil), 200)
	body := map[string]any{"name": "测试服务器", "address": strings.ReplaceAll(username, "_", "-") + ".example.invalid", "ssh_port": 22, "ssh_user": "root", "tags": []string{"test"}, "notes": "integration"}
	savedCSRF := csrf
	csrf = "bad"
	expect(request("POST", "/api/v1/hosts", body), 403)
	csrf = savedCSRF
	created := request("POST", "/api/v1/hosts", body)
	expect(created, 201)
	var result struct{ Data storage.Host }
	_ = json.Unmarshal(created.Body.Bytes(), &result)
	id := result.Data.ID
	defer s.Pool.Exec(context.Background(), "DELETE FROM hosts WHERE id=$1", id)
	if result.Data.Status != "pending" || result.Data.LastSeenAt != nil {
		t.Fatal("inventory falsely reports connected host")
	}
	expect(request("POST", "/api/v1/hosts", body), 409)
	body["name"] = "重命名服务器"
	expect(request("PUT", "/api/v1/hosts/"+id, body), 200)
	body["address"] = "https://invalid.example"
	expect(request("PUT", "/api/v1/hosts/"+id, body), 422)
	body["status"] = "online"
	expect(request("PUT", "/api/v1/hosts/"+id, body), 400)
	list := request("GET", "/api/v1/hosts", nil)
	expect(list, 200)
	if !strings.Contains(list.Body.String(), "重命名服务器") {
		t.Fatal("updated host missing")
	}
	expect(request("DELETE", "/api/v1/hosts/"+id, nil), 204)
	expect(request("DELETE", "/api/v1/hosts/"+id, nil), 404)
	expect(request("POST", "/api/v1/auth/logout", nil), 204)
	expect(request("GET", "/api/v1/auth/session", nil), 401)
	if _, err := s.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,expires_at) SELECT $1,id,now()-interval '1 second' FROM users WHERE username=$2", tokenHash(cookie.Value), username); err != nil {
		t.Fatal(err)
	}
	expect(request("GET", "/api/v1/hosts", nil), 401)
}
