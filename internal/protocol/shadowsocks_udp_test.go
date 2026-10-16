package protocol

import (
	"encoding/json"
	"testing"
)

func TestShadowsocksUDPOptIn(t *testing.T) {
	for _, kind := range []string{"shadowsocks", "shadowsocks2022"} {
		for _, enabled := range []bool{false, true} {
			for _, relay := range []bool{false, true} {
				s, e := NewSpec(Input{Name: "SS", Protocol: kind, Port: 24443, UDPEnabled: enabled})
				if e != nil {
					t.Fatal(e)
				}
				if enabled && s.RequiredAgentVersion() != "0.18.2" {
					t.Fatal("missing gate")
				}
				if relay {
					s.Relay = &Peer{Address: "203.0.113.1", Protocol: kind, Port: 24444, Credential: s.Credential}
				}
				b, e := Render(s)
				if e != nil {
					t.Fatal(e)
				}
				var cfg struct {
					Inbounds []map[string]any
					Route    struct{ Rules []map[string]any }
				}
				if e = json.Unmarshal(b, &cfg); e != nil {
					t.Fatal(e)
				}
				if (cfg.Inbounds[0]["network"] == "tcp") != (!enabled || relay) {
					t.Fatal("incorrect UDP listener policy")
				}
				private := false
				for _, r := range cfg.Route.Rules {
					if r["ip_is_private"] == true && r["action"] == "reject" {
						private = true
					}
				}
				if !private {
					t.Fatal("missing private guard")
				}
			}
		}
	}
}
