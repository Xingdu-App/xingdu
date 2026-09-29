package subscription

import (
	"go.yaml.in/yaml/v3"
	"strings"
	"testing"
)

func TestTLSProxyExports(t *testing.T) {
	for _, kind := range []string{"anytls", "http"} {
		n := fixture(t, kind, 1)
		for _, format := range []string{"stash", "mihomo"} {
			raw, err := Render(format, "TLS proxies", []Node{n}, nil, "proxy")
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Proxies []map[string]any `yaml:"proxies"`
			}
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				t.Fatal(err)
			}
			p := cfg.Proxies[0]
			pin := "fingerprint"
			if format == "stash" {
				pin = "server-cert-fingerprint"
			}
			if p["type"] != kind || p["password"] != n.Spec.Credential || p["sni"] != n.Spec.ServerName || p["skip-cert-verify"] != false || p[pin] == nil || p["udp"] != false {
				t.Fatalf("incorrect %s %s fields", format, kind)
			}
			if kind == "http" && (p["username"] != "xingdu" || p["tls"] != true) {
				t.Fatal("HTTPS must be authenticated and encrypted")
			}
			if strings.Contains(string(raw), "PRIVATE KEY") {
				t.Fatal("private key exported")
			}
		}
		for _, format := range []string{"surge", "loon", "hysteria2_uri"} {
			if Supports(format, kind) {
				t.Fatal("unsupported adapter advertised")
			}
			if _, err := Render(format, "TLS proxies", []Node{n}, nil, "proxy"); err == nil {
				t.Fatal("unsupported adapter accepted")
			}
		}
	}
}
