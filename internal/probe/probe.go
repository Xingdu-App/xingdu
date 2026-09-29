// Package probe executes real protocol connections from an independent worker.
package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func VerifyRuntime(path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != protocol.RuntimeSHA256[runtime.GOARCH] {
		return errors.New("runtime checksum mismatch")
	}
	return nil
}
func Run(ctx context.Context, binary, endpoint string, peer protocol.Peer) (int, string, error) {
	if err := peer.Validate(); err != nil {
		return 0, "", err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return 0, "", errors.New("HTTPS probe endpoint required")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cfg := map[string]any{"log": map[string]any{"disabled": true}, "inbounds": []any{map[string]any{"type": "socks", "listen": "127.0.0.1", "listen_port": port}}, "outbounds": []any{peer.Outbound()}, "route": map[string]any{"final": "exit"}}
	b, _ := json.Marshal(cfg)
	defer clear(b)
	run, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(run, binary, "run", "-c", "stdin")
	cmd.Stdin = bytes.NewReader(b)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	if err = cmd.Start(); err != nil {
		return 0, "", errors.New("probe runtime failed")
	}
	defer func() { cancel(); cmd.Wait() }()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ready := false
	for i := 0; i < 50; i++ {
		c, e := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if e == nil {
			c.Close()
			ready = true
			break
		}
		select {
		case <-run.Done():
			return 0, "", run.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		return 0, "", errors.New("probe runtime not ready")
	}
	proxy, _ := url.Parse("socks5://" + address)
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequestWithContext(run, "GET", endpoint, nil)
	start := time.Now()
	res, err := client.Do(req)
	elapsed := int(time.Since(start).Milliseconds())
	if err != nil {
		return elapsed, "", errors.New("protocol probe failed")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1025))
	if err != nil || res.StatusCode != 200 || len(body) > 1024 {
		return elapsed, "", errors.New("invalid probe response")
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return elapsed, "", errors.New("probe endpoint must return an IP address")
	}
	return elapsed, ip, nil
}
