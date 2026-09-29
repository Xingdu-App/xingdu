package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/agent"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

func TestMachineLifecycle(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, e := storage.Open(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	if e = admin.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	uri, _ := url.Parse(raw)
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	s, e := storage.Open(ctx, uri.String())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	uid, e := s.Register(ctx, "machine_"+storage.NewID("obj")[4:20], "test-hash", "machine test")
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", uid)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
	}()
	orgs, _ := s.Organizations(ctx, uid)
	scoped := storage.WithTenant(ctx, uid, orgs[0].ID)
	host, e := s.CreateHost(scoped, hosts.Input{Name: "machine", Address: "machine.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	if host.AgentVersion != nil {
		t.Fatal("new host exposes an Agent version")
	}
	token := machine.Token()
	if _, e = s.Enrollment(scoped, host.ID, "monitor", machine.Hash(token)); e != nil {
		t.Fatal(e)
	}
	v, _ := vault.New(strings.Repeat("ab", 32))
	h := New(s, Options{PublicOrigin: "http://127.0.0.1:15173", AgentOrigin: "https://control.example.invalid", CredentialVault: v})
	server := httptest.NewServer(h)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "agent.json")
	if e = agent.Initialize(ctx, path, server.URL, "monitor", token); e != nil {
		t.Fatal("agent enrollment:", e)
	}
	config, e := agent.Load(path)
	if e != nil || config.EnrollmentToken != "" {
		t.Fatal("bootstrap token persisted", e)
	}
	state, e := s.MachineState(scoped, host.ID)
	if e != nil || state.RequiredAgentVersion != machine.MinimumDeploymentVersion || state.Agent == nil || state.Agent.Metrics.Hostname == "" {
		t.Fatal("heartbeat metrics missing", e)
	}
	records, _ := s.Hosts(scoped)
	if len(records) != 1 || records[0].Status != "online" || records[0].AgentVersion == nil || *records[0].AgentVersion != machine.Version {
		t.Fatal("no live heartbeat")
	}
	if e = s.EnrollAgent(ctx, machine.Hash(token), machine.Hash(machine.Token()), "monitor"); !errors.Is(e, storage.ErrNotFound) {
		t.Fatal("enrollment replay accepted", e)
	}
	// Browser credentials cannot invoke the machine-only API.
	r := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", strings.NewReader(`{}`))
	r.Header.Set("Origin", "http://127.0.0.1:15173")
	r.Header.Set("Authorization", "Bearer "+config.Token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("browser bypass", w.Code)
	}
	// A stolen token has no tenant session and cannot use management endpoints.
	r = httptest.NewRequest("GET", "/api/v1/hosts", nil)
	r.Header.Set("Authorization", "Bearer "+config.Token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("agent token used as human identity")
	}
	admin.Pool.Exec(ctx, "UPDATE hosts SET last_seen_at=now()-interval '91 seconds' WHERE id=$1", host.ID)
	records, _ = s.Hosts(scoped)
	if records[0].Status != "offline" || records[0].AgentVersion == nil || *records[0].AgentVersion != machine.Version {
		t.Fatal("stale heartbeat online")
	}
	if e = agent.Sync(ctx, path, &config); e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeMachine(scoped, host.ID); e != nil {
		t.Fatal(e)
	}
	if e = agent.Sync(ctx, path, &config); !errors.Is(e, agent.ErrRevoked) {
		t.Fatal("revoked agent still active", e)
	}
	records, e = s.Hosts(scoped)
	if e != nil || len(records) != 1 || records[0].AgentVersion != nil {
		t.Fatal("revoked Agent version still visible", e)
	}
	// A pending enrollment cannot silently escalate its authorized mode.
	token = machine.Token()
	s.Enrollment(scoped, host.ID, "monitor", machine.Hash(token))
	if e = s.EnrollAgent(ctx, machine.Hash(token), machine.Hash(machine.Token()), "manage"); !errors.Is(e, storage.ErrForbidden) {
		t.Fatal("mode escalation", e)
	}
	// Queue data and retained credentials are encrypted and omitted from API-visible JSON.

	secret := machine.Secret{Target: machine.Target{Address: host.Address, Port: host.SSHPort, User: host.SSHUser}, Password: "test-private-password", Fingerprint: "SHA256:" + strings.Repeat("a", 43)}
	webToken := machine.Token()
	if e = s.NewSession(ctx, machine.Hash(webToken), uid, time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	installBody := map[string]any{"method": "password", "password": secret.Password, "private_key": "", "passphrase": "", "fingerprint": secret.Fingerprint, "mode": "monitor", "retain": true, "use_saved": false, "confirm_manage": false, "confirm_fingerprint": true}
	submit := func(want int) {
		t.Helper()
		body, _ := json.Marshal(installBody)
		req := httptest.NewRequest("POST", "/api/v1/hosts/"+host.ID+"/ssh/install", bytes.NewReader(body))
		req.Header.Set("Origin", "http://127.0.0.1:15173")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Xingdu-Request", "1")
		req.Header.Set("X-Xingdu-Organization", orgs[0].ID)
		req.Header.Set("X-CSRF-Token", csrfToken(webToken))
		req.AddCookie(&http.Cookie{Name: cookieName, Value: webToken})
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("queue got %d want %d: %s", res.Code, want, res.Body.String())
		}
	}
	submit(202)
	submit(409)
	state, e = s.MachineState(scoped, host.ID)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(state)
	if strings.Contains(string(encoded), secret.Password) || strings.Contains(string(encoded), "encrypted") || state.Credential == nil {
		t.Fatal("credential disclosure")
	}
	if _, e = s.ClaimMachineJob(ctx); e == nil {
		t.Fatal("API can claim global queue")
	}
	q.Set("options", "-crole=xingdu_worker")
	uri.RawQuery = q.Encode()
	worker, e := storage.Open(ctx, uri.String())
	if e != nil {
		t.Fatal(e)
	}
	defer worker.Close()
	claim, e := worker.ClaimMachineJob(ctx)
	if e != nil {
		t.Fatal(e)
	}
	claim, e = worker.LoadMachineJob(storage.WithTenant(ctx, claim.UserID, claim.OrgID), claim)
	if e != nil || claim.HostID != host.ID {
		t.Fatal("worker tenant scope", e)
	}
	if e = worker.FinishMachineJob(scoped, claim, "failed", "test_complete"); e != nil {
		t.Fatal(e)
	}
	var erased bool
	admin.Pool.QueryRow(ctx, "SELECT encrypted IS NULL FROM machine_jobs WHERE id=$1", claim.ID).Scan(&erased)
	if !erased {
		t.Fatal("temporary credential retained")
	}

	installBody["password"] = ""
	installBody["use_saved"] = true
	installBody["retain"] = false
	submit(202)
	if e = s.RevokeMachine(scoped, host.ID); e != nil {
		t.Fatal(e)
	}
	saved, e := s.SavedCredential(scoped, host.ID)
	if e != nil {
		t.Fatal(e)
	}
	plain, e := v.Open(saved.Encrypted, machine.AAD(orgs[0].ID, host.ID, "saved"))
	if e != nil || !bytes.Contains(plain, []byte(secret.Password)) {
		t.Fatal("retained credential unavailable", e)
	}
	clear(plain)
	host.Address = "changed.example.invalid"
	if _, e = s.UpdateHost(scoped, host.ID, host.Input); e != nil {
		t.Fatal(e)
	}
	submit(409)
	if e = s.DeleteCredential(scoped, host.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SavedCredential(scoped, host.ID); !errors.Is(e, storage.ErrNotFound) {
		t.Fatal("saved credential not deleted", e)
	}
}
