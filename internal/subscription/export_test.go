package subscription

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
	"xingdu.app/xingdu/internal/protocol"
)

func fixture(t *testing.T, kind string, index int) Node {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"node.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := protocol.NewSpec(protocol.Input{Name: "Demo", Protocol: kind, Port: 443, ServerName: "node.example.com", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}))})
	if err != nil {
		t.Fatal(err)
	}
	return Node{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", index), Name: "星渡: # &x [node], REJECT", Server: "2001:db8::1", Spec: spec}
}

func TestMihomoExport(t *testing.T) {
	kinds := []string{"trojan", "vless", "vmess", "hysteria2", "tuic"}
	var nodes []Node
	for i, k := range kinds {
		nodes = append(nodes, fixture(t, k, i))
	}
	rules := []Rule{{"domain", "EXAMPLE.COM", "proxy"}, {"domain_suffix", "example.org", "direct"}, {"ip_cidr", "192.0.2.10/24", "reject"}, {"ip_cidr", "2001:db8::1/32", "direct"}}
	b, err := RenderClash("示例", nodes, rules, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
		Groups  []struct {
			Name    string
			Type    string
			Proxies []string
		} `yaml:"proxy-groups"`
		Rules    []string
		AllowLAN bool `yaml:"allow-lan"`
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Proxies) != 5 || len(cfg.Groups) != 1 || cfg.Groups[0].Type != "select" || cfg.AllowLAN {
		t.Fatal("invalid configuration topology")
	}
	if !reflect.DeepEqual(cfg.Rules, []string{"DOMAIN,example.com,Xingdu", "DOMAIN-SUFFIX,example.org,DIRECT", "IP-CIDR,192.0.2.0/24,REJECT", "IP-CIDR6,2001:db8::/32,DIRECT", "MATCH,Xingdu"}) {
		t.Fatal("rule order or normalization incorrect")
	}
	for i, p := range cfg.Proxies {
		node := nodes[i]
		block, _ := pem.Decode([]byte(node.Spec.Certificate))
		digest := sha256.Sum256(block.Bytes)
		if p["fingerprint"] != hex.EncodeToString(digest[:]) || p["skip-cert-verify"] != false {
			t.Fatal("TLS pin missing")
		}
		if p["server"] != "2001:db8::1" || p["name"] != node.Name+" ["+node.ID+"]" || p["type"] != kinds[i] {
			t.Fatal("escaped name or IPv6 address corrupted")
		}
		if cfg.Groups[0].Proxies[i] != p["name"] {
			t.Fatal("selector and name mismatch")
		}
		if strings.Contains(string(b), node.Spec.PrivateKey) || strings.Contains(string(b), "PRIVATE KEY") || p["certificate"] != nil {
			t.Fatal("server private material exported")
		}
		if kinds[i] == "vless" || kinds[i] == "vmess" {
			if p["uuid"] != node.Spec.Credential || p["tls"] != true || p["servername"] != node.Spec.ServerName {
				t.Fatal("incorrect UUID/TLS")
			}
		}
		if kinds[i] == "tuic" {
			if p["uuid"] != node.Spec.Credential || p["password"] != node.Spec.Password || p["token"] != nil {
				t.Fatal("incorrect TUIC version or credentials")
			}
		}
	}
	if cfg.Proxies[2]["alterId"] != 0 || cfg.Proxies[2]["cipher"] != "auto" {
		t.Fatal("incorrect VMess cipher")
	}
	// Stable identities keep repeated display names unique across refreshes.
	if cfg.Groups[0].Proxies[0] == cfg.Groups[0].Proxies[1] {
		t.Fatal("duplicate proxy identity")
	}
}

func TestRuleGrammar(t *testing.T) {
	for _, rule := range []Rule{{"domain", "example.com,DIRECT", "proxy"}, {"domain", "*.example.com", "proxy"}, {"domain_suffix", "example.com\nMATCH,DIRECT", "direct"}, {"domain_suffix", "-bad.example", "direct"}, {"ip_cidr", "192.0.2.0/33", "reject"}, {"ip_cidr", "2001:db8::/129", "direct"}, {"domain", "example.com", "Xingdu"}, {"MATCH", "anything", "direct"}} {
		if ValidateRules([]Rule{rule}, "proxy") == nil {
			t.Fatalf("invalid rule accepted: %+v", rule)
		}
	}
	if ValidateRules(nil, "reject") == nil || ValidateRules(make([]Rule, 101), "proxy") == nil {
		t.Fatal("limits not applied")
	}
}

func TestUnavailableAndCorruptNodes(t *testing.T) {
	if _, err := RenderClash("Demo", nil, nil, "proxy"); !errors.Is(err, ErrNoNodes) {
		t.Fatal("missing unavailable error")
	}
	n := fixture(t, "trojan", 1)
	for _, change := range []func(*Node){func(n *Node) { n.Server = "https://node.example.com/path" }, func(n *Node) { n.Server = "[2001:db8::1]" }, func(n *Node) { n.Name = "bad\nname" }, func(n *Node) { n.Spec.Certificate = "invalid" }, func(n *Node) { n.Spec.Credential = "invalid" }, func(n *Node) { n.Spec.ServerName = "wrong.example.com" }} {
		copy := n
		change(&copy)
		if _, err := RenderClash("Demo", []Node{copy}, nil, "direct"); err == nil {
			t.Fatal("corrupt node accepted")
		}
	}
	if _, err := RenderClash("Demo", []Node{n, n}, nil, "direct"); err == nil {
		t.Fatal("duplicate node accepted")
	}
}

// Opt-in check against an independently downloaded official Mihomo binary.
// It does not start a listener or contact a proxy server.
func TestMihomoParser(t *testing.T) {
	binary := os.Getenv("XINGDU_TEST_MIHOMO_BINARY")
	if binary == "" {
		t.Skip("XINGDU_TEST_MIHOMO_BINARY not set")
	}
	var nodes []Node
	for i, kind := range []string{"trojan", "vless", "vmess", "hysteria2", "tuic"} {
		nodes = append(nodes, fixture(t, kind, i))
	}
	b, err := RenderClash("Parser check", nodes, []Rule{{"domain", "example.com", "proxy"}, {"domain_suffix", "example.org", "direct"}, {"ip_cidr", "2001:db8::/32", "reject"}}, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err = os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "-t", "-d", dir, "-f", path).CombinedOutput()
	if err != nil {
		t.Fatalf("Mihomo rejected export: %v\n%s", err, output)
	}
}

func TestStashExport(t *testing.T) {
	var nodes []Node
	for i, kind := range []string{"trojan", "vless", "vmess", "hysteria2", "tuic"} {
		nodes = append(nodes, fixture(t, kind, i))
	}
	b, err := Render("stash", "Stash", nodes, []Rule{{"domain_suffix", "example.com", "direct"}, {"ip_cidr", "2001:db8::/32", "reject"}}, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Proxies   []map[string]any `yaml:"proxies"`
		Rules     []string
		MixedPort *int `yaml:"mixed-port"`
	}
	if err = yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.MixedPort != nil || len(cfg.Proxies) != 5 {
		t.Fatal("invalid Stash config")
	}
	for i, p := range cfg.Proxies {
		cert, _ := pem.Decode([]byte(nodes[i].Spec.Certificate))
		digest := sha256.Sum256(cert.Bytes)
		if p["server-cert-fingerprint"] != hex.EncodeToString(digest[:]) || p["fingerprint"] != nil || p["skip-cert-verify"] != false || p["sni"] != nodes[i].Spec.ServerName {
			t.Fatal("Stash TLS mapping incorrect")
		}
	}
	if cfg.Proxies[3]["auth"] != nodes[3].Spec.Credential || cfg.Proxies[3]["password"] != nil {
		t.Fatal("Stash Hysteria 2 auth lost")
	}
	if cfg.Proxies[4]["version"] != 5 || cfg.Proxies[4]["password"] != nodes[4].Spec.Password {
		t.Fatal("Stash TUIC v5 mapping incorrect")
	}
	if cfg.Proxies[1]["tls"] != true || cfg.Proxies[2]["tls"] != true || cfg.Proxies[2]["cipher"] != "auto" {
		t.Fatal("TLS VMess/VLESS lost")
	}
	if strings.Contains(string(b), "PRIVATE KEY") {
		t.Fatal("private key exported")
	}
	if !reflect.DeepEqual(cfg.Rules, []string{"DOMAIN-SUFFIX,example.com,DIRECT", "IP-CIDR6,2001:db8::/32,REJECT", "MATCH,Xingdu"}) {
		t.Fatal("rules changed")
	}
	if _, err = Render("surge", "Unsupported", nodes, nil, "proxy"); err == nil {
		t.Fatal("unsupported client accepted")
	}
}
