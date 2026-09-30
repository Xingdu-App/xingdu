package bootstrap

import (
	"archive/tar"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/machine"
)

func TestSSRFPolicy(t *testing.T) {
	c := &Connector{}
	for _, s := range []string{"127.0.0.1", "::1", "169.254.169.254", "10.1.1.1", "100.100.100.200", "172.18.0.1", "192.168.1.1", "224.0.0.1", "::ffff:127.0.0.1", "fd00::1", "fd00:ec2::254", "168.63.129.16", "fe80::1", "2002:7f00:1::", "64:ff9b::a9fe:a9fe"} {
		if c.allowed(netip.MustParseAddr(s)) {
			t.Fatal("SSRF allowed", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !c.allowed(netip.MustParseAddr(s)) {
			t.Fatal("public target blocked", s)
		}
	}
	c.Allowed = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16")}
	if !c.allowed(netip.MustParseAddr("10.2.3.4")) || c.allowed(netip.MustParseAddr("127.0.0.1")) || c.allowed(netip.MustParseAddr("169.254.169.254")) {
		t.Fatal("unsafe operator allowlist")
	}
}
func TestSSHAuthenticationAndInstallBundle(t *testing.T) {
	_, hostKey, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(hostKey)
	pub, private, _ := ed25519.GenerateKey(rand.Reader)
	clientKey, _ := ssh.NewPublicKey(pub)
	var auths atomic.Int32
	var preflightExit atomic.Uint32
	bundles := make(chan map[string]string, 4)
	cfg := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		auths.Add(1)
		if string(p) != "test-password" {
			return nil, io.EOF
		}
		return nil, nil
	}, PublicKeyCallback: func(_ ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
		auths.Add(1)
		if string(k.Marshal()) != string(clientKey.Marshal()) {
			return nil, io.EOF
		}
		return nil, nil
	}}
	cfg.AddHostKey(hostSigner)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				server, channels, requests, e := ssh.NewServerConn(conn, cfg)
				if e != nil {
					conn.Close()
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for newChannel := range channels {
					ch, reqs, e := newChannel.Accept()
					if e != nil {
						return
					}
					go func() {
						defer ch.Close()
						for req := range reqs {
							if req.Type != "exec" {
								req.Reply(false, nil)
								continue
							}
							var p struct{ Command string }
							ssh.Unmarshal(req.Payload, &p)
							req.Reply(true, nil)
							if p.Command == "uname -sm" {
								ch.Write([]byte("Linux x86_64\n"))
							} else if strings.Contains(p.Command, installPreflight) {
								if status := preflightExit.Load(); status != 0 {
									ch.Stderr().Write([]byte("untrusted output: test-password\n"))
									ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
									return
								}
							} else {
								files := map[string]string{"command": p.Command}
								tr := tar.NewReader(ch)
								for {
									header, e := tr.Next()
									if e != nil {
										break
									}
									data, _ := io.ReadAll(io.LimitReader(tr, 1024*1024))
									files[header.Name] = string(data)
								}
								bundles <- files
							}
							ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							return
						}
					}()
				}
			}()
		}
	}()
	c := &Connector{ArtifactDir: t.TempDir(), dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, listener.Addr().String())
	}}
	os.WriteFile(filepath.Join(c.ArtifactDir, "xingdu-agent-linux-amd64"), []byte("test-agent-binary"), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	target := machine.Target{Address: "vps.example.invalid", Port: 22, User: "root"}
	fp, e := c.Inspect(ctx, target)
	if e != nil || fp != ssh.FingerprintSHA256(hostSigner.PublicKey()) {
		t.Fatal("inspect", e)
	}
	if auths.Load() != 0 {
		t.Fatal("inspection sent credentials")
	}
	secret := machine.Secret{Target: target, Method: "password", Password: "test-password", Fingerprint: "SHA256:" + strings.Repeat("a", 43), Mode: "monitor", EnrollmentToken: machine.Token()}
	if _, e = c.client(ctx, secret); e == nil || e.Error() != "host_key_changed" {
		t.Fatal("host pin not enforced", e)
	}
	if auths.Load() != 0 {
		t.Fatal("password sent before host verification")
	}
	secret.Fingerprint = fp
	if e = c.Install(ctx, secret, "https://control.example.invalid"); e != nil {
		t.Fatal(e)
	}
	bundle := <-bundles
	if bundle["enrollment.token"] != secret.EnrollmentToken+"\n" || strings.Contains(bundle["command"], secret.EnrollmentToken) || strings.Contains(bundle["command"], secret.Password) {
		t.Fatal("unsafe installation transport")
	}
	preflightExit.Store(70)
	if e = c.Install(ctx, secret, "https://control.example.invalid"); e == nil || e.Error() != "ssh_tar_missing" {
		t.Fatal("missing tar was not recorded safely", e)
	}
	select {
	case <-bundles:
		t.Fatal("credentials uploaded after failed preflight")
	default:
	}
	preflightExit.Store(75)
	if e = c.Install(ctx, secret, "https://control.example.invalid"); e == nil || e.Error() != "ssh_tar_install_failed" {
		t.Fatal("tar installation failure was not recorded safely", e)
	}
	select {
	case <-bundles:
		t.Fatal("credentials uploaded after package installation failure")
	default:
	}
	preflightExit.Store(0)
	digest := sha256.Sum256([]byte("test-agent-binary"))
	job := machine.Job{Action: "upgrade", TargetVersion: machine.Version, AgentHash: machine.Hash(machine.Token()), Arch: "amd64", ArtifactSHA256: hex.EncodeToString(digest[:])}
	if e = c.UpgradeChecked(ctx, secret, "https://control.example.invalid", job, nil); e != nil {
		t.Fatal("upgrade bundle", e)
	}
	upgradeBundle := <-bundles
	if upgradeBundle["enrollment.token"] != "" || !strings.Contains(upgradeBundle["install.sh"], "--upgrade < upgrade.json") || strings.Contains(upgradeBundle["command"], job.AgentHash) || !strings.Contains(upgradeBundle["upgrade.json"], job.AgentHash) {
		t.Fatal("unsafe upgrade bundle")
	}
	job.ArtifactSHA256 = strings.Repeat("0", 64)
	if e = c.UpgradeChecked(ctx, secret, "https://control.example.invalid", job, nil); e == nil || e.Error() != "upgrade_release_changed" {
		t.Fatal("changed artifact accepted", e)
	}
	block, _ := ssh.MarshalPrivateKey(private, "")
	secret.Method = "pem"
	secret.Password = ""
	secret.PrivateKey = string(pem.EncodeToMemory(block))
	if e = c.Install(ctx, secret, "https://control.example.invalid"); e != nil {
		t.Fatal("PEM auth", e)
	}
	<-bundles
	encrypted, _ := ssh.MarshalPrivateKeyWithPassphrase(private, "", []byte("test-passphrase"))
	secret.PrivateKey = string(pem.EncodeToMemory(encrypted))
	secret.Passphrase = "test-passphrase"
	if e = ValidateSecret(secret); e != nil {
		t.Fatal("encrypted PEM", e)
	}
	secret.Passphrase = "wrong"
	if e = ValidateSecret(secret); e == nil {
		t.Fatal("wrong passphrase accepted")
	}
}
