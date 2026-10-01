package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type matrixDeploymentStore struct {
	keyBoundaryStore
	deployment storage.Deployment
}

func (s *matrixDeploymentStore) QueueDeployment(_ context.Context, d storage.Deployment) error {
	s.deployment = d
	return nil
}
func (s *matrixDeploymentStore) DeploymentSecret(context.Context, string, string) (storage.Deployment, error) {
	return s.deployment, nil
}
func (s *matrixDeploymentStore) UpdateDeployment(_ context.Context, d storage.Deployment, _ []byte) error {
	s.deployment = d
	return nil
}

func TestCompatibility72APIEncryptedProjection(t *testing.T) {
	raw, err := os.ReadFile("../protocol/testdata/compatibility72.json")
	if err != nil {
		t.Fatal(err)
	}
	var inputs []protocol.Input
	if err = json.Unmarshal(raw, &inputs); err != nil {
		t.Fatal(err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"proxy.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	kb, _ := x509.MarshalPKCS8PrivateKey(key)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}))
	v, _ := vault.New(strings.Repeat("a", 64))
	for _, in := range inputs {
		t.Run(in.Name, func(t *testing.T) {
			org, host := storage.NewID("org"), storage.NewID("srv")
			store := &matrixDeploymentStore{keyBoundaryStore: keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), CreatedBy: storage.NewID("usr"), OrganizationID: org, Scopes: []string{"nodes:write", "nodes:credentials"}}}}
			h := New(store, Options{CredentialVault: v})
			call := func(method, path string, body any, want int) *httptest.ResponseRecorder {
				t.Helper()
				b, _ := json.Marshal(body)
				req := httptest.NewRequest(method, path, bytes.NewReader(b))
				req.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				if w.Code != want {
					t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
				}
				return w
			}
			if in.NeedsCertificate() {
				in.ServerName, in.Certificate, in.PrivateKey = "proxy.example.com", certPEM, keyPEM
			}
			base := "/api/v1/hosts/" + host + "/deployments"
			call("POST", base, struct {
				protocol.Input
				Confirm bool `json:"confirm_install"`
			}{in, true}, 202)
			decode := func() protocol.Spec {
				t.Helper()
				p, e := v.Open(store.deployment.Encrypted, deploymentAAD(org, host, store.deployment.ID))
				if e != nil {
					t.Fatal(e)
				}
				var spec protocol.Spec
				if json.Unmarshal(p, &spec) != nil {
					t.Fatal("bad encrypted spec")
				}
				return spec
			}
			spec := decode()
			if err := protocol.ValidateSpec(spec); err != nil {
				t.Fatal(err)
			}
			path := base + "/" + store.deployment.ID
			w := call("POST", path+"/connection", map[string]any{}, 200)
			if strings.Contains(w.Body.String(), "PRIVATE KEY") {
				t.Fatal("TLS private key exposed")
			}
			for _, secret := range []string{spec.PrivateKey, spec.EncryptionKey} {
				if secret != "" && strings.Contains(w.Body.String(), secret) {
					t.Fatal("server key exposed")
				}
			}
			if spec.WireGuardKeys != nil {
				if strings.Contains(w.Body.String(), spec.WireGuardKeys.ServerPrivate) || !strings.Contains(w.Body.String(), spec.WireGuardKeys.ClientPrivate) {
					t.Fatal("wrong WireGuard key projection")
				}
				for _, enabled := range []bool{false, true} {
					opts := *spec.WireGuard
					opts.Preshared = enabled
					call("PUT", path, map[string]any{"confirm": true, "wireguard": opts}, 202)
					next := decode()
					if (next.WireGuardKeys.Preshared != "") != enabled || next.WireGuardKeys.ServerPrivate != spec.WireGuardKeys.ServerPrivate {
						t.Fatal("PSK toggle changed identity or was lost")
					}
				}
			}
			if spec.V2Ray != nil && spec.V2Ray.Encryption && !strings.Contains(w.Body.String(), spec.ClientEncryption()) {
				t.Fatal("client encryption missing")
			}
		})
	}
}
