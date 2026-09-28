package httpapi

import (
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

func TestAccountHTTPPasswordBoundary(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
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
	s, err := storage.Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	password := "original-password-fixture"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	username := "security_" + storage.NewID()[:8]
	id, err := s.Register(ctx, username, string(hash), "Security test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		db.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
	}()
	token := strings.Repeat("e", 64)
	if err = s.NewSession(ctx, tokenHash(token), id, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	h := New(s, Options{PublicOrigin: "http://127.0.0.1:15173"})
	request := func(method, path, body, csrf string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", "http://127.0.0.1:15173")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s got %d want %d: %s", path, w.Code, want, w.Body.String())
		}
		return w
	}
	request("POST", "/api/v1/account/password", `{"current_password":"original-password-fixture","new_password":"replacement-password-fixture"}`, "wrong", 403)
	request("POST", "/api/v1/account/password", `{"current_password":"incorrect","new_password":"replacement-password-fixture"}`, csrfToken(token), 422)
	request("PUT", "/api/v1/account/profile", `{"display_name":"Profile fixture"}`, csrfToken(token), 204)
	response := request("GET", "/api/v1/account/sessions", "", "", 200)
	if strings.Contains(response.Body.String(), token) || strings.Contains(response.Body.String(), tokenHash(token)) {
		t.Fatal("session bearer material exposed")
	}
	var data struct{ Data []storage.AccountSession }
	if err = json.Unmarshal(response.Body.Bytes(), &data); err != nil || len(data.Data) != 1 || !data.Data[0].Current {
		t.Fatal(data, err)
	}
	request("POST", "/api/v1/account/password", `{"current_password":"original-password-fixture","new_password":"replacement-password-fixture"}`, csrfToken(token), 204)
	request("GET", "/api/v1/account/sessions", "", "", 401)
	request("POST", "/api/v1/auth/login", `{"username":"`+username+`","password":"original-password-fixture"}`, csrfToken(token), 401)
	login := request("POST", "/api/v1/auth/login", `{"username":"`+username+`","password":"replacement-password-fixture"}`, csrfToken(token), 200)
	if len(login.Result().Cookies()) == 0 {
		t.Fatal("login did not issue replacement session")
	}
	credentials, err := s.Credentials(ctx, username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte("replacement-password-fixture")) != nil {
		t.Fatal("replacement password not stored", err)
	}
}
