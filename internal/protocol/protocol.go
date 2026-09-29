// Package protocol defines the bounded, versioned deployment contract. It never
// accepts runtime JSON, commands, file paths, or download URLs from a tenant.
package protocol

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/machine"
)

type Input struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	ServerName  string `json:"server_name"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}
type Spec struct {
	Relay *Peer `json:"relay,omitempty"`
	Input
	Credential string `json:"credential"`
	Password   string `json:"password,omitempty"`
}
type Task struct {
	ID           string `json:"id"`
	DeploymentID string `json:"deployment_id"`
	Lease        string `json:"lease"`
	Action       string `json:"action"`
	Spec         Spec   `json:"spec"`
}
type Result struct {
	ID      string `json:"id"`
	Lease   string `json:"lease"`
	Success bool   `json:"success"`
	Code    string `json:"code"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var dnsPattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

func ValidID(s string) bool       { return id.ValidID(s) }
func IsQUIC(p string) bool        { return p == "hysteria2" || p == "tuic" }
func IsShadowsocks(p string) bool { return p == "shadowsocks" || p == "shadowsocks2022" }
func RequiresTLS(p string) bool   { return !IsShadowsocks(p) }
func Username(p string) string {
	if p == "http" {
		return "xingdu"
	}
	return ""
}
func Cipher(p string) string {
	switch p {
	case "shadowsocks":
		return "chacha20-ietf-poly1305"
	case "shadowsocks2022":
		return "2022-blake3-aes-256-gcm"
	}
	return ""
}
func MinimumAgentVersion(p string) string {
	if p == "anytls" || p == "http" {
		return "0.10.0-dev"
	}
	if IsShadowsocks(p) {
		return "0.8.0-dev"
	}
	return machine.MinimumDeploymentVersion
}
func ValidateInput(in Input) error {
	if strings.TrimSpace(in.Name) != in.Name || len([]rune(in.Name)) == 0 || len([]rune(in.Name)) > 80 || strings.ContainsFunc(in.Name, unicode.IsControl) {
		return errors.New("name must contain 1-80 printable characters")
	}
	switch in.Protocol {
	case "trojan", "vless", "vmess", "hysteria2", "tuic", "shadowsocks", "shadowsocks2022", "anytls", "http":
	default:
		return errors.New("unsupported protocol")
	}
	if in.Port < 1 || in.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if IsShadowsocks(in.Protocol) {
		if in.ServerName != "" || in.Certificate != "" || in.PrivateKey != "" {
			return errors.New("Shadowsocks does not accept TLS configuration")
		}
		return nil
	}
	if len(in.ServerName) == 0 || len(in.ServerName) > 253 || !dnsPattern.MatchString(in.ServerName) {
		return errors.New("invalid TLS server name")
	}
	for _, label := range strings.Split(in.ServerName, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid TLS server name")
		}
	}
	if len(in.Certificate) > 32768 || len(in.PrivateKey) > 16384 {
		return errors.New("certificate or key too large")
	}
	pair, err := tls.X509KeyPair([]byte(in.Certificate), []byte(in.PrivateKey))
	if err != nil || len(pair.Certificate) == 0 {
		return errors.New("certificate and private key must be matching PEM")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return errors.New("invalid certificate")
	}
	if time.Now().Before(cert.NotBefore) || !time.Now().Before(cert.NotAfter) {
		return errors.New("certificate is not currently valid")
	}
	if cert.VerifyHostname(in.ServerName) != nil {
		return errors.New("certificate does not cover TLS server name")
	}
	if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("certificate does not permit digital signatures")
	}
	if len(cert.ExtKeyUsage) > 0 {
		allowed := false
		for _, u := range cert.ExtKeyUsage {
			if u == x509.ExtKeyUsageServerAuth || u == x509.ExtKeyUsageAny {
				allowed = true
			}
		}
		if !allowed {
			return errors.New("certificate does not permit server authentication")
		}
	}
	return nil
}
func randomPassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func randomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func NewSpec(in Input) (Spec, error) {
	if err := ValidateInput(in); err != nil {
		return Spec{}, err
	}
	s := Spec{Input: in, Credential: randomPassword()}
	if in.Protocol == "shadowsocks2022" {
		raw, _ := base64.RawURLEncoding.DecodeString(s.Credential)
		s.Credential = base64.StdEncoding.EncodeToString(raw)
	}
	if in.Protocol == "vless" || in.Protocol == "vmess" || in.Protocol == "tuic" {
		s.Credential = randomUUID()
	}
	if in.Protocol == "tuic" {
		s.Password = randomPassword()
	}
	return s, nil
}
func ValidateSpec(s Spec) error {
	if s.Relay != nil {
		if err := s.Relay.Validate(); err != nil {
			return err
		}
	}
	if err := ValidateInput(s.Input); err != nil {
		return err
	}
	if s.Protocol == "vless" || s.Protocol == "vmess" || s.Protocol == "tuic" {
		if !uuidPattern.MatchString(s.Credential) {
			return errors.New("invalid UUID credential")
		}
	} else if s.Protocol == "shadowsocks2022" {
		raw, err := base64.StdEncoding.DecodeString(s.Credential)
		if err != nil || len(raw) != 32 || base64.StdEncoding.EncodeToString(raw) != s.Credential {
			return errors.New("invalid Shadowsocks 2022 key")
		}
	} else if !validPassword(s.Credential) {
		return errors.New("invalid password credential")
	}
	if s.Protocol == "tuic" {
		if !validPassword(s.Password) {
			return errors.New("invalid TUIC password")
		}
	} else if s.Password != "" {
		return errors.New("unexpected password")
	}
	return nil
}
func validPassword(s string) bool {
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32
}
func Render(s Spec) ([]byte, error) {
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	user := map[string]any{"name": "xingdu"}
	if s.Protocol == "vless" || s.Protocol == "vmess" || s.Protocol == "tuic" {
		user["uuid"] = s.Credential
	} else {
		user["password"] = s.Credential
	}
	if s.Protocol == "vmess" {
		user["alterId"] = 0
	}
	if s.Protocol == "tuic" {
		user["password"] = s.Password
	}
	if s.Protocol == "http" {
		delete(user, "name")
		user["username"] = "xingdu"
	}
	tlsConfig := map[string]any{"enabled": true, "server_name": s.ServerName, "min_version": "1.2", "certificate": strings.Split(strings.TrimSpace(s.Certificate), "\n"), "key": strings.Split(strings.TrimSpace(s.PrivateKey), "\n")}
	if IsQUIC(s.Protocol) {
		tlsConfig["min_version"] = "1.3"
		tlsConfig["alpn"] = []string{"h3"}
	}
	inbound := map[string]any{"type": s.Protocol, "tag": "xingdu-in", "listen": "::", "listen_port": s.Port, "users": []any{user}, "tls": tlsConfig}
	if IsShadowsocks(s.Protocol) {
		inbound["type"] = "shadowsocks"
		// The pinned runtime routes a reused UDP session only once. Keep SS
		// TCP-only until per-packet private-destination enforcement is available.
		inbound["network"] = "tcp"
		inbound["method"] = Cipher(s.Protocol)
		inbound["password"] = s.Credential
		delete(inbound, "users")
		delete(inbound, "tls")
	}
	if s.Protocol == "tuic" {
		inbound["congestion_control"] = "bbr"
	}
	// Block private destinations before direct egress to protect host metadata and
	// private services from otherwise authenticated proxy clients.
	cfg := map[string]any{
		"log":       map[string]any{"disabled": true},
		"dns":       map[string]any{"servers": []any{map[string]any{"type": "local", "tag": "local", "prefer_go": true}}},
		"inbounds":  []any{inbound},
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
		"route": map[string]any{"rules": []any{
			map[string]any{"action": "resolve", "server": "local"},
			map[string]any{"ip_is_private": true, "action": "reject"},
			map[string]any{"ip_cidr": []string{"0.0.0.0/8", "100.64.0.0/10", "168.63.129.16/32", "224.0.0.0/4", "240.0.0.0/4", "::/128", "ff00::/8"}, "action": "reject"},
		}, "final": "direct"},
	}
	if s.Relay != nil {
		cfg["outbounds"] = []any{s.Relay.Outbound()}
		cfg["route"].(map[string]any)["final"] = "exit"
	}
	if s.Protocol == "anytls" || s.Relay != nil {
		route := cfg["route"].(map[string]any)
		route["rules"] = append([]any{map[string]any{"network": "udp", "action": "reject"}}, route["rules"].([]any)...)
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// ValidResultCode bounds agent reports to non-sensitive, user-facing codes.
func ValidResultCode(code string) bool {
	switch code {
	case "updated", "update_rolled_back", "deployed", "removed", "restarted", "manage_required", "invalid_task", "unsafe_state", "ownership_mismatch", "stop_failed", "remove_failed", "reload_failed", "invalid_spec", "port_in_use", "instance_exists", "write_failed", "runtime_unavailable", "config_rejected", "start_failed", "journal_conflict", "interrupted", "journal_unavailable", "rollback_failed":
		return true
	default:
		return false
	}
}

// CertificateExpiry returns public certificate metadata, never key material.
func CertificateExpiry(certificate string) *time.Time {
	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return &cert.NotAfter
}

type ServiceStatus struct {
	RuntimeVersion string `json:"runtime_version,omitempty"`
	ID             string `json:"id"`
	Status         string `json:"status"`
}

// ValidRuntimeVersion bounds metadata without accepting arbitrary command output.
func ValidRuntimeVersion(v string) bool { return len(v) <= 64 && machine.VersionAtLeast(v, "0.0.0-0") }
