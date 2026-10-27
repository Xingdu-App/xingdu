package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"time"

	"xingdu.app/xingdu/internal/bootstrap"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/subscription"
)

// Steps describe evidence, never a synthesized claim of overall node health.
type Step struct {
	Status    string `json:"status"`
	Code      string `json:"code"`
	LatencyMS int    `json:"latency_ms,omitempty"`
	ExitIP    string `json:"exit_ip,omitempty"`
}
type AddressResult struct {
	Address    string `json:"address"`
	Family     string `json:"family"`
	TCP        Step   `json:"tcp"`
	TLS        Step   `json:"tls"`
	Forwarding Step   `json:"forwarding"`
}
type NetworkReport struct {
	DNS       Step            `json:"dns"`
	Addresses []AddressResult `json:"addresses"`
}

func unknown(code string) Step { return Step{Status: "unknown", Code: code} }
func skipped(code string) Step { return Step{Status: "not_applicable", Code: code} }
func failed(code string) Step  { return Step{Status: "failed", Code: code} }

// Only public addresses may be probed, independent of SSH management allowlists.
// Resolve all answers before dialing and pin the literal IP through every layer.
func Diagnose(ctx context.Context, address string, spec protocol.Spec, binary string) NetworkReport {
	return diagnose(ctx, address, spec, binary, net.DefaultResolver.LookupNetIP, (&net.Dialer{}).DialContext)
}
func diagnose(ctx context.Context, address string, spec protocol.Spec, binary string, lookup func(context.Context, string, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) NetworkReport {
	out := NetworkReport{DNS: failed("dns_failed"), Addresses: []AddressResult{}}
	resolve, cancel := context.WithTimeout(ctx, 3*time.Second)
	ips, err := lookup(resolve, "ip", address)
	cancel()
	if err != nil || len(ips) == 0 {
		return out
	}
	// Use the same strict public-target policy as machine access; no private allowlist.
	for _, ip := range ips {
		if !bootstrap.PublicTarget(ip) {
			out.DNS = failed("target_blocked")
			return out
		}
	}
	out.DNS = Step{Status: "passed", Code: "resolved"}
	selected := []netip.Addr{}
	seen := map[bool]bool{}
	for _, ip := range ips {
		ip = ip.Unmap()
		if !seen[ip.Is6()] {
			seen[ip.Is6()] = true
			selected = append(selected, ip)
		}
	}
	out.Addresses = make([]AddressResult, len(selected))
	var wg sync.WaitGroup
	for i, ip := range selected {
		wg.Add(1)
		go func(i int, ip netip.Addr) {
			defer wg.Done()
			out.Addresses[i] = diagnoseAddress(ctx, ip, spec, binary, dial)
		}(i, ip)
	}
	wg.Wait()
	return out
}
func diagnoseAddress(ctx context.Context, ip netip.Addr, spec protocol.Spec, binary string, dial func(context.Context, string, string) (net.Conn, error)) AddressResult {
	out := AddressResult{Address: ip.String(), Family: "IPv4", TCP: unknown("not_checked"), TLS: unknown("not_checked"), Forwarding: unknown("runtime_unavailable")}
	if ip.Is6() {
		out.Family = "IPv6"
	}
	udp := protocol.IsQUIC(spec.Protocol) || spec.Protocol == "wireguard"
	endpoint := net.JoinHostPort(ip.String(), strconv.Itoa(spec.Port))
	if udp {
		out.TCP = skipped("udp_transport")
		out.TLS = skipped("udp_transport")
	} else {
		check, cancel := context.WithTimeout(ctx, 3*time.Second)
		start := time.Now()
		conn, err := dial(check, "tcp", endpoint)
		cancel()
		if err != nil {
			out.TCP = failed("tcp_failed")
			if errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.EAFNOSUPPORT) {
				out.TCP = unknown("probe_network_unavailable")
			}
			out.TLS = unknown("tcp_required")
			out.Forwarding = unknown("tcp_required")
			return out
		}
		out.TCP = Step{Status: "passed", Code: "connected", LatencyMS: int(time.Since(start).Milliseconds())}
		if !spec.TLSEnabled() {
			out.TLS = skipped("no_tls")
			conn.Close()
		} else if spec.RealityEnabled() {
			out.TLS = skipped("reality_handshake")
			conn.Close()
		} else {
			roots := x509.NewCertPool()
			roots.AppendCertsFromPEM([]byte(spec.Certificate))
			cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: spec.ServerName, RootCAs: roots}
			if spec.V2Ray != nil {
				cfg.NextProtos = spec.V2Ray.ALPN
				if len(cfg.NextProtos) == 0 {
					switch spec.V2Ray.Network {
					case "grpc", "http":
						cfg.NextProtos = []string{"h2"}
					case "ws", "httpupgrade":
						cfg.NextProtos = []string{"http/1.1"}
					}
				}
			}
			if spec.Protocol == "trusttunnel" {
				cfg.NextProtos = []string{"h2"}
			}
			tlsConn := tls.Client(conn, cfg)
			check, cancel = context.WithTimeout(ctx, 3*time.Second)
			err = tlsConn.HandshakeContext(check)
			cancel()
			tlsConn.Close()
			if err != nil {
				out.TLS = failed("tls_failed")
				out.Forwarding = unknown("tls_required")
				return out
			}
			out.TLS = Step{Status: "passed", Code: "certificate_verified"}
		}
	}
	// Client capability checks precede runtime checks, so unsupported is explicit.
	node := subscription.Node{ID: "diagnosis", Name: spec.Name, Server: ip.String(), Spec: spec}
	if _, err := subscription.ProbeConfig(node, 12345); err != nil {
		out.Forwarding = unknown("client_unsupported")
		return out
	}
	if binary == "" {
		return out
	}
	latency, exit, err := RunNode(ctx, binary, "https://api.ipify.org", node)
	if err != nil {
		out.Forwarding = failed("protocol_or_egress_failed")
		if errors.Is(err, ErrRuntime) {
			out.Forwarding = unknown("runtime_unavailable")
		}
		return out
	}
	out.Forwarding = Step{Status: "passed", Code: "https_forwarded", LatencyMS: latency, ExitIP: exit}
	return out
}
