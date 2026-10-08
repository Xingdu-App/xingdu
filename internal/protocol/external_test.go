package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExternalExitRenderingAndNetworkBoundary(t *testing.T) {
	for _, kind := range []string{"socks", "http"} {
		s, err := NewSpec(Input{Name: "entry", Protocol: "shadowsocks", Port: 24443})
		if err != nil {
			t.Fatal(err)
		}
		s.Relay = &Peer{ExternalID: "ext_" + strings.Repeat("a", 32), Address: "93.184.216.34", Port: 1080, Protocol: kind, Username: "provider-user", Credential: "short"}
		raw, err := Render(s)
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err = json.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		outs := config["outbounds"].([]any)
		if len(outs) != 1 {
			t.Fatal("unexpected direct fallback")
		}
		out := outs[0].(map[string]any)
		if out["type"] != kind || out["username"] != "provider-user" || out["password"] != "short" || out["tls"] != nil {
			t.Fatal("external credentials or transport changed", out)
		}
		if kind == "socks" && (out["version"] != "5" || out["network"] != "tcp") {
			t.Fatal("SOCKS5 options missing")
		}
		if s.RequiredAgentVersion() != ExternalProxyAgentVersion {
			t.Fatal("new credentials accepted by older agent")
		}
		rules := config["route"].(map[string]any)["rules"].([]any)
		if rules[0].(map[string]any)["network"] != "udp" {
			t.Fatal("UDP not rejected")
		}
		for _, target := range []string{"10.0.0.1", "100.64.1.1", "100.100.100.200", "168.63.129.16", "127.0.0.1", "169.254.169.254", "192.0.2.1", "198.18.1.1", "::1", "fc00::1", "2001:db8::1", "::ffff:93.184.216.34", "64:ff9b::5db8:d822", "fe80::1%en0", "proxy.example.com"} {
			s.Relay.Address = target
			if _, err = Render(s); err == nil {
				t.Errorf("unsafe endpoint accepted: %s", target)
			}
		}
		s.Relay.Address = "2606:4700:4700::1111"
		if _, err = Render(s); err != nil {
			t.Fatal("public IPv6 rejected", err)
		}
		s.Relay.NodeID = "node_" + strings.Repeat("b", 32)
		if s.Relay.Validate() == nil {
			t.Fatal("mixed exit identities accepted")
		}
	}
	for _, credential := range []string{"", strings.Repeat("x", 256), "user\r\nheader", "nul\x00", "\xff"} {
		if ValidExternalCredential(credential) {
			t.Fatal("invalid credential accepted")
		}
	}
}
