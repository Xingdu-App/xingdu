package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTrustTunnelRuntimeContract(t *testing.T) {
	in := inputFixture(t)
	in.Protocol = "trusttunnel"
	s, err := NewSpec(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	var c TrustTunnelConfig
	if err = json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if c.Runtime != "trusttunnel" || len(c.Inbounds) != 1 || c.Inbounds[0].Port != s.Port {
		t.Fatal("invalid revision")
	}
	files, err := TrustTunnelFiles(c.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 || !strings.Contains(string(files[0]), "[listen_protocols.http2]") || strings.Contains(string(files[0]), "http3") || !strings.Contains(string(files[0]), "allow_private_network_connections = false") {
		t.Fatal("unsafe transport")
	}
	if strings.Contains(string(files[0]), s.Credential) || strings.Contains(string(files[1]), s.PrivateKey) {
		t.Fatal("secrets in manifest")
	}
	if !strings.Contains(string(files[2]), s.Credential) || string(files[4]) != s.PrivateKey {
		t.Fatal("missing credentials")
	}
	if Username(s.Protocol) != "xingdu" || IsQUIC(s.Protocol) || MinimumAgentVersion(s.Protocol) != "0.15.0-dev" {
		t.Fatal("invalid capabilities")
	}
	s.Relay = &Peer{}
	if _, err = Render(s); err == nil {
		t.Fatal("relay accepted")
	}
	s.Relay = nil
	s.Credential = "bad\"\ninjection"
	if _, err = Render(s); err == nil {
		t.Fatal("unsafe credential accepted")
	}
}
