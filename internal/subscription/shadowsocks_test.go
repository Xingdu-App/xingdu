package subscription

import (
	"fmt"
	"go.yaml.in/yaml/v3"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestShadowsocksExports(t *testing.T) {
	var nodes []Node
	for i, kind := range []string{"shadowsocks", "shadowsocks2022"} {
		s, err := protocol.NewSpec(protocol.Input{Name: "IP only", Protocol: kind, Port: 8388 + i})
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, Node{ID: fmt.Sprintf("node_%032x", i), Name: "IP only", Server: "192.0.2.10", Spec: s})
	}
	for _, format := range []string{"stash", "mihomo"} {
		b, err := Render(format, "IP only", nodes, nil, "proxy")
		if err != nil {
			t.Fatal(format, err)
		}
		var cfg struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		if err = yaml.Unmarshal(b, &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Proxies) != 2 {
			t.Fatal("lost nodes")
		}
		for i, p := range cfg.Proxies {
			if p["type"] != "ss" || p["cipher"] != protocol.Cipher(nodes[i].Spec.Protocol) || p["password"] != nodes[i].Spec.Credential || p["udp"] != false || p["server"] != nodes[i].Server {
				t.Fatal("incorrect SS fields", format)
			}
			for _, field := range []string{"tls", "sni", "servername", "certificate", "fingerprint", "server-cert-fingerprint", "skip-cert-verify"} {
				if _, ok := p[field]; ok {
					t.Fatal("unexpected TLS field", format, field)
				}
			}
		}
	}
	b, err := Render("surge", "IP only", nodes, nil, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if !strings.Contains(string(b), fmt.Sprintf("ss, %s, %d, encrypt-method=%s, password=%s, udp-relay=false", n.Server, n.Spec.Port, protocol.Cipher(n.Spec.Protocol), n.Spec.Credential)) {
			t.Fatal("invalid Surge mapping")
		}
	}
	if strings.Contains(string(b), "tls=") || strings.Contains(string(b), "fingerprint=") {
		t.Fatal("TLS applied to SS")
	}
	for _, format := range []string{"loon", "hysteria2_uri"} {
		if b, err := Render(format, "Unsupported", nodes, nil, "proxy"); err == nil || len(b) > 0 {
			t.Fatal("unsupported format silently accepted", format)
		}
	}
	// Mixed subscriptions must preserve existing TLS policy alongside IP-only nodes.
	nodes = append(nodes, fixture(t, "trojan", 2))
	if _, err = Render("stash", "Mixed", nodes, nil, "proxy"); err != nil {
		t.Fatal(err)
	}
}
