package protocol

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type XHTTPDownload struct {
	TLS  bool     `json:"tls"`
	ALPN []string `json:"alpn,omitempty"`
}

func (s Spec) UsesXray() bool {
	return (s.V2Ray != nil && s.V2Ray.Engine == "xray") || s.Protocol == "socks" && s.UDPEnabled
}
func (in Input) NeedsCertificate() bool {
	return in.TLSEnabled() || in.V2Ray != nil && in.V2Ray.Download != nil && in.V2Ray.Download.TLS
}
func (s Spec) ClientEncryption() string {
	if s.EncryptionPublicKey == "" {
		return "none"
	}
	return "mlkem768x25519plus.native.0rtt." + s.EncryptionPublicKey
}
func newEncryptionKeys(s *Spec) error {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	s.EncryptionKey = base64.RawURLEncoding.EncodeToString(key.Bytes())
	s.EncryptionPublicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	return nil
}
func validateEncryptionKeys(s Spec) error {
	enabled := s.V2Ray != nil && s.V2Ray.Encryption
	if !enabled {
		if s.EncryptionKey != "" || s.EncryptionPublicKey != "" {
			return errors.New("unexpected encryption key")
		}
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s.EncryptionKey)
	if err != nil {
		return errors.New("invalid encryption key")
	}
	key, err := ecdh.X25519().NewPrivateKey(b)
	if err != nil || base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()) != s.EncryptionPublicKey {
		return errors.New("invalid encryption key pair")
	}
	return nil
}
func (v V2RayOptions) validateXray(kind string) error {
	if kind != "vless" && kind != "vmess" {
		return errors.New("Xray options require VLESS or VMess")
	}
	switch v.Network {
	case "tcp", "ws", "grpc", "http", "httpupgrade", "xhttp":
	default:
		return errors.New("unsupported Xray transport")
	}
	// Reuse bounded path/host/service validation without the sing-box-only ALPN
	// and Vision restrictions. Xray can run Vision above VLESS Encryption.
	base := v
	base.Engine = ""
	base.Flow = ""
	base.ALPN = nil
	base.Encryption = false
	base.Download = nil
	base.Mode = ""
	base.Headers = nil
	base.Fingerprint = ""
	if base.Network == "xhttp" || base.Network == "httpupgrade" {
		base.Network = "ws"
	}
	if err := base.Validate(kind); err != nil {
		return err
	}
	if (v.TLS == nil || *v.TLS) && len(v.ALPN) > 0 {
		required := ""
		if v.Network == "ws" || v.Network == "httpupgrade" {
			required = "http/1.1"
		}
		if v.Network == "grpc" {
			required = "h2"
		}
		found := required == ""
		for _, alpn := range v.ALPN {
			if alpn == required {
				found = true
			}
		}
		if !found {
			return errors.New("ALPN excludes required transport protocol")
		}
	}
	if v.Flow != "" && (kind != "vless" || v.Flow != "xtls-rprx-vision" || (!v.Encryption && (v.Network != "tcp" || v.TLS != nil && !*v.TLS))) {
		return errors.New("Vision needs TCP TLS or VLESS Encryption")
	}
	if v.Encryption && kind != "vless" {
		return errors.New("VLESS Encryption requires VLESS")
	}
	if v.Download != nil && v.Network != "xhttp" {
		return errors.New("split download requires XHTTP")
	}
	if v.Mode != "" && v.Network != "xhttp" {
		return errors.New("mode requires XHTTP")
	}
	switch v.Mode {
	case "", "auto", "packet-up", "stream-up", "stream-one":
	default:
		return errors.New("invalid XHTTP mode")
	}
	if v.Download != nil && v.Mode == "stream-one" {
		return errors.New("stream-one cannot split download")
	}
	for _, list := range [][]string{v.ALPN, func() []string {
		if v.Download != nil {
			return v.Download.ALPN
		}
		return nil
	}()} {
		if len(list) > 3 {
			return errors.New("too many ALPN values")
		}
		seen := map[string]bool{}
		for _, a := range list {
			if seen[a] || (a != "h2" && a != "http/1.1" && a != "h3") {
				return errors.New("invalid ALPN")
			}
			seen[a] = true
			if a == "h3" && v.Network != "xhttp" {
				return errors.New("h3 requires XHTTP")
			}
		}
	}
	if len(v.Headers) > 8 {
		return errors.New("too many headers")
	}
	for k, val := range v.Headers {
		if k != "User-Agent" || len(val) > 1024 || strings.ContainsAny(val, "\r\n\x00") {
			return errors.New("only bounded User-Agent is supported")
		}
	}
	switch v.Fingerprint {
	case "", "chrome", "firefox", "safari", "edge":
	default:
		return errors.New("unsupported client fingerprint")
	}
	return nil
}

type XrayConfig struct {
	Runtime  string                `json:"runtime"`
	Spec     Spec                  `json:"spec"`
	Inbounds []TrustTunnelListener `json:"inbounds"`
}

func RenderXray(s Spec) ([]byte, error) {
	if !s.UsesXray() {
		return nil, errors.New("invalid Xray spec")
	}
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	return json.Marshal(XrayConfig{"xray", s, []TrustTunnelListener{{s.Port}}})
}

// XrayServer generates engine JSON. socket is a launcher-created private Unix
// socket for XHTTP, never a tenant-selected file or destination.
func XrayServer(s Spec, socket string) ([]byte, error) {
	if !s.UsesXray() {
		return nil, errors.New("invalid Xray spec")
	}
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	v := s.V2Ray
	if v == nil {
		v = &V2RayOptions{Network: "tcp"}
	}
	user := map[string]any{"id": s.Credential, "email": "xingdu"}
	if v.Flow != "" {
		user["flow"] = v.Flow
	}
	settings := map[string]any{"clients": []any{user}}
	if s.Protocol == "vless" {
		settings["decryption"] = "none"
		if v.Encryption {
			settings["decryption"] = "mlkem768x25519plus.native.600s." + s.EncryptionKey
		}
	}
	if s.Protocol == "socks" {
		settings = map[string]any{"auth": "password", "accounts": []any{map[string]any{"user": "xingdu", "pass": s.Credential}}, "udp": true, "ip": "0.0.0.0"}
	}
	stream := xrayTransport(v)
	if s.TLSEnabled() {
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{"alpn": v.ALPN, "certificates": []any{map[string]any{"certificate": strings.Split(strings.TrimSpace(s.Certificate), "\n"), "key": strings.Split(strings.TrimSpace(s.PrivateKey), "\n")}}}
	}
	inbound := map[string]any{"tag": "xingdu-in", "listen": "0.0.0.0", "port": s.Port, "protocol": s.Protocol, "settings": settings, "streamSettings": stream}
	if v.Network == "xhttp" {
		if socket == "" {
			return nil, errors.New("XHTTP requires a private listener")
		}
		inbound["listen"] = socket
		inbound["port"] = 0
		delete(stream, "tlsSettings")
		stream["security"] = "none"
	}
	cfg := map[string]any{"log": map[string]any{"loglevel": "none"}, "inbounds": []any{inbound}, "outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom", "settings": map[string]any{"domainStrategy": "UseIP"}}, map[string]any{"tag": "blocked", "protocol": "blackhole"}}, "routing": map[string]any{"domainStrategy": "IPOnDemand", "rules": []any{map[string]any{"type": "field", "ip": []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "168.63.129.16/32", "224.0.0.0/4", "240.0.0.0/4", "::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8"}, "outboundTag": "blocked"}}}}
	return json.Marshal(cfg)
}
func xrayTransport(v *V2RayOptions) map[string]any {
	t := map[string]any{"network": v.Network, "security": "none"}
	switch v.Network {
	case "tcp":
		t["network"] = "raw"
	case "http":
		t["network"] = "raw"
		t["rawSettings"] = map[string]any{"header": map[string]any{"type": "http", "request": map[string]any{"path": []string{v.Path}, "headers": map[string]any{"Host": []string{v.Host}}}}}
	case "ws":
		t["wsSettings"] = map[string]any{"path": v.Path, "host": v.Host}
	case "grpc":
		t["grpcSettings"] = map[string]any{"serviceName": v.ServiceName}
	case "httpupgrade":
		t["httpupgradeSettings"] = map[string]any{"path": v.Path, "host": v.Host}
	case "xhttp":
		t["xhttpSettings"] = map[string]any{"path": v.Path, "host": v.Host, "mode": v.Mode, "headers": v.Headers}
	}
	return t
}

// XrayClient is a reference-client configuration for acceptance. It only
// projects public certificate material and client credentials from a valid spec.
func XrayClient(s Spec, server string, port int) ([]byte, error) {
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	v := s.V2Ray
	if v == nil {
		return nil, errors.New("missing transport")
	}
	stream := xrayTransport(v)
	tlsSettings := func(alpn []string) map[string]any {
		return map[string]any{"serverName": s.ServerName, "alpn": alpn, "fingerprint": v.Fingerprint, "certificates": []any{map[string]any{"certificate": strings.Split(strings.TrimSpace(s.Certificate), "\n"), "usage": "verify"}}}
	}
	if s.TLSEnabled() {
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings(v.ALPN)
	}
	if v.Download != nil {
		d := xrayTransport(v)
		d["address"] = server
		d["port"] = s.Port
		if v.Download.TLS {
			d["security"] = "tls"
			d["tlsSettings"] = tlsSettings(v.Download.ALPN)
		}
		stream["xhttpSettings"].(map[string]any)["downloadSettings"] = d
	}
	u := map[string]any{"id": s.Credential}
	if s.Protocol == "vless" {
		u["encryption"] = s.ClientEncryption()
		u["flow"] = v.Flow
	} else {
		u["security"] = "auto"
	}
	cfg := map[string]any{"log": map[string]any{"loglevel": "warning"}, "inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": port, "protocol": "socks", "settings": map[string]any{"udp": true}}}, "outbounds": []any{map[string]any{"protocol": s.Protocol, "settings": map[string]any{"vnext": []any{map[string]any{"address": server, "port": s.Port, "users": []any{u}}}}, "streamSettings": stream}}}
	if v.PacketEncoding == "xudp" {
		cfg["outbounds"].([]any)[0].(map[string]any)["mux"] = map[string]any{"enabled": true, "concurrency": -1, "xudpConcurrency": 8, "xudpProxyUDP443": "allow"}
	}
	return json.Marshal(cfg)
}
