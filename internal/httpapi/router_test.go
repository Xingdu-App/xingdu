package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

type fakeStore struct{ err error }

func (s fakeStore) Ready(context.Context) error                   { return s.err }
func (s fakeStore) Hosts(context.Context) ([]storage.Host, error) { return []storage.Host{}, s.err }
func TestHealthSeparatesProcessFromDatabase(t *testing.T) {
	handler := New(fakeStore{err: errors.New("secret connection string")})
	for path, want := range map[string]int{"/health/live": 200, "/health/ready": 503, "/api/v1/hosts": 503} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Fatalf("%s: got %d want %d", path, w.Code, want)
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("database error leaked")
		}
	}
}
func TestHostsEmptyArrayAndReadOnly(t *testing.T) {
	h := New(fakeStore{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/hosts", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/hosts", nil))
	if w.Code != 405 {
		t.Fatalf("write endpoint unexpectedly enabled: %d", w.Code)
	}
}
