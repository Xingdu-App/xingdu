package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestV2RayRenderMatrix(t *testing.T) {
	for _, kind := range []string{"vless", "vmess"} {
		for _, network := range []string{"tcp", "ws", "grpc", "http"} {
			for _, enabled := range []bool{true, false} {
				in := inputFixture(t)
				in.Protocol = kind
				in.V2Ray = &V2RayOptions{Network: network, TLS: &enabled}
				if network == "grpc" {
					in.V2Ray.ServiceName = "lab"
				} else if network != "tcp" {
					in.V2Ray.Path = "/lab"
					in.V2Ray.Host = "proxy.example.com"
				}
				if !enabled {
					in.ServerName = ""
					in.Certificate = ""
					in.PrivateKey = ""
				}
				spec, err := NewSpec(in)
				if err != nil {
					t.Fatal(err)
				}
				data, err := Render(spec)
				if err != nil {
					t.Fatal(err)
				}
				var cfg struct {
					Inbounds []map[string]any
					Route    map[string]any
				}
				if err = json.Unmarshal(data, &cfg); err != nil {
					t.Fatal(err)
				}
				inbound := cfg.Inbounds[0]
				_, hasTLS := inbound["tls"]
				if hasTLS != enabled {
					t.Fatalf("%s/%s TLS lost", kind, network)
				}
				if network == "tcp" {
					if inbound["transport"] != nil {
						t.Fatal("plain TCP gained a transport")
					}
				} else if inbound["transport"].(map[string]any)["type"] != network {
					t.Fatal("transport downgraded")
				}
				if !strings.Contains(string(data), `"action": "reject"`) {
					t.Fatal("lost private egress protection")
				}
			}
		}
	}
}
func TestV2RayRejectLossyCombinations(t *testing.T) {
	disabled := false
	for _, options := range []V2RayOptions{
		{Network: "xhttp"}, {Network: "httpupgrade", Path: "/"}, {Network: "ws", Path: "/ok\r\nInjected: true"},
		{Network: "ws", Path: "/", Host: "a..b"}, {Network: "tcp", Path: "/"},
		{Network: "grpc"}, {Network: "ws", Path: "/", ServiceName: "no"},
		{Network: "grpc", ServiceName: "a/b"}, {Network: "grpc", ServiceName: "lab", ALPN: []string{"http/1.1"}},
		{Network: "ws", Path: "/", ALPN: []string{"h2"}}, {Network: "tcp", ALPN: []string{"h3"}},
		{Network: "tcp", TLS: &disabled, ALPN: []string{"h2"}},
		{Network: "tcp", TLS: &disabled, Flow: "xtls-rprx-vision"},
		{Network: "grpc", ServiceName: "lab", Flow: "xtls-rprx-vision"},
	} {
		in := inputFixture(t)
		in.Protocol = "vless"
		in.V2Ray = &options
		if ValidateInput(in) == nil {
			t.Fatalf("accepted %#v", options)
		}
	}
	in := inputFixture(t)
	in.V2Ray = &V2RayOptions{Network: "tcp", TLS: &disabled}
	if ValidateInput(in) == nil {
		t.Fatal("accepted plaintext Trojan")
	}
	in.Protocol = "vless"
	in.V2Ray.TLS = &disabled
	if ValidateInput(in) == nil {
		t.Fatal("plaintext accepted stale TLS material")
	}
}
func TestV2RayVisionAndVersion(t *testing.T) {
	in := inputFixture(t)
	in.Protocol = "vless"
	in.V2Ray = &V2RayOptions{Network: "tcp", Flow: "xtls-rprx-vision"}
	spec, err := NewSpec(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(spec)
	if err != nil || !strings.Contains(string(b), `"flow": "xtls-rprx-vision"`) {
		t.Fatal("lost Vision flow", err)
	}
	if MinimumAgentVersion("vless") != "0.16.0-dev" || MinimumAgentVersion("vmess") != "0.16.0-dev" {
		t.Fatal("old Agent would ignore transport")
	}
}
