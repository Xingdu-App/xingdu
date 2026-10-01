package protocol

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

type QUICOptions struct {
	ALPN       []string `json:"alpn,omitempty"`
	Congestion string   `json:"congestion,omitempty"`
	Salamander bool     `json:"salamander,omitempty"`
	UpMbps     int      `json:"up_mbps,omitempty"`
	DownMbps   int      `json:"down_mbps,omitempty"`
}
type WireGuardOptions struct {
	MTU       int   `json:"mtu"`
	Keepalive int   `json:"keepalive,omitempty"`
	Reserved  []int `json:"reserved,omitempty"`
	Preshared bool  `json:"preshared,omitempty"`
}
type WireGuardKeys struct {
	ServerPrivate string `json:"server_private"`
	ClientPrivate string `json:"client_private"`
	Preshared     string `json:"preshared,omitempty"`
}

func (in Input) validateExtra() error {
	if in.UDPEnabled && in.Protocol != "socks" {
		return errors.New("UDP option only applies to SOCKS")
	}
	if q := in.QUIC; q != nil {
		if !IsQUIC(in.Protocol) {
			return errors.New("QUIC options require a QUIC protocol")
		}
		if len(q.ALPN) > 2 {
			return errors.New("too many ALPN values")
		}
		for _, a := range q.ALPN {
			if a != "h3" && a != "hysteria" {
				return errors.New("unsupported QUIC ALPN")
			}
		}
		if q.Congestion != "" && (in.Protocol != "tuic" || (q.Congestion != "bbr" && q.Congestion != "cubic" && q.Congestion != "new_reno")) {
			return errors.New("invalid congestion controller")
		}
		if q.Salamander && in.Protocol != "hysteria2" {
			return errors.New("Salamander requires Hysteria 2")
		}
		if q.UpMbps < 0 || q.DownMbps < 0 || q.UpMbps > 100000 || q.DownMbps > 100000 || (in.Protocol != "hysteria" && (q.UpMbps != 0 || q.DownMbps != 0)) {
			return errors.New("invalid bandwidth")
		}
	}
	if w := in.WireGuard; w != nil {
		if in.Protocol != "wireguard" || w.MTU < 576 || w.MTU > 9000 || w.Keepalive < 0 || w.Keepalive > 65535 || (len(w.Reserved) != 0 && len(w.Reserved) != 3) {
			return errors.New("invalid WireGuard options")
		}
		for _, b := range w.Reserved {
			if b < 0 || b > 255 {
				return errors.New("invalid WireGuard reserved byte")
			}
		}
	} else if in.Protocol == "wireguard" {
		return errors.New("WireGuard options required")
	}
	return nil
}
func wgPublic(private string) string {
	b, e := base64.StdEncoding.DecodeString(private)
	if e != nil {
		return ""
	}
	k, e := ecdh.X25519().NewPrivateKey(b)
	if e != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(k.PublicKey().Bytes())
}
func newWGKeys() (*WireGuardKeys, error) {
	a, e := ecdh.X25519().GenerateKey(rand.Reader)
	if e != nil {
		return nil, e
	}
	b, e := ecdh.X25519().GenerateKey(rand.Reader)
	if e != nil {
		return nil, e
	}
	return &WireGuardKeys{ServerPrivate: base64.StdEncoding.EncodeToString(a.Bytes()), ClientPrivate: base64.StdEncoding.EncodeToString(b.Bytes())}, nil
}
func (s Spec) WireGuardClient() map[string]any {
	if s.WireGuardKeys == nil {
		return nil
	}
	return map[string]any{"private_key": s.WireGuardKeys.ClientPrivate, "public_key": wgPublic(s.WireGuardKeys.ServerPrivate), "pre_shared_key": s.WireGuardKeys.Preshared, "ip": "10.77.0.2", "mtu": s.WireGuard.MTU, "keepalive": s.WireGuard.Keepalive, "reserved": s.WireGuard.Reserved}
}
func (s Spec) applyExtra(inbound, tls, cfg map[string]any) {
	if q := s.QUIC; q != nil {
		if len(q.ALPN) > 0 {
			tls["alpn"] = q.ALPN
		}
		if q.Congestion != "" {
			inbound["congestion_control"] = q.Congestion
		}
		if q.Salamander {
			inbound["obfs"] = map[string]any{"type": "salamander", "password": s.ObfsPassword}
		}
		if q.UpMbps > 0 {
			inbound["up_mbps"] = q.UpMbps
		}
		if q.DownMbps > 0 {
			inbound["down_mbps"] = q.DownMbps
		}
	}
	if s.Protocol == "wireguard" {
		cfg["inbounds"] = []any{}
		cfg["endpoints"] = []any{map[string]any{"type": "wireguard", "tag": "xingdu-in", "system": false, "mtu": s.WireGuard.MTU, "address": []string{"10.77.0.1/32"}, "private_key": s.WireGuardKeys.ServerPrivate, "listen_port": s.Port, "peers": []any{map[string]any{"public_key": wgPublic(s.WireGuardKeys.ClientPrivate), "pre_shared_key": s.WireGuardKeys.Preshared, "allowed_ips": []string{"10.77.0.2/32"}}}}}
	}
}
