package bootstrap

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/agent"
	"xingdu.app/xingdu/internal/machine"
)

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("::/96"), netip.MustParsePrefix("::ffff:0:0/96"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("fc00::/7"), netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("ff00::/8"),
}

type Connector struct {
	Allowed     []netip.Prefix
	ArtifactDir string
	dial        func(context.Context, string, string) (net.Conn, error)
}

func (c *Connector) allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip == netip.MustParseAddr("100.100.100.200") || ip == netip.MustParseAddr("168.63.129.16") || ip == netip.MustParseAddr("fd00:ec2::254") {
		return false
	}
	if !ip.IsValid() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// Operator allowlists may enable a private management subnet, never metadata/loopback.
	for _, p := range c.Allowed {
		if p.Contains(ip) {
			return true
		}
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return ip.IsGlobalUnicast()
}
func (c *Connector) connection(ctx context.Context, t machine.Target) (net.Conn, error) {
	if c.dial != nil {
		return c.dial(ctx, "tcp", net.JoinHostPort(t.Address, strconv.Itoa(t.Port)))
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", t.Address)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("dns_failed")
	}
	for _, ip := range ips {
		if !c.allowed(ip) {
			return nil, errors.New("ssh_target_blocked")
		}
	}
	d := net.Dialer{Timeout: 8 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ips[0].Unmap().String(), strconv.Itoa(t.Port)))
	if err != nil {
		return nil, errors.New("ssh_unreachable")
	}
	return conn, nil
}
func ValidateSecret(s machine.Secret) error {
	if !strings.HasPrefix(s.Fingerprint, "SHA256:") || len(s.Fingerprint) != 50 {
		return errors.New("invalid_fingerprint")
	}
	switch s.Method {
	case "password":
		if len(s.Password) == 0 || len(s.Password) > 4096 || s.PrivateKey != "" || s.Passphrase != "" {
			return errors.New("invalid_password")
		}
	case "pem":
		if s.Password != "" || len(s.PrivateKey) > 24*1024 || len(s.Passphrase) > 4096 {
			return errors.New("invalid_private_key")
		}
		if _, err := signer(s); err != nil {
			return errors.New("invalid_private_key")
		}
	default:
		return errors.New("invalid_auth_method")
	}
	return nil
}
func signer(s machine.Secret) (ssh.Signer, error) {
	if s.Passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(s.PrivateKey), []byte(s.Passphrase))
	}
	return ssh.ParsePrivateKey([]byte(s.PrivateKey))
}
func (c *Connector) Inspect(ctx context.Context, t machine.Target) (string, error) {
	conn, err := c.connection(ctx, t)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	fingerprint := ""
	cfg := &ssh.ClientConfig{User: t.User, HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		fingerprint = ssh.FingerprintSHA256(k)
		return errors.New("fingerprint_only_no_authentication")
	}}
	client, _, _, _ := ssh.NewClientConn(conn, net.JoinHostPort(t.Address, strconv.Itoa(t.Port)), cfg)
	if client != nil {
		client.Close()
	}
	if fingerprint == "" {
		return "", errors.New("ssh_handshake_failed")
	}
	return fingerprint, nil
}
func (c *Connector) client(ctx context.Context, s machine.Secret) (*ssh.Client, error) {
	if err := ValidateSecret(s); err != nil {
		return nil, err
	}
	var auth ssh.AuthMethod
	if s.Method == "password" {
		auth = ssh.Password(s.Password)
	} else {
		k, err := signer(s)
		if err != nil {
			return nil, errors.New("invalid_private_key")
		}
		auth = ssh.PublicKeys(k)
	}
	conn, err := c.connection(ctx, s.Target)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(100 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	mismatch := false
	cfg := &ssh.ClientConfig{User: s.Target.User, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: func(_ string, _ net.Addr, k ssh.PublicKey) error {
		if subtle.ConstantTimeCompare([]byte(ssh.FingerprintSHA256(k)), []byte(s.Fingerprint)) != 1 {
			mismatch = true
			return errors.New("host_key_changed")
		}
		return nil
	}}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(s.Target.Address, strconv.Itoa(s.Target.Port)), cfg)
	if err != nil {
		stop()
		conn.Close()
		if mismatch {
			return nil, errors.New("host_key_changed")
		}
		return nil, errors.New("ssh_auth_or_handshake_failed")
	}
	// Cancellation closes the underlying connection; its callback ends with the caller context.
	return ssh.NewClient(sshConn, chans, reqs), nil
}

type limitedOutput struct{ buf bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if b.buf.Len() < 4096 {
		_, _ = b.buf.Write(p[:min(n, 4096-b.buf.Len())])
	}
	return n, nil
}
func run(client *ssh.Client, command string, input io.Reader) (string, error) {
	s, e := client.NewSession()
	if e != nil {
		return "", e
	}
	defer s.Close()
	var b limitedOutput
	s.Stdout = &b
	s.Stderr = io.Discard
	s.Stdin = input
	e = s.Run(command)
	return b.buf.String(), e
}
func (c *Connector) Install(ctx context.Context, s machine.Secret, origin string) error {
	return c.InstallChecked(ctx, s, origin, nil)
}
func (c *Connector) InstallChecked(ctx context.Context, s machine.Secret, origin string, check func() error) error {
	return c.applyChecked(ctx, s, origin, machine.Job{}, check)
}
func (c *Connector) UpgradeChecked(ctx context.Context, s machine.Secret, origin string, j machine.Job, check func() error) error {
	return c.applyChecked(ctx, s, origin, j, check)
}
func (c *Connector) applyChecked(ctx context.Context, s machine.Secret, origin string, j machine.Job, check func() error) error {
	upgrading := j.Action == "upgrade"
	if upgrading && (j.TargetVersion != machine.Version || !machine.ValidToken(j.AgentHash) || !machine.ValidToken(j.ArtifactSHA256)) {
		return errors.New("upgrade_release_changed")
	}
	if machine.Origin(origin) != nil || !strings.HasPrefix(origin, "https://") {
		return errors.New("public_https_origin_required")
	}
	if (!upgrading && !machine.ValidToken(s.EnrollmentToken)) || !machine.ValidMode(s.Mode) {
		return errors.New("invalid_install_request")
	}
	client, err := c.client(ctx, s)
	if err != nil {
		return err
	}
	defer client.Close()
	platform, err := run(client, "uname -sm", nil)
	if err != nil {
		return errors.New("platform_check_failed")
	}
	arch := ""
	switch strings.TrimSpace(platform) {
	case "Linux x86_64":
		arch = "amd64"
	case "Linux aarch64", "Linux arm64":
		arch = "arm64"
	default:
		return errors.New("unsupported_platform")
	}
	binary, err := os.ReadFile(filepath.Join(c.ArtifactDir, "xingdu-agent-linux-"+arch))
	if err != nil {
		return errors.New("agent_artifact_unavailable")
	}
	script := fmt.Sprintf("#!/bin/sh\nset -eu\n./agent --install --server %s --mode %s < enrollment.token\n", shellQuote(origin), s.Mode)
	extraName, extraData := "enrollment.token", []byte(s.EnrollmentToken+"\n")
	if upgrading {
		digest := sha256.Sum256(binary)
		if j.Arch != arch || hex.EncodeToString(digest[:]) != j.ArtifactSHA256 {
			return errors.New("upgrade_release_changed")
		}
		extraName = "upgrade.json"
		extraData, _ = json.Marshal(agent.UpgradeExpectation{Server: origin, Mode: s.Mode, TokenHash: j.AgentHash, Version: j.TargetVersion})
		script = fmt.Sprintf("#!/bin/sh\nset -eu\nprintf '%%s  agent\\n' '%s' | sha256sum -c - >/dev/null\ntest \"$(./agent --version)\" = '%s'\n./agent --upgrade < upgrade.json\n", j.ArtifactSHA256, "xingdu-agent "+j.TargetVersion)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range []struct {
		name string
		data []byte
		mode int64
	}{{"agent", binary, 0700}, {"install.sh", []byte(script), 0700}, {extraName, extraData, 0600}} {
		if err = tw.WriteHeader(&tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.data))}); err != nil {
			return errors.New("bundle_failed")
		}
		if _, err = tw.Write(f.data); err != nil {
			return errors.New("bundle_failed")
		}
	}
	if err = tw.Close(); err != nil {
		return errors.New("bundle_failed")
	}
	defer clear(buf.Bytes())
	// All paths and commands are fixed. Credentials and tokens travel in encrypted stdin, not argv.
	command := `umask 077; mkdir -p -m 0755 /usr/local/bin || exit 1; d=$(mktemp -d /usr/local/bin/.xingdu-install.XXXXXXXX) || exit 1; trap 'rm -rf "$d"' EXIT HUP INT TERM; cd "$d" || exit 1; tar -xf - || exit 1; sh ./install.sh`
	if s.Target.User != "root" {
		command = "sudo -n sh -c " + shellQuote(command)
	} else {
		command = "sh -c " + shellQuote(command)
	}
	if check != nil {
		if err = check(); err != nil {
			return errors.New("authorization_revoked")
		}
	}
	_, err = run(client, command, bytes.NewReader(buf.Bytes()))
	if err != nil {
		if upgrading {
			return errors.New("upgrade_failed_check_vps")
		}
		return errors.New("install_failed_check_vps")
	}
	return nil
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
