package subscription

import "xingdu.app/xingdu/internal/protocol"

func applyV2Ray(format string, s protocol.Spec, p map[string]any) error {
	v := s.V2Ray
	if v == nil {
		return nil
	}
	// Keep client capability checks separate from server-side validation. These
	// combinations cannot be represented by the currently supported adapters.
	if format != "stash" && format != "mihomo" {
		return &CompatibilityError{format, s.Protocol, "此客户端导出尚未支持自定义传输配置"}
	}
	if format == "mihomo" && v.Network == "xhttp" {
		return &CompatibilityError{format, s.Protocol, "此适配器尚未验证 Mihomo XHTTP；请使用对应参考客户端"}
	}
	if format == "stash" && (v.Network == "httpupgrade" || v.Flow != "" && (v.Network == "grpc" || v.Network == "http") || v.Network == "xhttp" && (hasNonH2(v.ALPN) || v.Download != nil && hasNonH2(v.Download.ALPN))) {
		return &CompatibilityError{format, s.Protocol, "当前 Stash Core 不支持此传输组合；请查看兼容性矩阵"}
	}
	if format == "stash" && (v.Network == "grpc" && !s.TLSEnabled()) {
		return &CompatibilityError{format, s.Protocol, "当前 Stash 适配器不支持 无 TLS 的 gRPC"}
	}
	if v.Fingerprint != "" {
		p["client-fingerprint"] = v.Fingerprint
	}
	if s.Protocol == "vless" && v.Encryption {
		p["encryption"] = s.ClientEncryption()
	}
	p["tls"] = s.TLSEnabled()
	p["network"] = v.Network
	if v.PacketEncoding != "" {
		p["packet-encoding"] = v.PacketEncoding
	}
	if v.Flow != "" {
		p["flow"] = v.Flow
	}
	if len(v.ALPN) > 0 {
		p["alpn"] = v.ALPN
	}
	switch v.Network {
	case "ws", "httpupgrade":
		opts := map[string]any{"path": v.Path}
		if v.Host != "" {
			opts["headers"] = map[string]string{"Host": v.Host}
		}
		p["ws-opts"] = opts
		if v.Network == "httpupgrade" {
			p["network"] = "ws"
			opts["v2ray-http-upgrade"] = true
		}
	case "grpc":
		p["grpc-opts"] = map[string]any{"grpc-service-name": v.ServiceName}
	case "xhttp":
		opts := map[string]any{"host": v.Host, "path": v.Path, "mode": func() string {
			if v.Mode == "" {
				return "auto"
			}
			return v.Mode
		}(), "headers": v.Headers}
		if v.Download != nil {
			opts["download-settings"] = map[string]any{"server": p["server"], "port": s.Port, "network": "xhttp", "tls": v.Download.TLS, "alpn": v.Download.ALPN, "servername": s.ServerName, "host": v.Host, "path": v.Path, "mode": v.Mode, "headers": v.Headers, "client-fingerprint": v.Fingerprint}
		}
		if v.Download != nil && v.Download.TLS {
			d := opts["download-settings"].(map[string]any)
			d["skip-cert-verify"] = false
			field := "fingerprint"
			if format == "stash" {
				field = "server-cert-fingerprint"
			}
			d[field] = certificatePin(Node{Spec: s})
		}
		p["xhttp-opts"] = opts
	case "http":
		if s.TLSEnabled() && !s.UsesXray() {
			p["network"] = "h2"
			opts := map[string]any{"path": v.Path}
			if v.Host != "" {
				opts["host"] = []string{v.Host}
			}
			p["h2-opts"] = opts
		} else {
			opts := map[string]any{"path": []string{v.Path}, "method": "GET"}
			if v.Host != "" {
				opts["headers"] = map[string]any{"Host": []string{v.Host}}
			}
			p["http-opts"] = opts
		}
	}
	if !s.TLSEnabled() {
		delete(p, "servername")
		delete(p, "sni")
	}
	return nil
}

func hasNonH2(alpn []string) bool {
	for _, a := range alpn {
		if a != "h2" {
			return true
		}
	}
	return false
}
