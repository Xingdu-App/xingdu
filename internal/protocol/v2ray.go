package protocol

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// V2RayOptions is a bounded transport contract, independent of client YAML and
// engine JSON. Nil preserves the original TCP + TLS deployment contract.
type V2RayOptions struct {
	Reality        bool              `json:"reality,omitempty"`
	PacketEncoding string            `json:"packet_encoding,omitempty"`
	Engine         string            `json:"engine,omitempty"`
	Encryption     bool              `json:"encryption,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	Download       *XHTTPDownload    `json:"download,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Fingerprint    string            `json:"fingerprint,omitempty"`
	Network        string            `json:"network"`
	TLS            *bool             `json:"tls,omitempty"`
	Path           string            `json:"path,omitempty"`
	Host           string            `json:"host,omitempty"`
	ServiceName    string            `json:"service_name,omitempty"`
	ALPN           []string          `json:"alpn,omitempty"`
	Flow           string            `json:"flow,omitempty"`
}

func (in Input) TLSEnabled() bool {
	if in.V2Ray != nil && in.V2Ray.TLS != nil {
		return *in.V2Ray.TLS
	}
	return RequiresTLS(in.Protocol)
}

func (v V2RayOptions) Validate(kind string) error {
	if v.Reality && (kind != "vless" || v.Network != "tcp" || v.TLS != nil && !*v.TLS || v.Encryption || v.Download != nil || len(v.ALPN) > 0) {
		return errors.New("REALITY requires VLESS TCP TLS without certificate ALPN, encryption or split download")
	}
	if kind == "trojan" && v.TLS != nil && !*v.TLS {
		return errors.New("Trojan requires TLS")
	}
	if v.PacketEncoding != "" && v.PacketEncoding != "xudp" {
		return errors.New("invalid packet encoding")
	}
	if v.Engine == "xray" {
		return v.validateXray(kind)
	}
	if v.Engine != "" && v.Engine != "sing-box" {
		return errors.New("unsupported engine")
	}
	if v.Encryption || v.Download != nil || v.Mode != "" || len(v.Headers) > 0 || v.Fingerprint != "" {
		return errors.New("advanced options require Xray")
	}
	if kind != "vless" && kind != "vmess" && kind != "trojan" {
		return errors.New("transport options require VLESS, VMess or Trojan")
	}
	switch v.Network {
	case "tcp", "ws", "grpc", "http":
	default:
		return errors.New("unsupported transport; XHTTP and HTTPUpgrade require a verified runtime adapter")
	}
	if v.Path != "" {
		u, err := url.ParseRequestURI(v.Path)
		if err != nil || len(v.Path) > 2048 || !strings.HasPrefix(v.Path, "/") || strings.HasPrefix(v.Path, "//") || u.IsAbs() || u.Fragment != "" || strings.ContainsFunc(u.Path, unicode.IsControl) || strings.ContainsFunc(v.Path, unicode.IsControl) {
			return errors.New("invalid transport path")
		}
	}
	if v.Host != "" {
		if len(v.Host) > 253 || !dnsPattern.MatchString(v.Host) {
			return errors.New("invalid transport host")
		}
		for _, label := range strings.Split(v.Host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return errors.New("invalid transport host")
			}
		}
	}
	if len(v.ServiceName) > 128 || strings.ContainsAny(v.ServiceName, "/?#\\") || strings.ContainsFunc(v.ServiceName, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return errors.New("invalid gRPC service name")
	}
	switch v.Network {
	case "tcp":
		if v.Path != "" || v.Host != "" || v.ServiceName != "" {
			return errors.New("TCP does not accept transport paths, hosts or services")
		}
	case "grpc":
		if v.Path != "" || v.Host != "" || v.ServiceName == "" {
			return errors.New("gRPC requires a service name and does not accept path or host")
		}
	default:
		if v.ServiceName != "" || v.Path == "" {
			return errors.New("HTTP transports require a path and do not accept a gRPC service name")
		}
	}
	tls := v.TLS == nil || *v.TLS
	if !tls && len(v.ALPN) > 0 {
		return errors.New("ALPN requires TLS")
	}
	if len(v.ALPN) > 2 {
		return errors.New("too many ALPN values")
	}
	seen := map[string]bool{}
	for _, a := range v.ALPN {
		if (a != "h2" && a != "http/1.1") || seen[a] {
			return errors.New("unsupported or duplicate ALPN")
		}
		seen[a] = true
	}
	// These transports negotiate a specific HTTP version. Fail rather than
	// advertise an ALPN list that cannot carry the selected transport.
	if len(v.ALPN) > 0 {
		if (v.Network == "grpc" || v.Network == "http") && (len(v.ALPN) != 1 || v.ALPN[0] != "h2") {
			return errors.New("TLS HTTP/gRPC transport requires h2 ALPN")
		}
		if v.Network == "ws" && (len(v.ALPN) != 1 || v.ALPN[0] != "http/1.1") {
			return errors.New("WebSocket requires http/1.1 ALPN")
		}
	}
	if v.Flow != "" && (kind != "vless" || v.Flow != "xtls-rprx-vision" || v.Network != "tcp" || !tls) {
		return errors.New("Vision requires VLESS TCP with TLS in this runtime")
	}
	return nil
}

func (v V2RayOptions) transport() map[string]any {
	if v.Network == "tcp" {
		return nil
	}
	t := map[string]any{"type": v.Network}
	if v.Network == "grpc" {
		t["service_name"] = v.ServiceName
		return t
	}
	t["path"] = v.Path
	if v.Host != "" && v.Network == "http" {
		t["host"] = []string{v.Host}
	}
	return t
}
func (v V2RayOptions) alpn() []string {
	if len(v.ALPN) > 0 {
		return v.ALPN
	}
	switch v.Network {
	case "ws":
		return []string{"http/1.1"}
	case "grpc", "http":
		return []string{"h2"}
	}
	return nil
}
