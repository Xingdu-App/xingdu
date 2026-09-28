package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
)

func TestConfigAndTransportBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	c := Config{Server: "http://127.0.0.1:18080", Token: machine.Token(), Mode: "monitor"}
	if e := save(path, c, true); e != nil {
		t.Fatal(e)
	}
	if _, e := Load(path); e != nil {
		t.Fatal(e)
	}
	if e := save(path, c, true); e == nil {
		t.Fatal("configuration overwritten")
	}
	os.Chmod(path, 0644)
	if _, e := Load(path); e == nil {
		t.Fatal("world-readable secret accepted")
	}
	os.Chmod(path, 0600)
	linked := filepath.Join(t.TempDir(), "link")
	os.Symlink(path, linked)
	if _, e := Load(linked); e == nil {
		t.Fatal("symlink config accepted")
	}
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("secret followed redirect") }))
	defer sink.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c.Server = redirect.URL
	if e := request(context.Background(), c, "/api/v1/agent/heartbeat", Collect(), true); e == nil {
		t.Fatal("redirect accepted")
	}
	if machine.Origin("http://public.example.com") == nil {
		t.Fatal("plaintext remote origin accepted")
	}
	unit := ServiceUnit("xingdu-agent", "monitor")
	for _, expected := range []string{"User=xingdu-agent", "NoNewPrivileges=yes", "ProtectSystem=strict", "CapabilityBoundingSet=", "Restart=on-failure"} {
		if !strings.Contains(unit, expected) {
			t.Fatal("missing sandbox", expected)
		}
	}
}
