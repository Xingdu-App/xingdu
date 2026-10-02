package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRealityIdentityAndRuntimeProjection(t *testing.T) {
	for _, engine := range []string{"sing-box", "xray"} {
		in := Input{Name: "Reality fixture", Protocol: "vless", Port: 443, ServerName: "www.microsoft.com", V2Ray: &V2RayOptions{Engine: engine, Network: "tcp", Reality: true, Flow: "xtls-rprx-vision"}}
		s, err := NewSpec(in)
		if err != nil {
			t.Fatal(err)
		}
		if s.NeedsCertificate() || s.Input.RequiredAgentVersion() != "0.17.0-dev" {
			t.Fatal("wrong feature gate or certificate requirement")
		}
		var b []byte
		if engine == "xray" {
			b, err = XrayServer(s, "")
		} else {
			b, err = Render(s)
		}
		if err != nil || !strings.Contains(string(b), s.RealityPrivateKey) || strings.Contains(string(b), "certificate") {
			t.Fatal("invalid REALITY server projection", err)
		}
		if err = ReconcileFeatureKeys(&s, false); err != nil {
			t.Fatal(err)
		}
		old := s.RealityPublicKey
		if err = ReconcileFeatureKeys(&s, true); err != nil || s.RealityPublicKey == old {
			t.Fatal("rotation failed", err)
		}
		s.RealityPublicKey = old
		if ValidateSpec(s) == nil {
			t.Fatal("mismatched key pair accepted")
		}
	}
}
func TestRealityRejectsUnsafeAndLossyInputs(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "metadata.google.internal", "private.example.com", "www.microsoft.com:80"} {
		in := Input{Name: "Unsafe", Protocol: "vless", Port: 443, ServerName: host, V2Ray: &V2RayOptions{Network: "tcp", Reality: true}}
		if ValidateInput(in) == nil {
			t.Fatal("unsafe handshake target accepted")
		}
	}
	for _, v := range []V2RayOptions{{Network: "ws", Path: "/", Reality: true}, {Network: "tcp", Reality: true, Encryption: true}, {Network: "tcp", Reality: true, ALPN: []string{"h2"}}} {
		if ValidateInput(Input{Name: "Invalid", Protocol: "vless", Port: 443, ServerName: "www.microsoft.com", V2Ray: &v}) == nil {
			t.Fatal("lossy combination accepted")
		}
	}
}
func TestTrojanTransportSettingsUsePassword(t *testing.T) {
	for _, engine := range []string{"sing-box", "xray"} {
		for _, network := range []string{"ws", "grpc"} {
			in := inputFixture(t)
			in.Protocol = "trojan"
			in.V2Ray = &V2RayOptions{Engine: engine, Network: network}
			if network == "ws" {
				in.V2Ray.Path = "/fixture"
			} else {
				in.V2Ray.ServiceName = "fixture"
			}
			s, err := NewSpec(in)
			if err != nil {
				t.Fatal(err)
			}
			if in.RequiredAgentVersion() != "0.17.0-dev" {
				t.Fatal("missing gate")
			}
			var b []byte
			if engine == "xray" {
				b, err = XrayServer(s, "")
			} else {
				b, err = Render(s)
			}
			var server map[string]any
			if err != nil || json.Unmarshal(b, &server) != nil {
				t.Fatal("invalid server config", err)
			}
			inbound := server["inbounds"].([]any)[0].(map[string]any)
			users := inbound["users"]
			if engine == "xray" {
				users = inbound["settings"].(map[string]any)["clients"]
			}
			if users.([]any)[0].(map[string]any)["password"] != s.Credential {
				t.Fatal("wrong authentication schema")
			}
			if engine == "xray" {
				b, err = XrayClient(s, "127.0.0.1", 21000)
				var cfg map[string]any
				if err != nil || json.Unmarshal(b, &cfg) != nil || !strings.Contains(string(b), `"servers"`) {
					t.Fatal("wrong client schema", err)
				}
			}
		}
	}
}
