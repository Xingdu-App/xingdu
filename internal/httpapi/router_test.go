package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

type fakeStore struct {
	Store
	err error
}

func (s fakeStore) Ready(context.Context) error { return s.err }
func (s fakeStore) Session(context.Context, string) (storage.Admin, error) {
	return storage.Admin{Username: "test"}, nil
}
func TestHealthAndAuthBoundaries(t *testing.T) {
	h := New(fakeStore{err: errors.New("private database error")}, Options{PublicOrigin: "http://127.0.0.1:15173"})
	for path, want := range map[string]int{"/health/live": 200, "/health/ready": 503, "/api/v1/hosts": 401, "/api/v1/system": 401} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Fatalf("%s: got %d want %d", path, w.Code, want)
		}
		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("database details leaked")
		}
	}
}
func TestWriteProtection(t *testing.T) {
	h := New(fakeStore{}, Options{PublicOrigin: "http://127.0.0.1:15173", SecureCookies: true})
	for _, tc := range []struct {
		origin, marker, content, csrf string
		want                          int
	}{
		{"https://attacker.example", "1", "application/json", "", 403},
		{"http://127.0.0.1:15173", "", "application/json", "", 403},
		{"http://127.0.0.1:15173", "1", "text/plain", "", 415},
		{"http://127.0.0.1:15173", "1", "application/json", "wrong", 403},
	} {
		r := httptest.NewRequest("POST", "/api/v1/hosts", strings.NewReader(`{}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Xingdu-Request", tc.marker)
		r.Header.Set("Content-Type", tc.content)
		r.Header.Set("X-CSRF-Token", tc.csrf)
		r.AddCookie(&http.Cookie{Name: cookieName, Value: strings.Repeat("a", 64)})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
		}
	}
}
func TestLimiterBounded(t *testing.T) {
	l := limiter{entries: make(map[string]attempt)}
	for range 10 {
		if !l.allow("one") {
			t.Fatal("blocked early")
		}
	}
	if l.allow("one") {
		t.Fatal("limit bypassed")
	}
}
