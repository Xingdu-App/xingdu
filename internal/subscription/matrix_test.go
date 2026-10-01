package subscription

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"xingdu.app/xingdu/internal/protocol"
)

// This verifies the export boundary, not connectivity of any Stash release.
func TestCompatibility72StashProjection(t *testing.T) {
	b, err := os.ReadFile("../protocol/testdata/compatibility72.json")
	if err != nil {
		t.Fatal(err)
	}
	var inputs []protocol.Input
	if err = json.Unmarshal(b, &inputs); err != nil {
		t.Fatal(err)
	}
	tls := fixture(t, "trojan", 1).Spec.Input
	rejected := map[int]bool{}
	for _, n := range []int{13, 14, 17, 18, 19, 20, 23, 24, 25, 26, 27, 28, 29, 30, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 51, 53, 54, 57, 58, 61, 63, 69} {
		rejected[n] = true
	}
	for i, in := range inputs {
		t.Run(fmt.Sprintf("case_%02d", i+1), func(t *testing.T) {
			if in.NeedsCertificate() {
				in.ServerName, in.Certificate, in.PrivateKey = tls.ServerName, tls.Certificate, tls.PrivateKey
			}
			spec, err := protocol.NewSpec(in)
			if err != nil {
				t.Fatal(err)
			}
			b, err := Render("stash", "Lab", []Node{{ID: fmt.Sprintf("node_%032x", i+1), Name: in.Name, Server: "proxy.example.com", Spec: spec}}, nil, "proxy")
			if rejected[i+1] {
				if err == nil {
					t.Fatal("unsupported combination silently exported")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{spec.PrivateKey, spec.EncryptionKey} {
				if secret != "" && strings.Contains(string(b), secret) {
					t.Fatal("server key leaked")
				}
			}
			if spec.WireGuardKeys != nil {
				var document struct {
					Proxies []map[string]any `yaml:"proxies"`
				}
				if err := yaml.Unmarshal(b, &document); err != nil {
					t.Fatal(err)
				}
				if len(document.Proxies) != 1 || document.Proxies[0]["preshared-key"] != spec.WireGuardClient()["pre_shared_key"] {
					t.Fatal("Stash WireGuard PSK field lost")
				}
				if _, exists := document.Proxies[0]["pre-shared-key"]; exists {
					t.Fatal("Mihomo PSK spelling leaked into Stash")
				}
				mihomo, err := Render("mihomo", "Lab", []Node{{ID: fmt.Sprintf("node_%032x", i+1), Name: in.Name, Server: "proxy.example.com", Spec: spec}}, nil, "proxy")
				if err != nil {
					t.Fatal(err)
				}
				document.Proxies = nil
				if err := yaml.Unmarshal(mihomo, &document); err != nil {
					t.Fatal(err)
				}
				if len(document.Proxies) != 1 || document.Proxies[0]["pre-shared-key"] != spec.WireGuardClient()["pre_shared_key"] {
					t.Fatal("Mihomo WireGuard PSK field lost")
				}
				if _, exists := document.Proxies[0]["preshared-key"]; exists {
					t.Fatal("Stash PSK spelling leaked into Mihomo")
				}
				if strings.Contains(string(b), spec.WireGuardKeys.ServerPrivate) {
					t.Fatal("server WireGuard key leaked")
				}
				if !strings.Contains(string(b), spec.WireGuardKeys.ClientPrivate) {
					t.Fatal("client key missing")
				}
			}
			if spec.V2Ray != nil && spec.V2Ray.Encryption && !strings.Contains(string(b), spec.ClientEncryption()) {
				t.Fatal("encryption lost")
			}
		})
	}
}
