package protocol

import (
	"encoding/json"
	"testing"
)

func TestTLSProxyConfiguration(t *testing.T) {
	for _, kind := range []string{"anytls", "http"} {
		in := inputFixture(t)
		in.Protocol = kind
		s, err := NewSpec(in)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Render(s)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Inbounds []map[string]any
			Route    struct{ Rules []map[string]any }
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		inbound := cfg.Inbounds[0]
		user := inbound["users"].([]any)[0].(map[string]any)
		if inbound["type"] != kind || user["password"] != s.Credential || inbound["tls"] == nil {
			t.Fatal("invalid TLS authentication")
		}
		if kind == "http" && (user["username"] != "xingdu" || user["name"] != nil) {
			t.Fatal("invalid HTTP username")
		}
		if kind == "anytls" && (cfg.Route.Rules[0]["network"] != "udp" || cfg.Route.Rules[0]["action"] != "reject") {
			t.Fatal("UDP must remain blocked")
		}
		if MinimumAgentVersion(kind) != "0.10.0-dev" {
			t.Fatal("old Agent must not receive new protocols")
		}
		s.Certificate = ""
		if ValidateSpec(s) == nil {
			t.Fatal("accepted unencrypted proxy")
		}
	}
}
