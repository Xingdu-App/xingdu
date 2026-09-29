package subscription

import (
	"errors"
	"go.yaml.in/yaml/v3"
	"strings"
	"testing"
)

func TestExtraProtocolAdapters(t *testing.T) {
	allowed := map[string][]string{
		"socks": {"stash", "mihomo", "surge"}, "mixed": {"stash", "mihomo", "surge"},
		"hysteria": {"stash", "mihomo"}, "shadowtls": {"stash", "mihomo"},
		"snell": {"stash", "surge"}, "snell6": {"surge"},
	}
	for kind, formats := range allowed {
		n := fixture(t, kind, 1)
		for _, format := range []string{"stash", "mihomo", "surge", "loon", "hysteria2_uri"} {
			t.Run(kind+"/"+format, func(t *testing.T) {
				ok := false
				for _, f := range formats {
					ok = ok || f == format
				}
				raw, err := Render(format, "Extra", []Node{n}, nil, "proxy")
				if !ok {
					var compatibility *CompatibilityError
					if !errors.As(err, &compatibility) {
						t.Fatal("unsupported export must fail explicitly", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), "PRIVATE KEY") || strings.Contains(string(raw), "skip-cert-verify: true") {
					t.Fatal("unsafe export")
				}
				if format == "surge" {
					if kind == "snell" && !strings.Contains(string(raw), "version=4") {
						t.Fatal("Snell server v5 must export v4 compatibility")
					}
					if kind == "snell6" && !strings.Contains(string(raw), "version=6") {
						t.Fatal("missing explicit Snell v6")
					}
					return
				}
				var cfg struct{ Proxies []map[string]any }
				if err := yaml.Unmarshal(raw, &cfg); err != nil {
					t.Fatal(err)
				}
				p := cfg.Proxies[0]
				if kind == "shadowtls" {
					opts := p["plugin-opts"].(map[string]any)
					if p["plugin"] != "shadow-tls" || p["password"] != n.Spec.Credential || opts["password"] != n.Spec.Password || opts["version"] != 3 {
						t.Fatal("missing separate ShadowTLS authentication")
					}
				}
				if kind != "hysteria" && p["udp"] != false {
					t.Fatal("must advertise TCP only")
				}
				if kind == "hysteria" && p["auth-str"] != n.Spec.Credential {
					t.Fatal("wrong Hysteria 1 auth field")
				}
			})
		}
	}
}
