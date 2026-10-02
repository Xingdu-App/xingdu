package subscription

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"

	"xingdu.app/xingdu/internal/protocol"
)

// Sing-box is an independent client adapter. Server engine JSON is never exported.
func singboxOutbound(n Node, tag string) (map[string]any, error) {
	s := n.Spec
	if s.RealityEnabled() && s.UsesXray() {
		return nil, &CompatibilityError{"singbox", s.Protocol, "当前 sing-box 客户端无法连接此 Xray REALITY 版本；请选择支持新版 REALITY 的 Xray 客户端"}
	}
	if !Supports("singbox", s.Protocol) {
		return nil, &CompatibilityError{"singbox", s.Protocol, "sing-box 导出尚未支持此协议"}
	}
	p := map[string]any{"type": s.Protocol, "tag": tag, "server": n.Server, "server_port": s.Port}
	switch s.Protocol {
	case "shadowsocks", "shadowsocks2022":
		p["type"], p["method"], p["password"], p["network"] = "shadowsocks", protocol.Cipher(s.Protocol), s.Credential, "tcp"
	case "trojan", "hysteria2", "anytls":
		p["password"] = s.Credential
	case "vless", "vmess":
		p["uuid"] = s.Credential
		if s.Protocol == "vmess" {
			p["security"], p["alter_id"] = "auto", 0
		}
	case "tuic":
		p["uuid"], p["password"], p["congestion_control"] = s.Credential, s.Password, "bbr"
	case "http", "socks", "mixed":
		p["username"], p["password"] = "xingdu", s.Credential
		if s.Protocol != "http" {
			p["type"], p["version"] = "socks", "5"
			if !s.UDPEnabled {
				p["network"] = "tcp"
			}
		}
	case "hysteria":
		p["auth_str"], p["up_mbps"], p["down_mbps"] = s.Credential, 100, 100
	}
	if s.TLSEnabled() {
		tls := map[string]any{"enabled": true, "server_name": s.ServerName, "insecure": false}
		if s.RealityEnabled() {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": "chrome"}
			tls["reality"] = map[string]any{"enabled": true, "public_key": s.RealityPublicKey, "short_id": s.RealityShortID}
		} else {
			block, _ := pem.Decode([]byte(s.Certificate))
			if block == nil {
				return nil, errors.New("missing client certificate")
			}
			// Trust only the selected leaf. Keeping the complete chain here could
			// turn an intermediate/root into a broader trust anchor.
			tls["certificate"] = strings.Split(strings.TrimSpace(string(pem.EncodeToMemory(block))), "\n")
		}
		if protocol.IsQUIC(s.Protocol) {
			tls["alpn"] = []string{"h3"}
			if s.Protocol == "hysteria" {
				tls["alpn"] = []string{"hysteria"}
			}
		}
		p["tls"] = tls
	}
	if v := s.V2Ray; v != nil {
		if v.Encryption || v.Network == "xhttp" || s.UsesXray() && v.Network == "http" {
			return nil, &CompatibilityError{"singbox", s.Protocol, "sing-box 导出不能保留此 Xray 传输或加密组合"}
		}
		if v.Flow != "" {
			p["flow"] = v.Flow
		}
		if v.PacketEncoding != "" {
			p["packet_encoding"] = v.PacketEncoding
		}
		if v.Network != "tcp" {
			tr := map[string]any{"type": v.Network}
			if v.Network == "grpc" {
				tr["service_name"] = v.ServiceName
			} else {
				tr["path"] = v.Path
			}
			if v.Host != "" {
				if v.Network == "http" {
					tr["host"] = []string{v.Host}
				} else if v.Network == "httpupgrade" {
					tr["host"] = v.Host
				} else {
					tr["headers"] = map[string]string{"Host": v.Host}
				}
			}
			p["transport"] = tr
		}
		if tls, ok := p["tls"].(map[string]any); ok {
			alpn := v.ALPN
			if len(alpn) == 0 {
				if v.Network == "ws" || v.Network == "httpupgrade" {
					alpn = []string{"http/1.1"}
				}
				if v.Network == "grpc" || v.Network == "http" {
					alpn = []string{"h2"}
				}
			}
			if len(alpn) > 0 {
				tls["alpn"] = alpn
			}
			if v.Fingerprint != "" {
				tls["utls"] = map[string]any{"enabled": true, "fingerprint": v.Fingerprint}
			}
		}
	}
	if q := s.QUIC; q != nil {
		if len(q.ALPN) > 0 {
			p["tls"].(map[string]any)["alpn"] = q.ALPN
		}
		if q.Congestion != "" {
			p["congestion_control"] = q.Congestion
		}
		if q.Salamander {
			p["obfs"] = map[string]any{"type": "salamander", "password": s.ObfsPassword}
		}
		if q.UpMbps > 0 {
			p["up_mbps"] = q.UpMbps
		}
		if q.DownMbps > 0 {
			p["down_mbps"] = q.DownMbps
		}
	}
	return p, nil
}

func renderSingbox(name string, nodes []Node, rules []Rule, final string) ([]byte, error) {
	if err := validateExport(name, nodes, rules, final); err != nil {
		return nil, err
	}
	outs := []any{map[string]any{"type": "direct", "tag": "direct"}}
	tags := []string{}
	for _, n := range nodes {
		tag := "node-" + n.ID
		tags = append(tags, tag)
		p, err := singboxOutbound(n, tag)
		if err != nil {
			return nil, err
		}
		p["tag"] = tag
		outs = append(outs, p)
	}
	outs = append(outs, map[string]any{"type": "selector", "tag": "proxy", "outbounds": tags, "default": tags[0]})
	routeRules := []any{}
	for _, r := range rules {
		p := map[string]any{}
		switch r.Type {
		case "domain":
			p["domain"] = []string{r.Value}
		case "domain_suffix":
			p["domain_suffix"] = []string{r.Value}
		case "ip_cidr":
			p["ip_cidr"] = []string{r.Value}
		}
		if r.Target == "reject" {
			p["action"] = "reject"
		} else {
			p["action"], p["outbound"] = "route", r.Target
		}
		routeRules = append(routeRules, p)
	}
	cfg := map[string]any{"log": map[string]any{"disabled": true}, "dns": map[string]any{"servers": []any{map[string]any{"type": "udp", "tag": "dns-public", "server": "9.9.9.9"}}}, "inbounds": []any{map[string]any{"type": "mixed", "tag": "local", "listen": "127.0.0.1", "listen_port": 7890}}, "outbounds": outs, "route": map[string]any{"rules": routeRules, "final": final, "default_domain_resolver": "dns-public"}}
	return json.MarshalIndent(cfg, "", "  ")
}
