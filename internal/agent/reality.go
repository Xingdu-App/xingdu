package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/protocol"
)

// Resolve through a public resolver inside the protected service cgroup, rather
// than depending on a blocked loopback stub resolver or re-resolving in Xray.
func resolveRealityTarget(ctx context.Context, host string) (string, error) {
	if !protocol.ValidHandshakeHost(host) {
		return "", errors.New("invalid REALITY handshake host")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r := net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, "9.9.9.9:53")
	}}
	ips, err := r.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return "", errors.New("REALITY resolution failed")
	}
	for _, ip := range ips {
		if !certificates.PublicAddress(ip.IP.String()) {
			return "", errors.New("unsafe REALITY address")
		}
	}
	return net.JoinHostPort(ips[0].IP.String(), "443"), nil
}

func pinRealityTarget(config []byte, target string) ([]byte, error) {
	var cfg map[string]any
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, err
	}
	ins, ok := cfg["inbounds"].([]any)
	if !ok || len(ins) != 1 {
		return nil, errors.New("invalid REALITY listener")
	}
	inbound, ok := ins[0].(map[string]any)
	if !ok {
		return nil, errors.New("invalid REALITY listener")
	}
	stream, ok := inbound["streamSettings"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid REALITY transport")
	}
	reality, ok := stream["realitySettings"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid REALITY settings")
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil || port != "443" || net.ParseIP(host) == nil || !certificates.PublicAddress(host) {
		return nil, errors.New("unsafe REALITY address")
	}
	reality["target"] = target
	return json.Marshal(cfg)
}
