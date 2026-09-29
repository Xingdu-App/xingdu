package httpapi

import (
	"context"
	"encoding/json"
	"mime"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/subscription"
	"xingdu.app/xingdu/internal/vault"
)

type routingContentFake struct {
	fakeStore
	sub        storage.Subscription
	deployment storage.Deployment
	hash       string
}

func (s *routingContentFake) SubscriptionContent(_ context.Context, _ string, hash string) (storage.Subscription, []storage.Deployment, error) {
	if hash != s.hash {
		return storage.Subscription{}, nil, storage.ErrNotFound
	}
	return s.sub, []storage.Deployment{s.deployment}, nil
}

func TestSubscriptionContentUsesRoutingAndRejectsURIOverride(t *testing.T) {
	v, err := vault.New(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := protocol.NewSpec(protocol.Input{Name: "Fixture", Protocol: "shadowsocks", Port: 8388})
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(spec)
	d := storage.Deployment{ID: storage.NewID("node"), HostID: storage.NewID("srv"), OrgID: storage.NewID("org"), Name: "Fixture", Server: "192.0.2.10"}
	d.Encrypted = v.Seal(plain, deploymentAAD(d.OrgID, d.HostID, d.ID))
	token := machine.Token()
	store := &routingContentFake{hash: machine.Hash(token), deployment: d, sub: storage.Subscription{ID: storage.NewID("sub"), Name: "日常订阅 Home", Format: "stash", FinalAction: "proxy", NodeIDs: []string{d.ID}, Rules: []subscription.Rule{{Type: "domain", Value: "example.com", Target: "group:work"}}, Routing: &subscription.Routing{Preset: "balanced-v1", Groups: []subscription.RoutingGroup{{ID: "proxy", Name: "Default", Type: "url-test"}, {ID: "work", Name: "Work", Type: "select", NodeIDs: []string{d.ID}}}, Targets: map[string]string{"proxy": "group:work"}, Final: "group:proxy"}}}
	h := New(store, Options{CredentialVault: v})
	path := "/api/v1/subscriptions/" + store.sub.ID + "/content?token=" + token
	for _, format := range []string{"stash", "mihomo", "surge"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path+"&format="+format, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", format, w.Code, w.Body)
		}
		disposition, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		ext := ".yaml"
		if format == "surge" {
			ext = ".conf"
		}
		if err != nil || disposition != "attachment" || params["filename"] != "日常订阅 Home"+ext {
			t.Fatal("subscription download name", w.Header().Get("Content-Disposition"), err)
		}
		text := w.Body.String()
		if !strings.Contains(text, "DOMAIN,example.com,Work") || !strings.Contains(text, "ChinaDomain.list") {
			t.Fatal("routing lost at HTTP boundary")
		}
		if strings.Contains(text, token) || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("capability boundary")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path+"&format=hysteria2_uri", nil))
	if w.Code != 422 || !strings.Contains(w.Body.String(), "client_incompatible") {
		t.Fatal("URI silently dropped routing", w.Code, w.Body)
	}
}
