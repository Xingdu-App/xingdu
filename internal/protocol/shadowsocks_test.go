package protocol

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestShadowsocksWithoutTLS(t *testing.T) {
	for _, kind := range []string{"shadowsocks", "shadowsocks2022"} {
		t.Run(kind, func(t *testing.T) {
			in := Input{Name: "IP-only", Protocol: kind, Port: 8388}
			s, err := NewSpec(in)
			if err != nil {
				t.Fatal(err)
			}
			other, err := NewSpec(in)
			if err != nil || other.Credential == s.Credential {
				t.Fatal("credentials must be independently generated")
			}
			raw, err := Render(s)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct{ Inbounds []map[string]any }
			if err = json.Unmarshal(raw, &cfg); err != nil {
				t.Fatal(err)
			}
			inbound := cfg.Inbounds[0]
			if inbound["type"] != "shadowsocks" || inbound["network"] != "tcp" || inbound["method"] != Cipher(kind) || inbound["password"] != s.Credential || inbound["tls"] != nil || inbound["users"] != nil {
				t.Fatal("incorrect single-user Shadowsocks inbound")
			}
			if CertificateExpiry(s.Certificate) != nil || !strings.Contains(string(raw), `"action": "resolve"`) || !strings.Contains(string(raw), `"ip_is_private": true`) {
				t.Fatal("certificate metadata or private-destination protection incorrect")
			}
			for _, field := range []string{"server_name", "certificate", "private_key"} {
				bad := in
				switch field {
				case "server_name":
					bad.ServerName = "example.com"
				case "certificate":
					bad.Certificate = "PEM"
				case "private_key":
					bad.PrivateKey = "key"
				}
				if ValidateInput(bad) == nil {
					t.Fatalf("accepted unused %s", field)
				}
			}
			if kind == "shadowsocks2022" {
				key, err := base64.StdEncoding.DecodeString(s.Credential)
				if err != nil || len(key) != 32 {
					t.Fatal("AES-256 key must contain 32 random bytes")
				}
				for _, invalid := range []string{"password", base64.StdEncoding.EncodeToString(make([]byte, 16)), strings.TrimSuffix(s.Credential, "="), s.Credential + "\n"} {
					bad := s
					bad.Credential = invalid
					if ValidateSpec(bad) == nil {
						t.Fatal("invalid SS2022 key accepted")
					}
				}
			}
		})
	}
	// Omitting TLS must not weaken validation for any existing protocol.
	for _, kind := range []string{"trojan", "vless", "vmess", "hysteria2", "tuic"} {
		if ValidateInput(Input{Name: "No TLS", Protocol: kind, Port: 443}) == nil {
			t.Fatal("missing TLS accepted", kind)
		}
	}
}
