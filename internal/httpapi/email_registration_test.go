package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/storage"
)

type fixtureEmailSender struct {
	code string
	fail bool
}

func (s *fixtureEmailSender) Configured() bool { return true }
func (s *fixtureEmailSender) Send(_ context.Context, _, code, _ string) error {
	s.code = code
	if s.fail {
		return errors.New("private mail credentials")
	}
	return nil
}
func TestEmailRegistrationRequiresDeliveryAndVerification(t *testing.T) {
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
	email := "verify_" + storage.NewID()[:8] + "@example.invalid"
	defer func() {
		db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by IN (SELECT id FROM users WHERE username=$1)", email)
		db.Pool.Exec(ctx, "DELETE FROM users WHERE username=$1", email)
		db.Pool.Exec(ctx, "DELETE FROM pending_registrations WHERE email=$1", email)
	}()
	mail := &fixtureEmailSender{}
	h := New(s, Options{PublicOrigin: "http://127.0.0.1:15173", RegistrationEnabled: true, EmailSender: mail})
	request := func(handler http.Handler, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Origin", "http://127.0.0.1:15173")
		r.Header.Set("X-Xingdu-Request", "1")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s got%d want%d %s", path, w.Code, want, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private mail") {
			t.Fatal("mail error leaked")
		}
		return w
	}
	body := `{"email":"` + email + `","password":"fixture-password-123","organization":"Verify test"}`
	request(New(s, Options{PublicOrigin: "http://127.0.0.1:15173", RegistrationEnabled: true}), "/api/v1/auth/register", body, 503)
	w := request(h, "/api/v1/auth/register", body, 202)
	var result struct {
		Data struct {
			Token string `json:"registration_token"`
		}
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Data.Token) != 64 || len(mail.code) != 8 || strings.Contains(w.Body.String(), mail.code) {
		t.Fatal("bad challenge response")
	}
	if _, err = s.Credentials(ctx, email); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("unverified user created", err)
	}
	request(h, "/api/v1/auth/login", `{"username":"`+email+`","password":"fixture-password-123"}`, 401)
	request(h, "/api/v1/auth/register", body, 429)
	request(h, "/api/v1/auth/register/verify", `{"registration_token":"`+result.Data.Token+`","code":"wrong"}`, 422)
	verify := `{"registration_token":"` + result.Data.Token + `","code":"` + mail.code + `"}`
	request(h, "/api/v1/auth/register/verify", verify, 201)
	request(h, "/api/v1/auth/register/verify", verify, 422)
	request(h, "/api/v1/auth/login", `{"username":"`+strings.ToUpper(email)+`","password":"fixture-password-123"}`, 200)
	var verified bool
	if err = db.Pool.QueryRow(ctx, "SELECT email_verified_at IS NOT NULL AND email=$1 FROM users WHERE username=$1", email).Scan(&verified); err != nil || !verified {
		t.Fatal("email not markedverified", err)
	}
	// Provider failure never activates the saved challenge.
	db.Pool.Exec(ctx, "DELETE FROM pending_registrations WHERE email=$1", email)
	mail.fail = true
	request(h, "/api/v1/auth/register", body, 503)
	var active bool
	db.Pool.QueryRow(ctx, "SELECT delivered FROM pending_registrations WHERE email=$1", email).Scan(&active)
	if active {
		t.Fatal("failed delivery activated challenge")
	}
}

func TestEmailConfigurationPlaceholderFailClosed(t *testing.T) {
	sender := emailverification.NewResend("PLACEHOLDER_REPLACE_WITH_RESEND_API_KEY", "Xingdu <noreply@xingdu.app>")
	handler := New(fakeStore{}, Options{PublicOrigin: "https://xingdu.zeabur.app", RegistrationEnabled: true, EmailSender: sender})
	config := httptest.NewRecorder()
	handler.ServeHTTP(config, httptest.NewRequest("GET", "/api/v1/auth/config", nil))
	var result struct{ Data map[string]any }
	if json.Unmarshal(config.Body.Bytes(), &result) != nil || config.Code != 200 || result.Data["registration_enabled"] != true || result.Data["email_verification_required"] != true || result.Data["email_delivery_configured"] != false {
		t.Fatal("unexpected placeholder configuration", config.Body.String())
	}
	request := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{"email":"fixture@example.invalid","password":"fixture-password","organization":"Fixture"}`))
	request.Header.Set("Origin", "https://xingdu.zeabur.app")
	request.Header.Set("X-Xingdu-Request", "1")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 503 || !strings.Contains(response.Body.String(), `"code":"email_unavailable"`) {
		t.Fatal("placeholder registration did not fail closed", response.Code, response.Body.String())
	}
}
