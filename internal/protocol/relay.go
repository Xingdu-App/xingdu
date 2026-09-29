package protocol

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/netip"
	"strings"
)

// Peer contains only client credentials and public TLS material. The exit's
// private key and management identity never leave its own deployment.
type Peer struct {
	NodeID      string `json:"node_id"`
	Address     string `json:"address"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	ServerName  string `json:"server_name,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	Credential  string `json:"credential"`
	Password    string `json:"password,omitempty"`
}

func (p Peer) Validate() error {
	ip, err := netip.ParseAddr(p.Address)
	if err != nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || p.Port < 1 || p.Port > 65535 {
		return errors.New("invalid relay endpoint")
	}
	switch p.Protocol {
	case "shadowsocks", "shadowsocks2022", "trojan", "vless", "vmess", "hysteria2", "tuic", "anytls", "http", "socks", "mixed", "hysteria", "shadowtls", "snell", "snell6":
	default:
		return errors.New("unsupported relay protocol")
	}
	if p.Protocol == "vless" || p.Protocol == "vmess" || p.Protocol == "tuic" {
		if !uuidPattern.MatchString(p.Credential) {
			return errors.New("invalid relay UUID")
		}
	} else if p.Protocol == "shadowsocks2022" || p.Protocol == "shadowtls" {
		b, e := base64.StdEncoding.DecodeString(p.Credential)
		if e != nil || len(b) != 32 {
			return errors.New("invalid relay key")
		}
	} else if !validPassword(p.Credential) {
		return errors.New("invalid relay password")
	}
	if (p.Protocol == "tuic" || p.Protocol == "shadowtls") && !validPassword(p.Password) {
		return errors.New("invalid relay password")
	}
	if p.Protocol == "shadowtls" && (!ValidHandshakeHost(p.ServerName) || p.Certificate != "") {
		return errors.New("invalid handshake domain")
	}
	if RequiresTLS(p.Protocol) {
		b, _ := pem.Decode([]byte(p.Certificate))
		if b == nil {
			return errors.New("relay certificate required")
		}
		cert, e := x509.ParseCertificate(b.Bytes)
		if e != nil || cert.VerifyHostname(p.ServerName) != nil {
			return errors.New("invalid relay certificate")
		}
	}
	return nil
}
func (p Peer) outbound() map[string]any {
	out := map[string]any{"type": p.Protocol, "tag": "exit", "server": p.Address, "server_port": p.Port}
	if IsShadowsocks(p.Protocol) || p.Protocol == "shadowtls" {
		out["type"] = "shadowsocks"
		out["method"] = Cipher(p.Protocol)
		out["password"] = p.Credential
		if p.Protocol == "shadowtls" {
			out["detour"] = "exit-transport"
			delete(out, "server")
			delete(out, "server_port")
		}
		return out
	}
	if p.Protocol == "socks" || p.Protocol == "mixed" {
		out["type"], out["version"], out["username"], out["password"], out["network"] = "socks", "5", "xingdu", p.Credential, "tcp"
		return out
	}
	if p.Protocol == "snell" || p.Protocol == "snell6" {
		out["type"], out["version"], out["psk"], out["network"] = "snell", SnellClientVersion(p.Protocol), p.Credential, "tcp"
		return out
	}
	out["tls"] = map[string]any{"enabled": true, "server_name": p.ServerName, "certificate": strings.Split(strings.TrimSpace(p.Certificate), "\n")}
	if IsQUIC(p.Protocol) {
		out["tls"].(map[string]any)["alpn"] = []string{"h3"}
	}
	switch p.Protocol {
	case "vless", "vmess", "tuic":
		out["uuid"] = p.Credential
	default:
		out["password"] = p.Credential
	}
	if p.Protocol == "vmess" {
		out["security"] = "auto"
	}
	if p.Protocol == "tuic" {
		out["password"] = p.Password
		out["congestion_control"] = "bbr"
	}
	if p.Protocol == "http" {
		out["username"] = "xingdu"
	}
	if p.Protocol == "hysteria" {
		delete(out, "password")
		out["auth_str"], out["up_mbps"], out["down_mbps"] = p.Credential, 100, 100
		out["tls"].(map[string]any)["alpn"] = []string{"hysteria"}
	}
	return out
}
func (p Peer) Outbounds() []any {
	out := []any{p.outbound()}
	if p.Protocol == "shadowtls" {
		out = append(out, map[string]any{"type": "shadowtls", "tag": "exit-transport", "server": p.Address, "server_port": p.Port, "version": 3, "password": p.Password, "tls": map[string]any{"enabled": true, "server_name": p.ServerName, "utls": map[string]any{"enabled": true, "fingerprint": "chrome"}}})
	}
	return out
}
