package subscription

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

//go:embed routing-catalog.json
var catalogJSON []byte

// RoutingCatalog returns public template metadata, without tenant data.
func RoutingCatalog() json.RawMessage { return append(json.RawMessage(nil), catalogJSON...) }

type RoutingGroup struct {
	Icon    string   `json:"icon,omitempty"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	NodeIDs []string `json:"node_ids"`
}

// Routing is a subscription-owned snapshot. Catalog versions are immutable;
// upstream rule contents are refreshed by the client, never fetched by the API.
type Routing struct {
	Preset  string            `json:"preset"`
	Groups  []RoutingGroup    `json:"groups"`
	Targets map[string]string `json:"targets"`
	Final   string            `json:"final"`
}

type ruleSource struct{ ID, URL string }
type binding struct{ Source, Target string }
type routingPreset struct {
	ID       string
	Bindings []binding
}
type routingCatalog struct {
	Sources []ruleSource
	Presets []routingPreset
}

var catalog = func() routingCatalog {
	var c routingCatalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}()
var groupID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func reservedGroupName(name string) bool {
	switch strings.ToLower(name) {
	case "direct", "reject", "reject-drop", "pass", "global":
		return true
	}
	return false
}

func presetFor(id string) (routingPreset, bool) {
	for _, p := range catalog.Presets {
		if p.ID == id {
			return p, true
		}
	}
	return routingPreset{}, false
}

func ValidateRouting(r *Routing, nodeIDs []string, rules []Rule, final, format string) error {
	if r == nil {
		return ValidateRules(rules, final)
	}
	if format == "hysteria2_uri" {
		return &CompatibilityError{Reason: "分享链接不支持配置模板，请选择完整配置格式"}
	}
	p, ok := presetFor(r.Preset)
	if !ok || len(r.Groups) < 1 || len(r.Groups) > 20 {
		return errors.New("invalid routing template or groups")
	}
	if final != "proxy" && final != "direct" {
		return errors.New("invalid legacy final policy")
	}
	nodes, groups, names := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, id := range nodeIDs {
		nodes[id] = true
	}
	for _, g := range r.Groups {
		if !ValidIconURL(g.Icon) {
			return errors.New("invalid group icon URL")
		}
		if !groupID.MatchString(g.ID) || groups[g.ID] || strings.TrimSpace(g.Name) != g.Name || len([]rune(g.Name)) < 1 || len([]rune(g.Name)) > 48 || strings.ContainsAny(g.Name, ",=[]#;/\"\\\u2028\u2029") || strings.ContainsFunc(g.Name, unicode.IsControl) || names[strings.ToLower(g.Name)] || reservedGroupName(g.Name) {
			return errors.New("invalid routing group")
		}
		if g.Type != "select" && g.Type != "url-test" && g.Type != "fallback" {
			return errors.New("invalid group mode")
		}
		groups[g.ID], names[strings.ToLower(g.Name)] = true, true
		seen := map[string]bool{}
		for _, id := range g.NodeIDs {
			if !nodes[id] || seen[id] || g.ID == "proxy" {
				return errors.New("invalid group membership")
			}
			seen[id] = true
		}
	}
	if !groups["proxy"] {
		return errors.New("missing default proxy group")
	}
	validTarget := func(target string) bool {
		return target == "proxy" || target == "direct" || target == "reject" || (strings.HasPrefix(target, "group:") && groups[strings.TrimPrefix(target, "group:")])
	}
	if !validTarget(r.Final) {
		return errors.New("invalid final target")
	}
	sources := map[string]bool{}
	for _, b := range p.Bindings {
		sources[b.Source] = true
		target := b.Target
		if override, exists := r.Targets[b.Source]; exists {
			target = override
		}
		if target != "disabled" && !validTarget(target) {
			return errors.New("invalid rule set target")
		}
	}
	for source := range r.Targets {
		if !sources[source] {
			return errors.New("unknown rule source")
		}
	}
	plain := append([]Rule(nil), rules...)
	for i := range plain {
		if !validTarget(plain[i].Target) {
			return errors.New("invalid custom rule target")
		}
		plain[i].Target = "proxy"
	}
	return ValidateRules(plain, "proxy")
}

const routingTestURL = "https://www.gstatic.com/generate_204"

// RenderRouting preserves the existing per-client protocol and certificate
// checks. Only policy groups and routing sections are replaced.
func RenderRouting(format, name string, nodes []Node, rules []Rule, final string, r *Routing, selectedIDs []string) ([]byte, error) {
	if err := ValidateRouting(r, selectedIDs, rules, final, format); err != nil {
		return nil, err
	}
	if r == nil {
		return Render(format, name, nodes, rules, final)
	}
	base, err := Render(format, name, nodes, nil, "proxy")
	if err != nil {
		return nil, err
	}
	ini := format == "surge" || format == "loon"
	labels := nodeLabels(nodes, ini)
	nodeNames, groupNames, used := map[string]string{}, map[string]string{}, map[string]bool{"direct": true, "reject": true, "reject-drop": true, "pass": true, "global": true}
	for i, n := range nodes {
		nodeNames[n.ID] = labels[i]
		used[strings.ToLower(labels[i])] = true
	}
	for _, g := range r.Groups {
		label := g.Name
		for suffix := 2; used[strings.ToLower(label)]; suffix++ {
			label = fmt.Sprintf("%s - %d", g.Name, suffix)
		}
		groupNames[g.ID], used[strings.ToLower(label)] = label, true
	}
	targetName := func(target string) string {
		if target == "proxy" {
			return groupNames["proxy"]
		}
		if strings.HasPrefix(target, "group:") {
			return groupNames[strings.TrimPrefix(target, "group:")]
		}
		return policy(target)
	}
	groups := []any{}
	var groupText strings.Builder
	for _, g := range r.Groups {
		members := []string{}
		if g.ID == "proxy" {
			members = append(members, labels...)
		} else {
			for _, id := range g.NodeIDs {
				if label, ok := nodeNames[id]; ok {
					members = append(members, label)
				}
			}
		}
		mode := g.Type
		// Empty category groups deliberately inherit the default group. A select
		// alias also avoids unsupported nested tests in clients such as Loon.
		if len(members) == 0 {
			members = []string{groupNames["proxy"]}
			mode = "select"
		}
		group := map[string]any{"name": groupNames[g.ID], "type": mode, "proxies": members}
		if g.Icon != "" && (format == "stash" || format == "mihomo") {
			group["icon"] = g.Icon
		}
		fmt.Fprintf(&groupText, "%s = %s, %s", groupNames[g.ID], mode, strings.Join(members, ", "))
		if mode != "select" {
			group["url"], group["interval"] = routingTestURL, 300
			if format == "surge" {
				groupText.WriteString(", interval=300")
			} else {
				fmt.Fprintf(&groupText, ", url=%s, interval=300", routingTestURL)
			}
		}
		groupText.WriteByte('\n')
		groups = append(groups, group)
	}
	clientRules := []string{}
	for _, rule := range rules {
		kind, value := "DOMAIN", strings.ToLower(rule.Value)
		if rule.Type == "domain_suffix" {
			kind = "DOMAIN-SUFFIX"
		}
		if rule.Type == "ip_cidr" {
			prefix, _ := netip.ParsePrefix(rule.Value)
			value = prefix.Masked().String()
			kind = "IP-CIDR"
			if prefix.Addr().Is6() {
				kind = "IP-CIDR6"
			}
		}
		line := kind + "," + value + "," + targetName(rule.Target)
		if rule.Type == "ip_cidr" {
			line += ",no-resolve"
		}
		clientRules = append(clientRules, line)
	}
	providers := map[string]any{}
	sources := map[string]ruleSource{}
	for _, source := range catalog.Sources {
		sources[source.ID] = source
	}
	preset, _ := presetFor(r.Preset)
	var remoteRules strings.Builder
	for _, b := range preset.Bindings {
		target := b.Target
		if override, ok := r.Targets[b.Source]; ok {
			target = override
		}
		if target == "disabled" {
			continue
		}
		source := sources[b.Source]
		provider := "xd-" + source.ID
		providers[provider] = map[string]any{"type": "http", "behavior": "classical", "format": "text", "url": source.URL, "path": "./rules/" + provider + ".list", "interval": 86400}
		ref := provider
		if ini {
			ref = source.URL
		}
		if format == "loon" {
			fmt.Fprintf(&remoteRules, "%s,policy=%s,enabled=true\n", ref, targetName(target))
		} else {
			clientRules = append(clientRules, "RULE-SET,"+ref+","+targetName(target))
		}
	}
	last := "MATCH"
	if ini {
		last = "FINAL"
	}
	clientRules = append(clientRules, last+","+targetName(r.Final))
	if ini {
		prefix, _, found := strings.Cut(string(base), "\n[Proxy Group]\n")
		if !found {
			return nil, errors.New("missing client policy section")
		}
		if format == "surge" {
			prefix = strings.Replace(prefix, "[Proxy]\n", "[General]\nproxy-test-url = "+routingTestURL+"\n\n[Proxy]\n", 1)
		}
		out := prefix + "\n[Proxy Group]\n" + groupText.String() + "\n[Rule]\n" + strings.Join(clientRules, "\n") + "\n"
		if format == "loon" {
			out += "\n[Remote Rule]\n" + remoteRules.String()
		}
		return []byte(out), nil
	}
	var config map[string]any
	if err := yaml.Unmarshal(base, &config); err != nil {
		return nil, err
	}
	config["proxy-groups"], config["rules"], config["rule-providers"] = groups, clientRules, providers
	return yaml.Marshal(config)
}

// Icon URLs are metadata fetched by clients, never by the control plane.
func ValidIconURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 2048 || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || !strings.Contains(host, ".") {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
	}
	return true
}
func IconWarnings(r *Routing, format string) []string {
	out := []string{}
	if r == nil {
		return out
	}
	has := false
	for _, g := range r.Groups {
		has = has || g.Icon != ""
	}
	if has {
		if format == "mihomo" {
			out = append(out, "icon_display_depends_on_dashboard")
		} else if format != "stash" {
			out = append(out, "group_icons_not_exported_for_format")
		}
	}
	return out
}
