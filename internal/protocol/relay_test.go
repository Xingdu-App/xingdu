package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRelayUsesOnlyExplicitExitAndRejectsUDP(t *testing.T) {
	s, e := NewSpec(Input{Name: "entry", Protocol: "shadowsocks", Port: 12345})
	if e != nil {
		t.Fatal(e)
	}
	s.Relay = &Peer{NodeID: "node_test", Address: "93.184.216.34", Protocol: "shadowsocks", Port: 443, Credential: s.Credential}
	b, e := Render(s)
	if e != nil {
		t.Fatal(e)
	}
	var c map[string]any
	json.Unmarshal(b, &c)
	out := c["outbounds"].([]any)
	if len(out) != 1 || out[0].(map[string]any)["tag"] != "exit" {
		t.Fatal("relay can fall back to direct")
	}
	rules := c["route"].(map[string]any)["rules"].([]any)
	if rules[0].(map[string]any)["network"] != "udp" {
		t.Fatal("relay UDP permitted")
	}
	if strings.Contains(string(b), "private_key") {
		t.Fatal("private key in peer")
	}
	for _, address := range []string{"127.0.0.1", "169.254.169.254", "::1", "malicious.example"} {
		s.Relay.Address = address
		if _, e = Render(s); e == nil {
			t.Fatal("unsafe relay endpoint", address)
		}
	}
}
