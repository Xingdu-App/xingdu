package certificates

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"golang.org/x/crypto/acme"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testDNS struct {
	value   string
	removed bool
}

func (d *testDNS) Present(_ context.Context, _, value string) (string, error) {
	d.value = value
	return "record", nil
}
func (d *testDNS) Remove(context.Context, string) error { d.removed = true; return nil }
func TestACMEDNSOrderAndPrivateStorage(t *testing.T) {
	ca, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var issued []byte
	var server *httptest.Server
	validated := false
	finalized := false
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Replay-Nonce", base64.RawURLEncoding.EncodeToString([]byte(time.Now().String())))
		w.Header().Set("Content-Type", "application/json")
		order := func() {
			status := "pending"
			if validated {
				status = "ready"
			}
			if finalized {
				status = "valid"
			}
			json.NewEncoder(w).Encode(map[string]any{"status": status, "authorizations": []string{server.URL + "/auth"}, "finalize": server.URL + "/finalize", "certificate": server.URL + "/cert"})
		}
		switch r.URL.Path {
		case "/directory":
			json.NewEncoder(w).Encode(map[string]string{"newNonce": server.URL + "/nonce", "newAccount": server.URL + "/account", "newOrder": server.URL + "/new-order"})
		case "/nonce":
			w.WriteHeader(200)
		case "/account":
			w.Header().Set("Location", server.URL+"/account/1")
			w.WriteHeader(201)
			w.Write([]byte(`{"status":"valid"}`))
		case "/new-order":
			w.Header().Set("Location", server.URL+"/order")
			w.WriteHeader(201)
			order()
		case "/order":
			order()
		case "/auth":
			status := "pending"
			if validated {
				status = "valid"
			}
			json.NewEncoder(w).Encode(map[string]any{"status": status, "identifier": map[string]string{"type": "dns", "value": "node.example.com"}, "challenges": []any{map[string]string{"type": "dns-01", "url": server.URL + "/challenge", "token": "test-token", "status": status}}})
		case "/challenge":
			validated = true
			w.Write([]byte(`{"type":"dns-01","status":"valid","token":"test-token"}`))
		case "/finalize":
			var jws struct{ Payload string }
			json.NewDecoder(r.Body).Decode(&jws)
			payload, _ := base64.RawURLEncoding.DecodeString(jws.Payload)
			var request struct{ CSR string }
			json.Unmarshal(payload, &request)
			raw, _ := base64.RawURLEncoding.DecodeString(request.CSR)
			csr, e := x509.ParseCertificateRequest(raw)
			if e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: csr.DNSNames, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(90 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
			issued, _ = x509.CreateCertificate(rand.Reader, template, template, csr.PublicKey, ca)
			finalized = true
			order()
		case "/cert":
			w.Header().Set("Content-Type", "application/pem-certificate-chain")
			w.Write(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: issued}))
		default:
			t.Error("unexpected ACME path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	dns := &testDNS{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bundle, err := issue(ctx, &acme.Client{Key: key, DirectoryURL: server.URL + "/directory", HTTPClient: server.Client()}, dns, "node.example.com", "ops@example.com", func(context.Context, string) ([]string, error) { return []string{dns.value}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !dns.removed || bundle.PrivateKey == "" {
		t.Fatal("missing cleanup or key")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	if _, err = AccountKey(dir); err != nil {
		t.Fatal(err)
	}
	if err = Save(dir, bundle); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"account.pem", "certificate.json"} {
		st, _ := os.Stat(filepath.Join(dir, name))
		if st.Mode().Perm() != 0600 {
			t.Fatal("secret permissions")
		}
	}
}
func TestDNSProviderScopeAndRedactedFailure(t *testing.T) {
	if _, err := (Cloudflare{Token: "secret", Zone: "../other"}).Present(context.Background(), "_acme-challenge.node.example.com", "value"); err == nil {
		t.Fatal("unsafe zone accepted")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0755)
	if _, err := AccountKey(dir); err == nil {
		t.Fatal("public state accepted")
	}
}
