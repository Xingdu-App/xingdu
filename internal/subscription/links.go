package subscription

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"

	"xingdu.app/xingdu/internal/protocol"
)

func LinkOnly(format string) bool {
	return format == "uri" || format == "base64" || format == "hysteria2_uri"
}

func renderLinks(format, name string, nodes []Node, rules []Rule, final string) ([]byte, error) {
	if err := validateExport(name, nodes, rules, final); err != nil {
		return nil, err
	}
	if len(rules) > 0 || final != "proxy" {
		return nil, &CompatibilityError{format, "", "分享链接不包含分流规则；请清空规则并将默认连接设为使用节点"}
	}
	labels := nodeLabels(nodes, false)
	var out strings.Builder
	for i, n := range nodes {
		s := n.Spec
		if !Supports(format, s.Protocol) {
			return nil, &CompatibilityError{format, s.Protocol, "通用链接导出尚未支持此协议"}
		}
		if s.Relay != nil {
			return nil, &CompatibilityError{format, s.Protocol, "通用链接不能保留中转配置"}
		}
		if s.TLSEnabled() && !s.RealityEnabled() && s.Protocol != "hysteria2" && verifyPublicCertificate(n) != nil {
			return nil, &CompatibilityError{format, s.Protocol, "通用链接需要系统信任的完整证书链；私有 CA 请使用完整配置"}
		}
		v := s.V2Ray
		if v != nil && (v.Download != nil || len(v.Headers) > 0 || v.Network == "xhttp" || v.Network == "http" || s.Protocol == "vmess" && (v.PacketEncoding != "" || v.Fingerprint != "")) {
			return nil, &CompatibilityError{format, s.Protocol, "通用链接不能保留此高级传输配置，请选择完整配置"}
		}
		q := url.Values{}
		if s.TLSEnabled() {
			q.Set("security", "tls")
			q.Set("sni", s.ServerName)
		} else {
			q.Set("security", "none")
		}
		if v != nil {
			q.Set("type", v.Network)
			if v.Path != "" {
				q.Set("path", v.Path)
			}
			if v.Host != "" {
				q.Set("host", v.Host)
			}
			if v.ServiceName != "" {
				q.Set("serviceName", v.ServiceName)
			}
			if len(v.ALPN) > 0 {
				q.Set("alpn", strings.Join(v.ALPN, ","))
			}
			if v.Flow != "" {
				q.Set("flow", v.Flow)
			}
			if v.Fingerprint != "" {
				q.Set("fp", v.Fingerprint)
			}
			if v.PacketEncoding != "" {
				q.Set("packetEncoding", v.PacketEncoding)
			}
		} else {
			q.Set("type", "tcp")
		}
		if s.RealityEnabled() {
			q.Set("security", "reality")
			q.Set("pbk", s.RealityPublicKey)
			q.Set("sid", s.RealityShortID)
			q.Set("fp", "chrome")
		}
		u := url.URL{Scheme: s.Protocol, User: url.User(s.Credential), Host: net.JoinHostPort(n.Server, strconv.Itoa(s.Port)), RawQuery: q.Encode(), Fragment: labels[i]}
		switch s.Protocol {
		case "vless":
			q.Set("encryption", s.ClientEncryption())
			u.RawQuery = q.Encode()
		case "shadowsocks", "shadowsocks2022":
			u.Scheme = "ss"
			u.User = url.User(base64.RawURLEncoding.EncodeToString([]byte(protocol.Cipher(s.Protocol) + ":" + s.Credential)))
			u.RawQuery = ""
		case "vmess":
			netType, path, host, alpn := "tcp", "", "", ""
			if v != nil {
				netType, path, host, alpn = v.Network, v.Path, v.Host, strings.Join(v.ALPN, ",")
				if netType == "grpc" {
					path = v.ServiceName
				}
			}
			tls := ""
			if s.TLSEnabled() {
				tls = "tls"
			}
			data, _ := json.Marshal(map[string]any{"v": "2", "ps": labels[i], "add": n.Server, "port": strconv.Itoa(s.Port), "id": s.Credential, "aid": "0", "scy": "auto", "net": netType, "type": "none", "host": host, "path": path, "tls": tls, "sni": s.ServerName, "alpn": alpn})
			out.WriteString("vmess://" + base64.StdEncoding.EncodeToString(data) + "\n")
			continue
		case "hysteria2":
			u.Path = "/"
			q = url.Values{"sni": {s.ServerName}, "insecure": {"0"}, "pinSHA256": {certificatePin(n)}}
			if s.QUIC != nil {
				if s.QUIC.Salamander {
					q.Set("obfs", "salamander")
					q.Set("obfs-password", s.ObfsPassword)
				}
				if len(s.QUIC.ALPN) > 0 {
					q.Set("alpn", strings.Join(s.QUIC.ALPN, ","))
				}
			}
			u.RawQuery = q.Encode()
		case "tuic":
			u.User = url.UserPassword(s.Credential, s.Password)
			q = url.Values{"sni": {s.ServerName}, "allow_insecure": {"0"}, "congestion_control": {"bbr"}, "alpn": {"h3"}, "udp_relay_mode": {"native"}}
			if s.QUIC != nil {
				if s.QUIC.Congestion != "" {
					q.Set("congestion_control", s.QUIC.Congestion)
				}
				if len(s.QUIC.ALPN) > 0 {
					q.Set("alpn", strings.Join(s.QUIC.ALPN, ","))
				}
			}
			u.RawQuery = q.Encode()
		}
		out.WriteString(u.String() + "\n")
	}
	data := []byte(out.String())
	if format == "base64" {
		return []byte(base64.StdEncoding.EncodeToString(data)), nil
	}
	return data, nil
}
