package subscription

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func realityNode(t *testing.T) Node {
	t.Helper()
	s, err := protocol.NewSpec(protocol.Input{Name: "Reality fixture", Protocol: "vless", Port: 443, ServerName: "www.microsoft.com", V2Ray: &protocol.V2RayOptions{Network: "tcp", Reality: true, Flow: "xtls-rprx-vision"}})
	if err != nil {
		t.Fatal(err)
	}
	return Node{ID: "node_00000000000000000000000000000009", Name: "Reality # / + 中文", Server: "2001:db8::1", Spec: s}
}
func TestNativeExportsPreserveIdentityAndRejectLoss(t *testing.T) {
	n := realityNode(t)
	for _, format := range []string{"singbox", "mihomo", "uri", "base64"} {
		b, err := Render(format, "fixture", []Node{n}, nil, "proxy")
		if err != nil {
			t.Fatal(format, err)
		}
		if strings.Contains(string(b), n.Spec.RealityPrivateKey) {
			t.Fatal("server private key exported")
		}
		if format == "base64" {
			b, err = base64.StdEncoding.DecodeString(string(b))
			if err != nil {
				t.Fatal(err)
			}
		}
		if !strings.Contains(string(b), n.Spec.RealityPublicKey) || !strings.Contains(string(b), n.Spec.RealityShortID) {
			t.Fatal("client identity missing", format)
		}
		if format == "uri" || format == "base64" {
			u, err := url.Parse(strings.TrimSpace(string(b)))
			if err != nil || u.Host != "[2001:db8::1]:443" || u.Fragment != n.Name || u.Query().Get("security") != "reality" {
				t.Fatal("incorrect escaped URI")
			}
		}
	}
	if _, err := Render("stash", "fixture", []Node{n}, nil, "proxy"); err == nil {
		t.Fatal("unverified REALITY adapter accepted")
	}
	xray := n
	v := *n.Spec.V2Ray
	v.Engine = "xray"
	xray.Spec.V2Ray = &v
	for _, format := range []string{"singbox", "mihomo", "stash"} {
		if _, err := Render(format, "fixture", []Node{xray}, nil, "proxy"); err == nil {
			t.Fatal("incompatible or unverified Xray REALITY client accepted", format)
		}
	}
	for _, format := range []string{"uri", "base64"} {
		if _, err := Render(format, "fixture", []Node{fixture(t, "trojan", 1)}, nil, "proxy"); err == nil {
			t.Fatal("private CA silently weakened")
		}
		if _, err := Render(format, "fixture", []Node{n}, []Rule{{Type: "domain", Value: "example.com", Target: "direct"}}, "proxy"); err == nil {
			t.Fatal("rules silently discarded")
		}
	}
}
func TestSingboxExportOfficialParser(t *testing.T) {
	binary := os.Getenv("XINGDU_TEST_V2RAY_RUNTIME")
	nodes := []Node{realityNode(t)}
	for i, kind := range []string{"shadowsocks", "shadowsocks2022", "vless", "vmess", "trojan", "hysteria2", "tuic", "anytls", "http", "socks", "mixed", "hysteria"} {
		nodes = append(nodes, fixture(t, kind, i+20))
	}
	for i, network := range []string{"ws", "grpc"} {
		n := fixture(t, "trojan", i+40)
		in := n.Spec.Input
		in.V2Ray = &protocol.V2RayOptions{Network: network}
		if network == "ws" {
			in.V2Ray.Path = "/fixture"
			in.V2Ray.Host = "node.example.com"
		} else {
			in.V2Ray.ServiceName = "fixture"
		}
		var err error
		n.Spec, err = protocol.NewSpec(in)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, n)
	}
	b, err := Render("singbox", "fixture", nodes, []Rule{{Type: "domain_suffix", Value: "example.com", Target: "direct"}, {Type: "ip_cidr", Value: "192.0.2.0/24", Target: "reject"}}, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if json.Unmarshal(b, &cfg) != nil {
		t.Fatal("invalid JSON")
	}
	for _, n := range nodes {
		if strings.Contains(string(b), n.Spec.PrivateKey) && n.Spec.PrivateKey != "" {
			t.Fatal("TLS private key exported")
		}
	}
	if binary == "" {
		t.Log("parser check skipped; set XINGDU_TEST_V2RAY_RUNTIME")
		return
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "client.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--platform", "linux/arm64", "-v", binary+":/runtime:ro", "-v", dir+":/configs:ro", "--entrypoint", "/runtime", "xingdu-lab-amazon:latest", "check", "-c", "/configs/client.json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("official parser rejected native export: %v %s", err, out)
	}
	t.Log(fmt.Sprintf("official parser accepted %d nodes and ordered rules", len(nodes)))
}
