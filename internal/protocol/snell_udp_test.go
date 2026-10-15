package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnellV6UDPOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		in := Input{Name: "Snell", Protocol: "snell6", Port: 24443, UDPEnabled: enabled}
		s, err := NewSpec(in)
		if err != nil {
			t.Fatal(err)
		}
		if enabled && in.RequiredAgentVersion() != "0.18.1" {
			t.Fatal("missing version gate")
		}
		for _, relay := range []bool{false, true} {
			if relay {
				s.Relay = &Peer{Address: "203.0.113.1", Protocol: "snell6", Port: 24444, Credential: s.Credential}
			}
			b, err := Render(s)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Route struct{ Rules []map[string]any }
			}
			if err = json.Unmarshal(b, &cfg); err != nil {
				t.Fatal(err)
			}
			blocked, private := false, false
			for _, r := range cfg.Route.Rules {
				if r["network"] == "udp" && r["action"] == "reject" {
					blocked = true
				}
				if r["ip_is_private"] == true && r["action"] == "reject" {
					private = true
				}
			}
			if blocked != (!enabled || relay) {
				t.Fatalf("wrong UDP policy enabled=%v relay=%v", enabled, relay)
			}
			if !private {
				t.Fatal("private destination guard lost")
			}
		}
	}
	s, _ := NewSpec(Input{Name: "External", Protocol: "snell6", Port: 24443, UDPEnabled: true})
	s.Relay = &Peer{ExternalID: "ext_" + strings.Repeat("a", 32)}
	if s.RequiredAgentVersion() != "0.18.1" {
		t.Fatal("external exit weakened UDP version gate")
	}
	if ValidateInput(Input{Name: "Legacy", Protocol: "snell", Port: 24443, UDPEnabled: true}) == nil {
		t.Fatal("legacy Snell must remain TCP only")
	}
}
