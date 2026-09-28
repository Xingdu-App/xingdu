package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
)

type subscriptionFake struct {
	fakeStore
	expected string
	calls    int
	disabled bool
}

func (s *subscriptionFake) SubscriptionContent(_ context.Context, _ string, hash string) (storage.Subscription, []storage.Deployment, error) {
	s.calls++
	if s.disabled || hash != s.expected {
		return storage.Subscription{}, nil, storage.ErrNotFound
	}
	return storage.Subscription{Name: "test", Format: "stash", Enabled: true}, []storage.Deployment{}, nil
}
func TestSubscriptionPublicCapabilityHeaders(t *testing.T) {
	token := machine.Token()
	store := &subscriptionFake{expected: machine.Hash(token)}
	h := New(store, Options{})
	base := "/api/v1/subscriptions/" + storage.NewID("sub") + "/content?format=mihomo&token="
	for _, test := range []struct {
		token string
		want  int
	}{{"bad", 404}, {machine.Token(), 404}, {token, 409}} {
		r := httptest.NewRequest("GET", base+test.token, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("status %d want %d: %s", w.Code, test.want, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatal("capability content cacheable")
		}
		if strings.Contains(w.Body.String(), token) {
			t.Fatal("token leaked in error")
		}
	}
	if store.calls != 2 {
		t.Fatal("malformed token queried storage")
	}
	store.disabled = true
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", base+token, nil))
	if w.Code != http.StatusNotFound {
		t.Fatal(w.Code)
	}
}
func TestSubscriptionPublicRateLimit(t *testing.T) {
	store := &subscriptionFake{}
	h := New(store, Options{})
	path := "/api/v1/subscriptions/" + storage.NewID("sub") + "/content?token=bad"
	for i := 0; i < 121; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if i == 120 && w.Code != 429 {
			t.Fatal("no rate limit", w.Code)
		}
	}
}
func TestSubscriptionMetadataNeverSerializesCapability(t *testing.T) {
	b, err := json.Marshal(storage.Subscription{ID: storage.NewID("sub"), Name: "Example"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"token", "encrypted", "password", "credential"} {
		if strings.Contains(string(b), key) {
			t.Fatal("secret field", key)
		}
	}
}

func TestClientFormatBoundary(t *testing.T) {
	token := machine.Token()
	store := &subscriptionFake{expected: machine.Hash(token)}
	h := New(store, Options{})
	for _, tc := range []struct {
		format string
		status int
	}{
		{"surge", 409}, {"hysteria2_uri", 409}, {"loon", 422}, {"shadowrocket", 422}, {"unknown", 422},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/subscriptions/"+storage.NewID("sub")+"/content?token="+token+"&format="+tc.format, nil))
		if w.Code != tc.status {
			t.Fatalf("%s got %d want %d", tc.format, w.Code, tc.status)
		}
		if strings.Contains(w.Body.String(), token) {
			t.Fatal("token leaked")
		}
	}
}
