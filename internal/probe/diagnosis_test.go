package probe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func TestDiagnosisPinsBothFamiliesAndDoesNotClaimForwarding(t *testing.T) {
	lookup := func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("2606:4700:4700::1111")}, nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			t.Errorf("unexpected network %s", network)
		}
		switch address {
		case "1.1.1.1:443":
			a, b := net.Pipe()
			b.Close()
			return a, nil
		case "[2606:4700:4700::1111]:443":
			return nil, &net.OpError{Err: syscall.ENETUNREACH}
		default:
			t.Errorf("unresolved address %s", address)
			return nil, errors.New("bad target")
		}
	}
	spec := protocol.Spec{Input: protocol.Input{Protocol: "shadowsocks", Port: 443}, Credential: "password"}
	report := diagnose(context.Background(), "node.example.invalid", spec, "", lookup, dial)
	if len(report.Addresses) != 2 || report.Addresses[0].TCP.Status != "passed" || report.Addresses[0].Forwarding.Status != "unknown" || report.Addresses[1].TCP.Code != "probe_network_unavailable" {
		t.Fatalf("unexpected evidence: %+v", report)
	}
}
func TestDiagnosisRejectsMixedPublicPrivateDNSBeforeDial(t *testing.T) {
	for _, bad := range []string{"::1", "::ffff:127.0.0.1", "fd00::1", "fe80::1%eth0", "100.100.100.200", "64:ff9b::a00:1"} {
		t.Run(bad, func(t *testing.T) {
			lookup := func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr(bad)}, nil
			}
			dial := func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("unsafe target dialed")
				return nil, nil
			}
			report := diagnose(context.Background(), "node.example.invalid", protocol.Spec{}, "", lookup, dial)
			if report.DNS.Code != "target_blocked" || len(report.Addresses) != 0 {
				t.Fatalf("unsafe result %+v", report)
			}
		})
	}
}
func TestUDPDiagnosisNeverUsesTCP(t *testing.T) {
	lookup := func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111")}, nil
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("UDP node tested using TCP")
		return nil, nil
	}
	report := diagnose(context.Background(), "node.example.invalid", protocol.Spec{Input: protocol.Input{Protocol: "hysteria2", Port: 443}}, "", lookup, dial)
	if report.Addresses[0].TCP.Status != "not_applicable" || report.Addresses[0].TLS.Status != "not_applicable" || report.Addresses[0].Forwarding.Status != "unknown" {
		t.Fatalf("unexpected UDP evidence %+v", report)
	}
}

func TestDiagnosisTLSVerifiesSelectedCertificateAndHostname(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "node.example.invalid"}, DNSNames: []string{"node.example.invalid"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	lookup := func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}
	for _, name := range []string{"node.example.invalid", "wrong.example.invalid"} {
		t.Run(name, func(t *testing.T) {
			dial := func(context.Context, string, string) (net.Conn, error) {
				a, b := net.Pipe()
				go func() {
					c := tls.Server(b, &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}})
					c.Handshake()
					b.Close()
				}()
				return a, nil
			}
			spec := protocol.Spec{Input: protocol.Input{Protocol: "trojan", Port: 443, Name: "TLS test", ServerName: name, Certificate: certPEM}, Credential: "fixture-password"}
			report := diagnose(context.Background(), "node.example.invalid", spec, "", lookup, dial)
			want := "passed"
			if name != "node.example.invalid" {
				want = "failed"
			}
			if report.Addresses[0].TLS.Status != want || report.Addresses[0].Forwarding.Status != "unknown" {
				t.Fatalf("unexpected TLS report %+v", report)
			}
		})
	}
}
