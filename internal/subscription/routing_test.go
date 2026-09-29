package subscription

import (
	"context"
	"encoding/json"
	"go.yaml.in/yaml/v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func routingFixture(t *testing.T, id string) *Routing {
	t.Helper()
	var raw struct {
		Presets []struct {
			ID     string
			Groups []RoutingGroup
			Final  string
		}
	}
	if err := json.Unmarshal(RoutingCatalog(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, p := range raw.Presets {
		if p.ID == id {
			return &Routing{Preset: id, Groups: p.Groups, Targets: map[string]string{}, Final: p.Final}
		}
	}
	t.Fatal("missing preset")
	return nil
}

func TestRoutingCatalogAndExports(t *testing.T) {
	nodes := []Node{fixture(t, "shadowsocks", 1), fixture(t, "shadowsocks", 2)}
	nodes[0].Name = "默认代理" // Policy/node namespaces must remain unambiguous.
	ids := []string{nodes[0].ID, nodes[1].ID}
	for _, p := range catalog.Presets {
		for _, format := range []string{"stash", "mihomo", "surge"} {
			t.Run(p.ID+"/"+format, func(t *testing.T) {
				r := routingFixture(t, p.ID)
				if len(r.Groups) > 1 {
					r.Groups[1].NodeIDs = []string{ids[1]}
					r.Groups[1].Type = "fallback"
				}
				r.Targets["ads"] = "disabled"
				rules := []Rule{{Type: "domain", Value: "override.example", Target: "group:proxy"}}
				data, err := RenderRouting(format, "Test", nodes, rules, "proxy", r, ids)
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				if strings.Contains(text, "BanAD.list") || !strings.Contains(text, "ChinaDomain.list") {
					t.Fatal("source enable/disable not respected")
				}
				if format == "surge" {
					for _, want := range []string{"proxy-test-url = https://www.gstatic.com/generate_204", "默认代理 - 2 = url-test,", "RULE-SET,https://raw.githubusercontent.com/ACL4SSR/", "FINAL," + finalLabel(r)} {
						if !strings.Contains(text, want) {
							t.Fatal("missing", want)
						}
					}
					if strings.Index(text, "DOMAIN,override.example") > strings.Index(text, "RULE-SET,") {
						t.Fatal("custom rules must precede sets")
					}
					return
				}
				var cfg struct {
					Groups []struct {
						Name, Type string
						Proxies    []string
					} `yaml:"proxy-groups"`
					Rules     []string
					Providers map[string]any `yaml:"rule-providers"`
				}
				if err = yaml.Unmarshal(data, &cfg); err != nil {
					t.Fatal(err)
				}
				if cfg.Groups[0].Name != "默认代理 - 2" || cfg.Rules[0] != "DOMAIN,override.example,默认代理 - 2" || cfg.Rules[len(cfg.Rules)-1] != "MATCH,"+finalLabel(r) {
					t.Fatal("targets or priority incorrect", cfg)
				}
				if len(cfg.Providers) != len(p.Bindings)-1 {
					t.Fatal("missing full sources")
				}
				for i, g := range cfg.Groups {
					if len(g.Proxies) == 0 {
						t.Fatal("empty group causes implicit DIRECT")
					}
					if i > 1 && (g.Type != "select" || g.Proxies[0] != cfg.Groups[0].Name) {
						t.Fatal("unassigned group must inherit default")
					}
				}
			})
		}
	}
}

func TestRoutingValidationAndUnavailableMembers(t *testing.T) {
	nodes := []Node{fixture(t, "shadowsocks", 1)}
	ids := []string{nodes[0].ID, "node_00000000000000000000000000000002"}
	mutations := map[string]func(*Routing){
		"unknown preset":            func(r *Routing) { r.Preset = "url:https://example.com" },
		"missing root":              func(r *Routing) { r.Groups = r.Groups[1:] },
		"unknown member":            func(r *Routing) { r.Groups[1].NodeIDs = []string{"node_00000000000000000000000000000003"} },
		"duplicate member":          func(r *Routing) { r.Groups[1].NodeIDs = []string{ids[0], ids[0]} },
		"root excludes members":     func(r *Routing) { r.Groups[0].NodeIDs = []string{ids[0]} },
		"injection":                 func(r *Routing) { r.Groups[1].Name = "evil,DIRECT" },
		"newline":                   func(r *Routing) { r.Groups[1].Name = "evil\n[Rule]" },
		"comment injection":         func(r *Routing) { r.Groups[1].Name = "// comment" },
		"global reserved":           func(r *Routing) { r.Groups[1].Name = "GLOBAL" },
		"case-insensitive reserved": func(r *Routing) { r.Groups[1].Name = "Direct" },
		"reserved":                  func(r *Routing) { r.Groups[1].Name = "REJECT" },
		"duplicate name":            func(r *Routing) { r.Groups[1].Name = r.Groups[0].Name },
		"duplicate id":              func(r *Routing) { r.Groups[1].ID = "proxy" },
		"bad mode":                  func(r *Routing) { r.Groups[1].Type = "relay" },
		"missing target":            func(r *Routing) { r.Targets["ads"] = "group:missing" },
		"arbitrary source":          func(r *Routing) { r.Targets["https://example.com"] = "direct" },
		"bad final":                 func(r *Routing) { r.Final = "disabled" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r := routingFixture(t, "streaming-v1")
			mutate(r)
			if ValidateRouting(r, ids, nil, "proxy", "stash") == nil {
				t.Fatal("accepted invalid routing")
			}
		})
	}
	r := routingFixture(t, "streaming-v1")
	r.Groups[1].NodeIDs = []string{ids[1]}
	r.Groups[1].Type = "url-test"
	data, err := RenderRouting("stash", "Test", nodes, nil, "proxy", r, ids)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Groups []struct {
			Name, Type string
			Proxies    []string
		} `yaml:"proxy-groups"`
	}
	yaml.Unmarshal(data, &cfg)
	if cfg.Groups[1].Type != "select" || cfg.Groups[1].Proxies[0] != cfg.Groups[0].Name {
		t.Fatal("unavailable group silently bypasses proxy")
	}
	if _, err = RenderRouting("stash", "Test", nil, nil, "proxy", r, ids); err == nil {
		t.Fatal("empty subscription must fail")
	}
	if _, err = RenderRouting("hysteria2_uri", "Test", nodes, nil, "proxy", r, ids); err == nil {
		t.Fatal("URI cannot carry routing")
	}
	if ValidateRouting(nil, ids, []Rule{{"domain", "example.com", "group:proxy"}}, "proxy", "stash") == nil {
		t.Fatal("legacy rules may not reference groups")
	}
	if ValidateRouting(r, ids, []Rule{{"domain", "example.com", "group:missing"}}, "proxy", "stash") == nil {
		t.Fatal("missing custom rule target")
	}
	old, _ := Render("stash", "Test", nodes, nil, "proxy")
	same, _ := RenderRouting("stash", "Test", nodes, nil, "proxy", nil, ids)
	if string(old) != string(same) {
		t.Fatal("legacy export changed")
	}
}

// Uses local fixture caches, so syntax validation needs no upstream network or
// live server. It does not establish Stash/Surge/Loon application acceptance.
func TestRoutingMihomoParser(t *testing.T) {
	binary := os.Getenv("XINGDU_TEST_MIHOMO_BINARY")
	if binary == "" {
		t.Skip("optional Mihomo binary not set")
	}
	for _, p := range catalog.Presets {
		t.Run(p.ID, func(t *testing.T) {
			nodes := []Node{fixture(t, "shadowsocks", 1)}
			r := routingFixture(t, p.ID)
			b, err := RenderRouting("mihomo", "Parser", nodes, nil, "proxy", r, []string{nodes[0].ID})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			os.Mkdir(filepath.Join(dir, "rules"), 0700)
			for _, s := range catalog.Sources {
				contents := []byte("DOMAIN-SUFFIX,example.com\nIP-CIDR,192.0.2.0/24,no-resolve\n")
				if cache := os.Getenv("XINGDU_TEST_ROUTING_RULE_CACHE"); cache != "" {
					contents, err = os.ReadFile(filepath.Join(cache, "xd-"+s.ID+".list"))
					if err != nil {
						t.Fatal(err)
					}
				}
				if err = os.WriteFile(filepath.Join(dir, "rules", "xd-"+s.ID+".list"), contents, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "config.yaml")
			os.WriteFile(path, b, 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, binary, "-t", "-d", dir, "-f", path).CombinedOutput()
			if err != nil {
				t.Fatalf("Mihomo rejected preset: %v\n%s", err, output)
			}
		})
	}
}

func finalLabel(r *Routing) string {
	if r.Final == "direct" {
		return "DIRECT"
	}
	return "默认代理 - 2"
}

func TestCombinedPresetsCoverWorkAndEntertainment(t *testing.T) {
	sourceIDs := map[string]bool{}
	for _, source := range catalog.Sources {
		if sourceIDs[source.ID] || !strings.HasPrefix(source.URL, "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/") {
			t.Fatal("invalid or duplicate trusted source", source.ID)
		}
		sourceIDs[source.ID] = true
	}
	for _, id := range []string{"allround-v1", "allround-fine-v1", "entertainment-v1", "gaming-v1", "work-cloud-v1", "direct-first-v1"} {
		t.Run(id, func(t *testing.T) {
			preset, ok := presetFor(id)
			if !ok {
				t.Fatal("missing preset")
			}
			positions := map[string]int{}
			targets := map[string]string{}
			for i, b := range preset.Bindings {
				if !sourceIDs[b.Source] {
					t.Fatal("unknown source", b.Source)
				}
				if _, exists := positions[b.Source]; exists {
					t.Fatal("duplicate rule set", b.Source)
				}
				positions[b.Source], targets[b.Source] = i, b.Target
			}
			for _, source := range []string{"ai", "claude", "gemini", "github", "docker", "developer", "netflix", "youtube", "disney", "spotify", "steam", "telegram", "discord", "microsoft", "google"} {
				if !strings.HasPrefix(targets[source], "group:") {
					t.Fatal("combined preset omitted a workload", source)
				}
			}
			for _, pair := range [][2]string{{"gemini", "google"}, {"youtube", "google"}, {"github", "microsoft"}, {"docker", "developer"}, {"steam-cn", "steam"}, {"game-download", "epic"}, {"netflix", "proxy"}, {"proxy", "china"}} {
				if positions[pair[0]] >= positions[pair[1]] {
					t.Fatal("specific service shadowed by broad source", pair)
				}
			}
			if targets["steam-cn"] != "direct" {
				t.Fatal("domestic downloads should precede gaming proxies")
			}
			if id == "allround-v1" && (targets["github"] != "group:developer" || targets["netflix"] != "group:media" || targets["claude"] != "group:ai") {
				t.Fatal("compact groups lost their purpose")
			}
			if id == "direct-first-v1" && routingFixture(t, id).Final != "direct" {
				t.Fatal("unexpected proxy catchall")
			}
		})
	}
}
