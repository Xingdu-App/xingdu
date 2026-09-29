package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
)

func TestSelfUpdateVerifiedDownloadAndAuthorization(t *testing.T) {
	for _, scenario := range []string{"ok", "digest", "redirect", "denied", "downgrade", "architecture"} {
		t.Run(scenario, func(t *testing.T) {
			binary := []byte("test updater")
			digest := sha256.Sum256(binary)
			task := machine.UpdateTask{ID: "job_" + strings.Repeat("a", 32), Lease: "lease_" + strings.Repeat("b", 32), Version: "99.0.0", Arch: runtime.GOARCH, SHA256: hex.EncodeToString(digest[:])}
			if scenario == "digest" {
				task.SHA256 = strings.Repeat("c", 64)
			}
			if scenario == "downgrade" {
				task.Version = "0.1.0"
			}
			if scenario == "architecture" {
				task.Arch = "invalid"
			}
			token := machine.Token()
			checked, applied, failed, executed := false, false, false, false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/v1/agent/update/") && r.Header.Get("Authorization") != "Bearer "+token {
					t.Error("missing machine auth")
				}
				switch r.URL.Path {
				case "/api/v1/agent/update/claim":
					json.NewEncoder(w).Encode(map[string]any{"data": task})
				case "/api/v1/agent/update/check":
					checked = true
					if scenario == "denied" {
						w.WriteHeader(403)
						return
					}
					w.Write([]byte("{}"))
				case "/api/v1/agent/update/applied":
					applied = true
					w.Write([]byte("{}"))
				case "/api/v1/agent/update/failed":
					failed = true
					w.Write([]byte("{}"))
				default:
					if scenario == "redirect" {
						http.Redirect(w, r, "/unexpected", 302)
						return
					}
					w.Write(binary)
				}
			}))
			defer srv.Close()
			dir := t.TempDir()
			c := Config{Server: srv.URL, Token: token, Mode: "monitor"}
			err := selfUpdate(context.Background(), c, func(_ context.Context, path string, e UpgradeExpectation) error {
				executed = true
				b, err := os.ReadFile(path)
				if err != nil || string(b) != string(binary) || !checked || e.TokenHash != machine.Hash(token) || e.Server != srv.URL || e.Mode != "monitor" || e.Version != task.Version {
					t.Fatal("unverified application")
				}
				return nil
			}, dir)
			if scenario == "ok" {
				if err != nil || !executed || !applied || failed {
					t.Fatal("valid update failed", err)
				}
			} else if err == nil || executed || applied || !failed {
				t.Fatal("unsafe update executed", err)
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 0 {
				t.Fatal("staging files retained")
			}
		})
	}
}
