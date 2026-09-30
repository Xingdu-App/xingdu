package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type certificateStoreFake struct{ keyBoundaryStore }

func (certificateStoreFake) Certificates(context.Context) ([]storage.ManagedCertificate, int, error) {
	return []storage.ManagedCertificate{{ID: storage.NewID("cert"), Domain: "node.example.com", State: "issued", Encrypted: []byte("private-certificate-fixture")}}, 10, nil
}
func (certificateStoreFake) CreateCertificate(context.Context, string, string) (storage.ManagedCertificate, error) {
	return storage.ManagedCertificate{}, storage.ErrCertificatePaid
}
func (certificateStoreFake) CreatePlatformCertificate(context.Context, string, string, string) (storage.ManagedCertificate, error) {
	return storage.ManagedCertificate{}, storage.ErrCertificatePaid
}
func (certificateStoreFake) QueueCertificate(context.Context, string, string) error {
	return storage.ErrCertificatePaid
}
func (certificateStoreFake) DeleteCertificate(context.Context, string) error {
	return storage.ErrNotFound
}
func (certificateStoreFake) CertificateSecret(context.Context, string) (storage.ManagedCertificate, error) {
	return storage.ManagedCertificate{}, storage.ErrCertificatePaid
}
func TestCertificateAPIScopesAndSecretRedaction(t *testing.T) {
	cfg := certificates.ManagedConfig{ValidationDomain: "validation.example.com", Email: "operator@example.com", StateDir: t.TempDir(), Token: "fixture", Zone: strings.Repeat("a", 32), Directory: certificates.Staging, AcceptTerms: true}
	v, _ := vault.New(strings.Repeat("a", 64))
	s := certificateStoreFake{keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), OrganizationID: storage.NewID("org"), CreatedBy: storage.NewID("usr"), Scopes: []string{"certificates:read"}}}}
	call := func(method, path, body string, store certificateStoreFake, config certificates.ManagedConfig) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		New(store, Options{Certificates: config, CredentialVault: v}).ServeHTTP(w, r)
		return w
	}
	w := call("GET", "/api/v1/certificates", "", s, cfg)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-certificate-fixture") || strings.Contains(w.Body.String(), "encrypted") {
		t.Fatal("metadata leak or response failure", w.Code, w.Body.String())
	}
	if w = call("POST", "/api/v1/certificates", `{"domain":"node.example.com"}`, s, cfg); w.Code != 403 || !strings.Contains(w.Body.String(), "insufficient_scope") {
		t.Fatal("read key can create", w.Code, w.Body.String())
	}
	s.key.Scopes = []string{"certificates:write"}
	if w = call("POST", "/api/v1/certificates", `{"domain":"node.example.com"}`, s, cfg); w.Code != 403 || !strings.Contains(w.Body.String(), "paid_subscription_required") {
		t.Fatal("paid gate", w.Code, w.Body.String())
	}
	if w = call("POST", "/api/v1/certificates", `{"domain":"node.example.com"}`, s, certificates.ManagedConfig{}); w.Code != 503 {
		t.Fatal("unconfigured provider enabled", w.Code)
	}
	id := storage.NewID("cert")
	if apiScope("POST", "/api/v1/certificates/"+id+"/apply") != "" {
		t.Fatal("API key unexpectedly permits node application")
	}
	if apiScope("POST", "/api/v1/certificates/"+id+"/issue") != "certificates:write" {
		t.Fatal("issue scope missing")
	}
}
