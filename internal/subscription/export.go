// Package subscription renders a bounded client configuration, never arbitrary YAML.
package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net/netip"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

type Rule struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Target string `json:"target"`
}

type Node struct {
	ID, Name, Server string
	Spec             protocol.Spec
}

var ErrNoNodes = errors.New("subscription has no available nodes")

func domain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}

// ValidateRules accepts only structured, ordered rules; commas, wildcards and
// user-supplied policies cannot escape into the client's rule grammar.
func ValidateRules(rules []Rule, final string) error {
	if final != "proxy" && final != "direct" {
		return errors.New("invalid final policy")
	}
	if len(rules) > 100 {
		return errors.New("too many rules")
	}
	for _, rule := range rules {
		if rule.Target != "proxy" && rule.Target != "direct" && rule.Target != "reject" {
			return errors.New("invalid rule target")
		}
		switch rule.Type {
		case "domain", "domain_suffix":
			if !domain(rule.Value) {
				return errors.New("invalid rule domain")
			}
		case "ip_cidr":
			if _, err := netip.ParsePrefix(rule.Value); err != nil {
				return errors.New("invalid rule CIDR")
			}
		default:
			return errors.New("unsupported rule type")
		}
	}
	return nil
}

func policy(target string) string {
	switch target {
	case "direct":
		return "DIRECT"
	case "reject":
		return "REJECT"
	default:
		return "Xingdu"
	}
}

// RenderClash produces a complete Mihomo configuration. Other Clash-family
// clients may support different fields/protocols and are not implied compatible.
// TLS is pinned to each deployment's leaf certificate; skip-cert-verify is never
// enabled. Server TLS private keys are never included in the client document.
func RenderClash(name string, nodes []Node, rules []Rule, final string) ([]byte, error) {
	return Render("mihomo", name, nodes, rules, final)
}

func validateExport(name string, nodes []Node, rules []Rule, final string) error {
	if err := ValidateRules(rules, final); err != nil {
		return err
	}
	if len(nodes) == 0 {
		return ErrNoNodes
	}
	if len(nodes) > 100 {
		return errors.New("too many nodes")
	}
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 80 || strings.ContainsFunc(name, unicode.IsControl) {
		return errors.New("invalid subscription name")
	}
	seen := map[string]bool{}
	for _, node := range nodes {
		if !id.Valid("node", node.ID) || seen[node.ID] {
			return errors.New("invalid or duplicate node ID")
		}
		seen[node.ID] = true
		if ip, err := netip.ParseAddr(node.Server); err != nil {
			if !domain(node.Server) {
				return errors.New("invalid node server")
			}
		} else if ip.Zone() != "" {
			return errors.New("invalid node server")
		}
		if strings.TrimSpace(node.Name) == "" || len([]rune(node.Name)) > 80 || strings.ContainsFunc(node.Name, unicode.IsControl) {
			return errors.New("invalid node name")
		}
		if err := protocol.ValidateSpec(node.Spec); err != nil {
			return errors.New("invalid node configuration")
		}
	}
	return nil
}

// Render uses explicit client adapters: Stash and Mihomo do not share all fields.
func Render(format, name string, nodes []Node, rules []Rule, final string) ([]byte, error) {
	if !ValidFormat(format) {
		return nil, errors.New("unsupported client format")
	}

	if format == "singbox" {
		return renderSingbox(name, nodes, rules, final)
	}
	if format == "uri" || format == "base64" {
		return renderLinks(format, name, nodes, rules, final)
	}
	if format != "stash" && format != "mihomo" {
		return renderOther(format, name, nodes, rules, final)
	}
	if err := validateExport(name, nodes, rules, final); err != nil {
		return nil, err
	}
	proxies := make([]map[string]any, 0, len(nodes))
	names := make([]string, 0, len(nodes))
	exportNames := nodeLabels(nodes, false)
	for i, node := range nodes {
		if !Supports(format, node.Spec.Protocol) {
			return nil, &CompatibilityError{format, node.Spec.Protocol, "所选客户端格式不支持该协议"}
		}
		s := node.Spec
		label := exportNames[i]
		p := map[string]any{"name": label, "type": s.Protocol, "server": node.Server, "port": s.Port, "udp": true}
		if s.TLSEnabled() && !s.RealityEnabled() {
			cert, _ := pem.Decode([]byte(s.Certificate))
			if cert == nil || cert.Type != "CERTIFICATE" {
				return nil, errors.New("invalid node certificate")
			}
			digest := sha256.Sum256(cert.Bytes)
			p["skip-cert-verify"] = false
			p["fingerprint"] = hex.EncodeToString(digest[:])
		}
		switch s.Protocol {
		case "shadowsocks", "shadowsocks2022", "shadowtls":
			p["type"] = "ss"
			p["udp"] = false
			p["cipher"] = protocol.Cipher(s.Protocol)
			p["password"] = s.Credential
			if s.Protocol == "shadowtls" {
				p["plugin"] = "shadow-tls"
				p["plugin-opts"] = map[string]any{"host": s.ServerName, "password": s.Password, "version": 3}
				if format == "stash" {
					p["plugin-opts"].(map[string]any)["skip-cert-verify"] = false
				} else {
					p["client-fingerprint"] = "chrome"
				}
			}
		case "socks", "mixed":
			p["type"], p["username"], p["password"], p["udp"] = "socks5", "xingdu", s.Credential, s.UDPEnabled
		case "wireguard":
			w := s.WireGuardClient()
			p["private-key"] = w["private_key"]
			p["public-key"] = w["public_key"]
			if format == "stash" {
				p["preshared-key"] = w["pre_shared_key"]
			} else {
				p["pre-shared-key"] = w["pre_shared_key"]
			}
			p["ip"] = w["ip"]
			p["mtu"] = w["mtu"]
			p["keepalive"] = w["keepalive"]
			if len(s.WireGuard.Reserved) > 0 {
				p["reserved"] = s.WireGuard.Reserved
			}
		case "snell":
			p["type"], p["psk"], p["version"], p["udp"] = "snell", s.Credential, 4, false
		case "hysteria":
			p["auth-str"], p["sni"], p["alpn"] = s.Credential, s.ServerName, []string{"hysteria"}
			if format == "stash" {
				p["up-speed"], p["down-speed"], p["protocol"] = 100, 100, "udp"
			} else {
				p["up"], p["down"] = "100 Mbps", "100 Mbps"
			}
		case "trusttunnel":
			p["username"], p["password"], p["sni"] = "xingdu", s.Credential, s.ServerName
			p["quic"], p["alpn"] = false, []string{"h2"}
		case "anytls", "http":
			p["password"] = s.Credential
			p["sni"] = s.ServerName
			p["udp"] = false
			if s.Protocol == "http" {
				p["username"] = "xingdu"
				p["tls"] = true
			}
		case "trojan":
			p["password"] = s.Credential
			p["sni"] = s.ServerName
		case "vless", "vmess":
			p["uuid"] = s.Credential
			p["tls"] = true
			p["servername"] = s.ServerName
			p["network"] = "tcp"
			if s.Protocol == "vmess" {
				p["alterId"] = 0
				p["cipher"] = "auto"
			}
		case "hysteria2":
			p["password"] = s.Credential
			p["sni"] = s.ServerName
			p["alpn"] = []string{"h3"}
		case "tuic":
			p["uuid"] = s.Credential
			p["password"] = s.Password
			p["sni"] = s.ServerName
			p["alpn"] = []string{"h3"}
			p["congestion-controller"] = "bbr"
			p["udp-relay-mode"] = "native"
		}
		if format == "stash" {
			if s.TLSEnabled() && !s.RealityEnabled() {
				p["server-cert-fingerprint"] = p["fingerprint"]
				delete(p, "fingerprint")
				p["sni"] = s.ServerName
				delete(p, "servername")
			}
			if s.Protocol == "vmess" {
				delete(p, "network")
			} // Plain TCP is the default.
			if s.Protocol == "hysteria2" {
				p["auth"] = s.Credential
				delete(p, "password")
			}
			if s.Protocol == "tuic" {
				p["version"] = 5
				delete(p, "congestion-controller")
				delete(p, "udp-relay-mode")
			}
		}
		if q := s.QUIC; q != nil {
			if format == "stash" && s.Protocol == "tuic" && q.Congestion != "" && q.Congestion != "bbr" {
				return nil, &CompatibilityError{format, s.Protocol, "当前 Stash TUIC 使用 BBR，无法保留指定拥塞控制器"}
			}
			if len(q.ALPN) > 0 {
				p["alpn"] = q.ALPN
			}
			if q.Salamander {
				p["obfs"] = "salamander"
				p["obfs-password"] = s.ObfsPassword
			}
			if q.Congestion != "" {
				p["congestion-controller"] = q.Congestion
			}
			if q.UpMbps > 0 {
				if format == "stash" {
					p["up-speed"] = q.UpMbps
				} else {
					p["up"] = q.UpMbps
				}
			}
			if q.DownMbps > 0 {
				if format == "stash" {
					p["down-speed"] = q.DownMbps
				} else {
					p["down"] = q.DownMbps
				}
			}
		}
		if err := applyV2Ray(format, s, p); err != nil {
			return nil, err
		}
		proxies = append(proxies, p)
		names = append(names, label)
	}
	clientRules := make([]string, 0, len(rules)+1)
	for _, rule := range rules {
		kind, value := "DOMAIN", strings.ToLower(rule.Value)
		if rule.Type == "domain_suffix" {
			kind = "DOMAIN-SUFFIX"
		}
		if rule.Type == "ip_cidr" {
			prefix, _ := netip.ParsePrefix(rule.Value)
			kind, value = "IP-CIDR", prefix.Masked().String()
			if prefix.Addr().Is6() {
				kind = "IP-CIDR6"
			}
		}
		clientRules = append(clientRules, kind+","+value+","+policy(rule.Target))
	}
	clientRules = append(clientRules, "MATCH,"+policy(final))
	config := map[string]any{"mixed-port": 7890, "allow-lan": false, "mode": "rule", "log-level": "warning", "proxies": proxies, "proxy-groups": []any{map[string]any{"name": "Xingdu", "type": "select", "proxies": names}}, "rules": clientRules}
	if format == "stash" {
		delete(config, "mixed-port")
	}
	return yaml.Marshal(config)
}
