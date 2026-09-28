package protocol

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func inputFixture(t *testing.T) Input {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "proxy.example.com"}, DNSNames: []string{"proxy.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	kb, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return Input{Name: "Example", Protocol: "trojan", Port: 443, ServerName: "proxy.example.com", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}))}
}
func TestRenderProtocols(t *testing.T) {
	for _, p := range []string{"trojan", "vless", "vmess", "hysteria2", "tuic"} {
		t.Run(p, func(t *testing.T) {
			in := inputFixture(t)
			in.Protocol = p
			s, e := NewSpec(in)
			if e != nil {
				t.Fatal(e)
			}
			b, e := Render(s)
			if e != nil {
				t.Fatal(e)
			}
			var cfg map[string]any
			if json.Unmarshal(b, &cfg) != nil {
				t.Fatal("invalid JSON")
			}
			if !strings.Contains(string(b), `"enabled": true`) || !strings.Contains(string(b), `"action": "resolve"`) {
				t.Fatal("missing TLS or DNS protection")
			}
			if p == "tuic" && (s.Password == "" || s.Password == s.Credential) {
				t.Fatal("TUIC needs distinct UUID and password")
			}
			other, e := NewSpec(in)
			if e != nil || other.Credential == s.Credential {
				t.Fatal("credentials not random")
			}
		})
	}
}
func TestRejectUnsafeInput(t *testing.T) {
	base := inputFixture(t)
	for _, change := range []func(*Input){func(i *Input) { i.Protocol = "quic" }, func(i *Input) { i.ServerName = "example.com\nExecStart=/bin/sh" }, func(i *Input) { i.ServerName = "other.example.com" }, func(i *Input) { i.Port = 65536 }, func(i *Input) { i.Name = "bad\nname" }, func(i *Input) { i.Certificate = strings.Repeat("x", 32769) }, func(i *Input) { i.PrivateKey = inputFixture(t).PrivateKey }} {
		in := base
		change(&in)
		if ValidateInput(in) == nil {
			t.Fatal("unsafe input accepted")
		}
	}
	s, e := NewSpec(base)
	if e != nil {
		t.Fatal(e)
	}
	s.Credential = "password\n"
	if _, e = Render(s); e == nil {
		t.Fatal("invalid credential accepted")
	}
}
func TestTaskIdentifiers(t *testing.T) {
	for _, s := range []string{"../../service", "", "123", "00000000-0000-4000-8000-00000000000g"} {
		if ValidID(s) {
			t.Fatal("bad id")
		}
	}
	if !ValidID(randomUUID()) {
		t.Fatal("generated invalid id")
	}
}
