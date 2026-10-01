package protocol

import (
	"encoding/json"
	"testing"
)

func extraInput(t *testing.T, kind string) Input {
	in := Input{Name: "Test", Protocol: kind, Port: 24443}
	if RequiresTLS(kind) {
		in = inputFixture(t)
		in.Protocol = kind
	}
	if kind == "shadowtls" {
		in.ServerName = "www.microsoft.com"
	}
	return in
}

func TestExtraProtocolBoundaries(t *testing.T) {
	for _, kind := range []string{"socks", "mixed", "hysteria", "shadowtls", "snell", "snell6"} {
		t.Run(kind, func(t *testing.T) {
			s, err := NewSpec(extraInput(t, kind))
			if err != nil {
				t.Fatal(err)
			}
			if MinimumAgentVersion(kind) != "0.14.0-dev" && MinimumAgentVersion(kind) != "0.16.0-dev" {
				t.Fatal("old agents must not receive new kinds")
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
			in := cfg.Inbounds[0]
			switch kind {
			case "socks", "mixed":
				user := in["users"].([]any)[0].(map[string]any)
				if user["username"] != "xingdu" || user["password"] != s.Credential || in["tls"] != nil {
					t.Fatal("must require explicit username/password")
				}
			case "shadowtls":
				if len(cfg.Inbounds) != 2 || in["detour"] != "xingdu-inner" || cfg.Inbounds[1]["password"] != s.Credential || s.Password == "" || s.Password == s.Credential {
					t.Fatal("missing independent inner encryption")
				}
				for _, host := range []string{"localhost", "127.0.0.1", "169.254.169.254", "www.microsoft.com.evil.test", "custom.example.com"} {
					bad := s
					bad.ServerName = host
					if ValidateSpec(bad) == nil {
						t.Fatal("unapproved handshake accepted")
					}
				}
			case "snell", "snell6":
				want := float64(5)
				if kind == "snell6" {
					want = 6
				}
				if in["version"] != want || in["psk"] != s.Credential {
					t.Fatal("wrong Snell server version")
				}
			}
			if kind != "hysteria" && (cfg.Route.Rules[0]["network"] != "udp" || cfg.Route.Rules[0]["action"] != "reject") {
				t.Fatal("UDP must be blocked")
			}
			peer := Peer{Address: "203.0.113.1", Protocol: kind, Port: s.Port, Credential: s.Credential, Password: s.Password, ServerName: s.ServerName, Certificate: s.Certificate}
			if err := peer.Validate(); err != nil {
				t.Fatal(err)
			}
			out := peer.Outbounds()
			if kind == "shadowtls" && len(out) != 2 {
				t.Fatal("relay needs outer transport")
			}
			bad := s
			bad.Credential = ""
			if ValidateSpec(bad) == nil {
				t.Fatal("empty credential accepted")
			}
			if kind == "shadowtls" {
				bad = s
				bad.Password = ""
				if ValidateSpec(bad) == nil {
					t.Fatal("empty outer credential accepted")
				}
			}
		})
	}
	if ValidateInput(Input{Name: "Naive", Protocol: "naive", Port: 443}) == nil {
		t.Fatal("Naive must remain excluded")
	}
}
