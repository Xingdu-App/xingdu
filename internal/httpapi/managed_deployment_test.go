package httpapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type managedDeploymentFake struct {
	certificateStoreFake
	cert   storage.ManagedCertificate
	queued *storage.Deployment
	err    error
}

func (s *managedDeploymentFake) CertificateSecret(context.Context, string) (storage.ManagedCertificate, error) {
	return s.cert, s.err
}
func (s *managedDeploymentFake) QueueDeployment(_ context.Context, d storage.Deployment) error {
	*s.queued = d
	return nil
}

func TestManagedCertificateDeploymentBoundary(t *testing.T) {
	v, _ := vault.New(strings.Repeat("a", 64))
	org, host, certID := storage.NewID("org"), storage.NewID("srv"), storage.NewID("cert")
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"node.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	bundle := certificates.Bundle{Directory: certificates.Production, Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))}
	for _, tc := range []struct {
		name                                  string
		scopes                                []string
		directory, host, kind, manual, server string
		err                                   error
		want                                  int
	}{
		{"success", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "trojan", "", "", nil, 202},
		{"missing certificate permission", []string{"nodes:write"}, certificates.Production, host, "trojan", "", "", nil, 403},
		{"missing node permission", []string{"certificates:write"}, certificates.Production, host, "trojan", "", "", nil, 403},
		{"test CA", []string{"nodes:write", "certificates:write"}, certificates.Staging, host, "trojan", "", "", nil, 422},
		{"wrong host", []string{"nodes:write", "certificates:write"}, certificates.Production, storage.NewID("srv"), "trojan", "", "", nil, 422},
		{"non TLS", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "shadowsocks", "", "", nil, 422},
		{"mixed PEM", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "trojan", "manual", "", nil, 422},
		{"wrong domain", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "trojan", "", "other.example.com", nil, 422},
		{"unpaid", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "trojan", "", "", storage.ErrCertificatePaid, 403},
		{"foreign certificate", []string{"nodes:write", "certificates:write"}, certificates.Production, host, "trojan", "", "", storage.ErrNotFound, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := bundle
			b.Directory = tc.directory
			plain, _ := json.Marshal(b)
			var queued storage.Deployment
			s := &managedDeploymentFake{certificateStoreFake: certificateStoreFake{keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), OrganizationID: org, CreatedBy: storage.NewID("usr"), Scopes: tc.scopes}}}, cert: storage.ManagedCertificate{ID: certID, Domain: "node.example.com", Platform: true, HostID: tc.host, Encrypted: v.Seal(plain, storage.CertificateAAD(org, certID))}, queued: &queued, err: tc.err}
			body, _ := json.Marshal(map[string]any{"name": "Trojan fixture", "protocol": tc.kind, "port": 8443, "certificate_id": certID, "confirm_install": true, "certificate": tc.manual, "server_name": tc.server})
			r := httptest.NewRequest("POST", "/api/v1/hosts/"+host+"/deployments", strings.NewReader(string(body)))
			r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			New(s, Options{CredentialVault: v}).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "PRIVATE KEY") || strings.Contains(w.Body.String(), "private_key") {
				t.Fatal("response leaks key")
			}
			if tc.want != 202 {
				if queued.ID != "" {
					t.Fatal("invalid request queued")
				}
				return
			}
			plain, err := v.Open(queued.Encrypted, deploymentAAD(org, host, queued.ID))
			if err != nil {
				t.Fatal(err)
			}
			var spec protocol.Spec
			if json.Unmarshal(plain, &spec) != nil || spec.PrivateKey != bundle.PrivateKey || spec.ServerName != "node.example.com" || queued.CertificateID != certID {
				t.Fatal("managed material not injected")
			}
		})
	}
}
