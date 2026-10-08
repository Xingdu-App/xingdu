package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type externalStoreFake struct {
	keyBoundaryStore
	profile    storage.ExternalProxy
	previous   []byte
	deployment storage.Deployment
	updated    storage.Deployment
}

func (s *externalStoreFake) ExternalProxies(context.Context) ([]storage.ExternalProxy, error) {
	return []storage.ExternalProxy{s.profile}, nil
}
func (s *externalStoreFake) ExternalProxySecret(_ context.Context, id string) (storage.ExternalProxy, error) {
	if id != s.profile.ID {
		return storage.ExternalProxy{}, storage.ErrNotFound
	}
	return s.profile, nil
}
func (s *externalStoreFake) SaveExternalProxy(_ context.Context, p storage.ExternalProxy, previous []byte, _ bool) (storage.ExternalProxy, error) {
	s.profile = p
	s.previous = previous
	return p, nil
}
func (s *externalStoreFake) DeleteExternalProxy(context.Context, string) error {
	return storage.ErrConflict
}
func (s *externalStoreFake) DeploymentSecret(context.Context, string, string) (storage.Deployment, error) {
	return s.deployment, nil
}
func (s *externalStoreFake) UpdateDeployment(_ context.Context, d storage.Deployment, _ []byte) error {
	s.updated = d
	return nil
}
func TestExternalProxyAPICredentialsAndScopes(t *testing.T) {
	v, _ := vault.New(strings.Repeat("a", 64))
	s := &externalStoreFake{keyBoundaryStore: keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), OrganizationID: storage.NewID("org"), CreatedBy: storage.NewID("usr"), Scopes: []string{"exits:read"}}}}
	s.profile = storage.ExternalProxy{ID: storage.NewID("ext"), Encrypted: []byte("secret-ciphertext"), Name: "Exit"}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		New(s, Options{CredentialVault: v}).ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/api/v1/external-proxies", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-ciphertext") || strings.Contains(w.Body.String(), "encrypted") {
		t.Fatal("inventory leaks secrets", w.Code, w.Body.String())
	}
	body := `{"name":"ISP exit","address":"93.184.216.34","port":1080,"protocol":"socks","username":"fixture-user","password":"fixture-password"}`
	if w = call("POST", "/api/v1/external-proxies", body); w.Code != 403 {
		t.Fatal("read key creates exit", w.Code)
	}
	s.key.Scopes = []string{"exits:write"}
	w = call("POST", "/api/v1/external-proxies", body)
	if w.Code != 201 || strings.Contains(w.Body.String(), "fixture-user") || strings.Contains(w.Body.String(), "fixture-password") {
		t.Fatal("creation or redaction failure", w.Code, w.Body.String())
	}
	plain, err := v.Open(s.profile.Encrypted, externalAAD(s.key.OrganizationID, s.profile.ID))
	if err != nil || !strings.Contains(string(plain), "fixture-password") {
		t.Fatal("credentials not encrypted with correct binding", err)
	}
	clear(plain)
	if _, err = v.Open(s.profile.Encrypted, externalAAD(storage.NewID("org"), s.profile.ID)); err == nil {
		t.Fatal("cross tenant AAD accepted")
	}
	if _, err = v.Open(s.profile.Encrypted, externalAAD(s.key.OrganizationID, storage.NewID("ext"))); err == nil {
		t.Fatal("cross profile AAD accepted")
	}
	cipher := s.profile.Encrypted
	path := "/api/v1/external-proxies/" + s.profile.ID
	if w = call("PUT", path, `{"name":"Edited","address":"93.184.216.34","port":1080,"protocol":"http"}`); w.Code != 200 || len(s.previous) != len(cipher) || s.profile.Encrypted != nil {
		t.Fatal("omitted credentials not preserved by storage contract", w.Code)
	}
	for _, bad := range []string{`{"name":"X","address":"127.0.0.1","port":1080,"protocol":"socks","username":"u","password":"p"}`, `{"name":"X","address":"93.184.216.34","port":1080,"protocol":"socks","username":"u"}`, `{"name":"X","address":"proxy.example.com","port":1080,"protocol":"http","username":"u","password":"p"}`} {
		if w = call("POST", "/api/v1/external-proxies", bad); w.Code != 422 {
			t.Fatal("invalid input accepted", w.Code)
		}
	}
	if w = call("DELETE", path, `{}`); w.Code != 409 {
		t.Fatal("dependency conflict lost", w.Code)
	}
	if apiScope("GET", path) != "" || apiScope("GET", path+"/credentials") != "" {
		t.Fatal("secret route accidentally scoped")
	}
}
func TestExternalRouteResolutionAndClearing(t *testing.T) {
	v, _ := vault.New(strings.Repeat("b", 64))
	s := &externalStoreFake{keyBoundaryStore: keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), OrganizationID: storage.NewID("org"), CreatedBy: storage.NewID("usr"), Scopes: []string{"nodes:write"}}}}
	s.profile = storage.ExternalProxy{ID: storage.NewID("ext"), Name: "Exit", Address: "93.184.216.34", Port: 1080, Protocol: "socks"}
	plain, _ := json.Marshal(externalCredentials{Username: "fixture-user", Password: "fixture-password"})
	s.profile.Encrypted = v.Seal(plain, externalAAD(s.key.OrganizationID, s.profile.ID))
	clear(plain)
	// Use a real spec so validation covers runtime restrictions and snapshots.
	spec, err := protocol.NewSpec(protocol.Input{Name: "entry", Protocol: "shadowsocks", Port: 24443})
	if err != nil {
		t.Fatal(err)
	}
	s.deployment = storage.Deployment{ID: storage.NewID("node"), HostID: storage.NewID("srv"), Protocol: spec.Protocol}
	seal := func() {
		plain, _ := json.Marshal(spec)
		s.deployment.Encrypted = v.Seal(plain, deploymentAAD(s.key.OrganizationID, s.deployment.HostID, s.deployment.ID))
		clear(plain)
	}
	seal()
	path := "/api/v1/hosts/" + s.deployment.HostID + "/deployments/" + s.deployment.ID
	call := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		New(s, Options{CredentialVault: v}).ServeHTTP(w, r)
		return w
	}
	body := `{"external_exit_id":"` + s.profile.ID + `","confirm":true}`
	if w := call(body); w.Code != 403 {
		t.Fatal("nodes:write alone selects exit", w.Code, w.Body.String())
	}
	s.key.Scopes = append(s.key.Scopes, "exits:write")
	if w := call(body); w.Code != 202 || strings.Contains(w.Body.String(), "fixture-password") {
		t.Fatal("external update failed or leaked", w.Code, w.Body.String())
	}
	if s.updated.ExternalExitID != s.profile.ID || s.updated.RelayExitID != "" || s.updated.MinimumAgentVersion != protocol.ExternalProxyAgentVersion {
		t.Fatal("external route metadata incorrect")
	}
	decoded, err := v.Open(s.updated.Encrypted, deploymentAAD(s.key.OrganizationID, s.deployment.HostID, s.deployment.ID))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(decoded, &spec); err != nil {
		t.Fatal(err)
	}
	clear(decoded)
	if spec.Relay == nil || spec.Relay.Username != "fixture-user" {
		t.Fatal("credential snapshot missing")
	}
	s.deployment = s.updated
	seal()
	if w := call(`{"exit_node_id":"","external_exit_id":"","confirm":true}`); w.Code != 202 {
		t.Fatal("cannot clear external route", w.Code, w.Body.String())
	}
	if s.updated.ExternalExitID != "" || s.updated.RelayExitID != "" || s.updated.ExternalCipher != nil {
		t.Fatal("stale route metadata retained")
	}
}
