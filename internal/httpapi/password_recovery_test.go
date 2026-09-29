package httpapi

import (
	"context"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/storage"
)

type recoveryFixture struct {
	Store
	token, code, email   string
	delivered, completed bool
}

func (s *recoveryFixture) BeginPasswordRecovery(_ context.Context, token, email, code string) error {
	s.token = token
	s.code = code
	s.email = email
	return nil
}
func (s *recoveryFixture) DeliverPasswordRecovery(_ context.Context, token string) error {
	s.delivered = token == s.token
	return nil
}
func (s *recoveryFixture) CompletePasswordRecovery(_ context.Context, token, code, hash string) error {
	if !s.delivered || s.completed || token != s.token || code != s.code {
		return storage.ErrVerification
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("new-password-fixture")) != nil {
		return storage.ErrInvalid
	}
	s.completed = true
	return nil
}

type recoveryMailer struct {
	code string
	fail bool
}

func (m *recoveryMailer) Configured() bool                                   { return true }
func (m *recoveryMailer) Send(context.Context, string, string, string) error { return nil }
func (m *recoveryMailer) SendMessage(_ context.Context, _ string, msg emailverification.Message, _ string) error {
	m.code = msg.Code
	if m.fail {
		return emailverification.ErrUnavailable
	}
	return nil
}
func TestPasswordRecoveryHTTP(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "provider failure"}[failed], func(t *testing.T) {
			store := &recoveryFixture{}
			mailer := &recoveryMailer{fail: failed}
			handler := New(store, Options{PublicOrigin: "https://xingdu.example", EmailSender: mailer})
			call := func(path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", path, strings.NewReader(body))
				r.Header.Set("Origin", "https://xingdu.example")
				r.Header.Set("X-Xingdu-Request", "1")
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				return w
			}
			w := call("/api/v1/auth/password-recovery", `{"email":"fixture@example.invalid"}`)
			if failed {
				if w.Code != 503 || store.delivered {
					t.Fatal("provider failure activated recovery", w.Code)
				}
				return
			}
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var out struct {
				Data struct {
					Token string `json:"recovery_token"`
				}
			}
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if out.Data.Token == store.token || len(out.Data.Token) != 64 || store.code == mailer.code {
				t.Fatal("raw token/code stored")
			}
			body, _ := json.Marshal(map[string]string{"recovery_token": out.Data.Token, "code": mailer.code, "new_password": "new-password-fixture"})
			w = call("/api/v1/auth/password-recovery/complete", string(body))
			if w.Code != http.StatusNoContent {
				t.Fatal(w.Code, w.Body.String())
			}
			if w = call("/api/v1/auth/password-recovery/complete", string(body)); w.Code != 422 {
				t.Fatal("replayed recovery succeeded")
			}
		})
	}
}
