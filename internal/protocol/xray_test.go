package protocol

import (
	"encoding/json"
	"testing"
)

func TestXrayListenerSupportsIPv6(t *testing.T) {
	noTLS := false
	spec, err := NewSpec(Input{Name: "Dual stack", Protocol: "vless", Port: 443, V2Ray: &V2RayOptions{Engine: "xray", Network: "tcp", TLS: &noTLS}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := XrayServer(spec, "")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Inbounds []struct {
			Listen string `json:"listen"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(raw, &cfg) != nil || len(cfg.Inbounds) != 1 || cfg.Inbounds[0].Listen != "::" {
		t.Fatal("Xray listener must include IPv6")
	}
}
