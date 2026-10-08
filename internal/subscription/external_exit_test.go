package subscription

import (
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestExternalExitCredentialsNeverReachClientExports(t *testing.T) {
	for _, format := range []string{"stash", "mihomo", "surge", "loon", "singbox"} {
		n := fixture(t, "shadowsocks", 1)
		n.Spec.Relay = &protocol.Peer{ExternalID: "ext_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: "93.184.216.34", Protocol: "socks", Port: 1080, Username: "provider-private-user", Credential: "provider-private-password"}
		raw, err := Render(format, "Entry subscription", []Node{n}, nil, "proxy")
		if format == "loon" {
			if err == nil || len(raw) != 0 {
				t.Fatal("unsupported client emitted configuration")
			}
			continue
		}
		if err != nil {
			t.Fatal(format, err)
		}
		for _, private := range []string{"provider-private-user", "provider-private-password", "external_id", "93.184.216.34"} {
			if strings.Contains(string(raw), private) {
				t.Errorf("%s contains private external config", format)
			}
		}
		if !strings.Contains(string(raw), n.Spec.Credential) {
			t.Fatalf("%s lost entry credentials", format)
		}
	}
}
