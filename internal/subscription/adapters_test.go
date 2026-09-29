package subscription

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurgeStrictAdapters(t *testing.T) {
	var nodes []Node
	for i, kind := range []string{"trojan", "vmess", "hysteria2", "tuic"} {
		nodes = append(nodes, fixture(t, kind, i))
	}
	nodes[0].Name = "DIRECT,evil=direct # [Rule]"
	b, err := Render("surge", "Surge", nodes, []Rule{{"domain_suffix", "EXAMPLE.COM", "direct"}, {"ip_cidr", "2001:db8::1/32", "reject"}}, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, n := range nodes {
		if !strings.Contains(text, "server-cert-fingerprint-sha256="+certificatePin(n)) || !strings.Contains(text, n.Spec.Credential) || strings.Contains(text, n.Spec.PrivateKey) {
			t.Fatal("lost TLS pin/auth or exposed private key")
		}
		if strings.ContainsAny(iniLabel(n), ",=#[]\r\n") {
			t.Fatal("unsafe INI identity")
		}
	}
	for _, want := range []string{"tuic-v5,", "uuid=" + nodes[3].Spec.Credential + ", password=" + nodes[3].Spec.Password, "vmess-aead=true", "tls=true", "skip-cert-verify=false", "DOMAIN-SUFFIX,example.com,DIRECT\nIP-CIDR6,2001:db8::/32,REJECT\nFINAL,Xingdu"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing mapping: %s", want)
		}
	}
	// Every selected node is preserved; unsupported ones make the entire export fail.
	nodes = append(nodes, fixture(t, "vless", 9))
	if b, e := Render("surge", "Demo", nodes, nil, "direct"); e == nil || len(b) > 0 {
		t.Fatal("unsupported node silently dropped")
	}
}

func TestHysteriaURI(t *testing.T) {
	n := fixture(t, "hysteria2", 0)
	n.Name = "星渡 # ? & / %"
	b, err := Render("hysteria2_uri", "Share", []Node{n}, nil, "proxy")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "hysteria2" || u.Hostname() != n.Server || u.Port() != "443" || u.User.Username() != n.Spec.Credential || u.Fragment != n.Name {
		t.Fatal("URI escaping corrupted connection identity")
	}
	if u.Query().Get("insecure") != "0" || u.Query().Get("pinSHA256") != certificatePin(n) || u.Query().Get("sni") != n.Spec.ServerName {
		t.Fatal("URI TLS policy lost")
	}
	if strings.Contains(string(b), "PRIVATE KEY") {
		t.Fatal("private key exposed")
	}
	for _, tc := range []struct {
		nodes []Node
		rules []Rule
		final string
	}{
		{[]Node{n}, []Rule{{"domain", "example.com", "proxy"}}, "proxy"},
		{[]Node{n}, nil, "direct"},
		{[]Node{n, fixture(t, "trojan", 1)}, nil, "proxy"},
	} {
		if b, e := Render("hysteria2_uri", "Share", tc.nodes, tc.rules, tc.final); e == nil || len(b) > 0 {
			t.Fatal("share export discarded policy or protocol")
		}
	}
}

func TestLoonRejectsPrivateCA(t *testing.T) {
	_, err := Render("loon", "Loon", []Node{fixture(t, "trojan", 1)}, nil, "proxy")
	var compatibility *CompatibilityError
	if !errors.As(err, &compatibility) || !strings.Contains(compatibility.Reason, "证书链") {
		t.Fatal("private CA silently accepted", err)
	}
}

// A child process gives Go a fresh system-root cache, with an isolated fixture
// trust store. It never changes the machine trust store or downloads certificates.
func TestLoonWithIsolatedTrustStore(t *testing.T) {
	if fixturePath := os.Getenv("XINGDU_LOON_TEST_FIXTURE"); fixturePath != "" {
		rootPEM, err := os.ReadFile(os.Getenv("SSL_CERT_FILE"))
		if err != nil {
			t.Fatal(err)
		}
		roots := x509.NewCertPool()
		roots.AppendCertsFromPEM(rootPEM)
		x509.SetFallbackRoots(roots)
		raw, err := os.ReadFile(fixturePath)
		if err != nil {
			t.Fatal(err)
		}
		var nodes []Node
		if err = json.Unmarshal(raw, &nodes); err != nil {
			t.Fatal(err)
		}
		b, err := Render("loon", "Loon", nodes, []Rule{{"domain", "example.com", "proxy"}}, "proxy")
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for _, want := range []string{"trojan,", "vmess,", "vless,", "hysteria2,", "transport=tcp, over-tls=true", "tls-name=node.example.com, skip-cert-verify=false", "FINAL,Xingdu"} {
			if !strings.Contains(text, want) {
				t.Fatal("missing Loon field", want)
			}
		}
		for _, n := range nodes {
			if !strings.Contains(text, n.Spec.Credential) || strings.Contains(text, n.Spec.PrivateKey) {
				t.Fatal("invalid credential boundary")
			}
		}
		return
	}
	var nodes []Node
	for i, k := range []string{"trojan", "vmess", "vless", "hysteria2"} {
		nodes = append(nodes, fixture(t, k, i))
	}
	dir := t.TempDir()
	certs := ""
	for _, n := range nodes {
		certs += n.Spec.Certificate
	}
	certPath := filepath.Join(dir, "roots.pem")
	fixturePath := filepath.Join(dir, "nodes.json")
	raw, _ := json.Marshal(nodes)
	if err := os.WriteFile(certPath, []byte(certs), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixturePath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLoonWithIsolatedTrustStore$")
	cmd.Env = append(os.Environ(), "SSL_CERT_FILE="+certPath, "SSL_CERT_DIR="+dir, "XINGDU_LOON_TEST_FIXTURE="+fixturePath)
	// macOS uses the platform trust store and ignores SSL_CERT_FILE. The pure-Go
	// verifier can still be exercised through SetFallbackRoots + GODEBUG below.
	cmd.Env = append(cmd.Env, "GODEBUG=x509usefallbackroots=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Loon fixture validation failed: %v %s", err, output)
	}
}
