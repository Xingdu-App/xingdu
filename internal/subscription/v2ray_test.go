package subscription

import (
	"context"
	"go.yaml.in/yaml/v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func TestV2RayExportsPreserveTransport(t *testing.T) {
	for _, format := range []string{"stash", "mihomo"} {
		for _, kind := range []string{"vless", "vmess"} {
			for _, network := range []string{"tcp", "ws", "grpc", "http"} {
				for _, tls := range []bool{false, true} {
					node := fixture(t, kind, 1)
					node.Spec.V2Ray = &protocol.V2RayOptions{Network: network, TLS: &tls}
					if network == "grpc" {
						node.Spec.V2Ray.ServiceName = "lab"
					} else if network != "tcp" {
						node.Spec.V2Ray.Path = "/lab"
						node.Spec.V2Ray.Host = "proxy.example.com"
					}
					if !tls {
						node.Spec.Certificate = ""
						node.Spec.PrivateKey = ""
						node.Spec.ServerName = ""
					}
					b, err := Render(format, "Lab", []Node{node}, nil, "proxy")
					if format == "stash" && (network == "grpc" && !tls) {
						if err == nil {
							t.Fatal("unsupported Stash combination exported")
						}
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					var cfg struct{ Proxies []map[string]any }
					if err = yaml.Unmarshal(b, &cfg); err != nil {
						t.Fatal(err)
					}
					p := cfg.Proxies[0]
					expected := network
					if network == "http" && tls {
						expected = "h2"
					}

					if p["network"] != expected || p["tls"] != tls {
						t.Fatalf("lost %s/%s/%s TLS=%v", format, kind, network, tls)
					}
					if !tls && (p["fingerprint"] != nil || p["server-cert-fingerprint"] != nil || p["sni"] != nil) {
						t.Fatal("plaintext exported TLS metadata")
					}
					if strings.Contains(string(b), "PRIVATE KEY") {
						t.Fatal("server private key leaked")
					}
					if binary := os.Getenv("XINGDU_TEST_STASH_CONFIG_PROBE"); format == "stash" && binary != "" {
						dir := t.TempDir()
						path := filepath.Join(dir, "config.yaml")
						if err := os.WriteFile(path, b, 0600); err != nil {
							t.Fatal(err)
						}
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						out, err := exec.CommandContext(ctx, binary, path).CombinedOutput()
						cancel()
						if err != nil || string(out) != "1\tOK\nFULL\tOK\n" {
							t.Fatalf("Stash parser %s/%s TLS=%v: %v %s", kind, network, tls, err, out)
						}
					}
					if network == "grpc" && p["grpc-opts"].(map[string]any)["grpc-service-name"] != "lab" {
						t.Fatal("lost service name")
					}
					if network == "ws" && p["ws-opts"].(map[string]any)["path"] != "/lab" {
						t.Fatal("lost WS path")
					}
				}
			}
		}
	}
}
func TestV2RayRejectUnsupportedClientAdapters(t *testing.T) {
	n := fixture(t, "vmess", 1)
	n.Spec.V2Ray = &protocol.V2RayOptions{Network: "ws", Path: "/"}
	for _, format := range []string{"surge", "loon", "hysteria2_uri"} {
		if _, err := Render(format, "Lab", []Node{n}, nil, "proxy"); err == nil {
			t.Fatal("silently exported TCP", format)
		}
	}
}
