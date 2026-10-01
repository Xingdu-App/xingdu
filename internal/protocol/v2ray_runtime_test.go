package protocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in authenticated forwarding against the pinned Linux runtime. Docker has
// no external network or published ports; the public-looking target exists only
// on this disposable container's loopback. No host route or firewall is changed.
func TestV2RayRuntimeForwarding(t *testing.T) {
	binary := os.Getenv("XINGDU_TEST_V2RAY_RUNTIME")
	if binary == "" {
		t.Skip("set XINGDU_TEST_V2RAY_RUNTIME to pinned Linux arm64 sing-box")
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != RuntimeSHA256["arm64"] {
		t.Fatal("runtime is not the pinned arm64 executable")
	}
	dir := t.TempDir()
	var server map[string]any
	var inbounds, clients, outbounds, rules []any
	var probes []string
	index := 0
	for _, kind := range []string{"vless", "vmess"} {
		for _, network := range []string{"tcp", "ws", "grpc", "http", "vision"} {
			for _, enabled := range []bool{false, true} {
				if network == "vision" && (kind != "vless" || !enabled) {
					continue
				}
				in := inputFixture(t)
				in.Protocol = kind
				in.Port = 20000 + index
				in.V2Ray = &V2RayOptions{Network: network, TLS: &enabled}
				if network == "vision" {
					in.V2Ray.Network = "tcp"
					in.V2Ray.Flow = "xtls-rprx-vision"
				} else if network == "grpc" {
					in.V2Ray.ServiceName = "lab"
				} else if network != "tcp" {
					in.V2Ray.Path = "/lab"
					in.V2Ray.Host = "proxy.example.com"
				}
				if !enabled {
					in.Certificate = ""
					in.PrivateKey = ""
					in.ServerName = ""
				}
				s, e := NewSpec(in)
				if e != nil {
					t.Fatal(e)
				}
				rendered, e := Render(s)
				if e != nil {
					t.Fatal(e)
				}
				var cfg map[string]any
				if e = json.Unmarshal(rendered, &cfg); e != nil {
					t.Fatal(e)
				}
				inbound := cfg["inbounds"].([]any)[0].(map[string]any)
				inbound["tag"] = fmt.Sprintf("server-%d", index)
				inbounds = append(inbounds, inbound)
				server = cfg
				tag := fmt.Sprintf("client-%d", index)
				clients = append(clients, map[string]any{"type": "mixed", "tag": tag, "listen": "127.0.0.1", "listen_port": 21000 + index})
				out := map[string]any{"type": kind, "tag": tag, "server": "127.0.0.1", "server_port": in.Port, "uuid": s.Credential}
				if in.V2Ray.Flow != "" {
					out["flow"] = in.V2Ray.Flow
				}
				if kind == "vmess" {
					out["security"] = "auto"
				}
				if enabled {
					out["tls"] = map[string]any{"enabled": true, "server_name": in.ServerName, "certificate": strings.Split(strings.TrimSpace(in.Certificate), "\n"), "alpn": in.V2Ray.alpn()}
				}
				if tr := in.V2Ray.transport(); tr != nil {
					out["transport"] = tr
				}
				outbounds = append(outbounds, out)
				rules = append(rules, map[string]any{"inbound": []string{tag}, "action": "route", "outbound": tag})
				probes = append(probes, fmt.Sprintf("test \"$(curl -fsS --max-time 6 --socks5-hostname 127.0.0.1:%d http://93.184.216.34:18081/probe)\" = xingdu-v2ray-runtime-ok || { echo 'failed %s/%s TLS=%t'; tail -15 /tmp/server.log; exit 1; }", 21000+index, kind, network, enabled))
				index++
			}
		}
	}
	server["inbounds"] = inbounds
	server["log"] = map[string]any{"level": "debug"}
	// Deliberately wrong authentication, while using an otherwise valid transport.
	bad := map[string]any{"type": "vless", "tag": "bad", "server": "127.0.0.1", "server_port": 20000, "uuid": randomUUID()}
	clients = append(clients, map[string]any{"type": "mixed", "tag": "bad", "listen": "127.0.0.1", "listen_port": 21999})
	outbounds = append(outbounds, bad)
	rules = append(rules, map[string]any{"inbound": []string{"bad"}, "action": "route", "outbound": "bad"})
	client := map[string]any{"log": map[string]any{"disabled": true}, "inbounds": clients, "outbounds": outbounds, "route": map[string]any{"rules": rules}}
	for name, cfg := range map[string]any{"server.json": server, "client.json": client} {
		b, e := json.Marshal(cfg)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	script := `set -eu
ip link set lo up
ip addr add 93.184.216.34/32 dev lo
mkdir /tmp/target
printf xingdu-v2ray-runtime-ok > /tmp/target/probe
python3 -m http.server 18081 --bind 0.0.0.0 --directory /tmp/target >/tmp/target.log 2>&1 &
/runtime check -c /configs/server.json
/runtime check -c /configs/client.json
/runtime run -c /configs/server.json >/tmp/server.log 2>&1 &
/runtime run -c /configs/client.json >/tmp/client.log 2>&1 &
sleep 1
` + strings.Join(probes, "\n") + `
if curl -fsS --max-time 3 --socks5-hostname 127.0.0.1:21999 http://93.184.216.34:18081/probe >/dev/null 2>&1; then echo 'bad credential accepted'; exit 1; fi
if curl -fsS --max-time 3 --socks5-hostname 127.0.0.1:21000 http://127.0.0.1:18081/probe >/dev/null 2>&1; then echo 'private target accepted'; exit 1; fi
echo '17 transport/TLS/Vision handshakes, bad authentication and private egress checks passed'
`
	if err = os.WriteFile(filepath.Join(dir, "test.sh"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := fmt.Sprintf("xingdu-v2ray-test-%d", time.Now().UnixNano())
	defer exec.Command("docker", "rm", "-f", name).Run()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none", "--cap-add", "NET_ADMIN", "--platform", "linux/arm64", "-v", binary+":/runtime:ro", "-v", dir+":/configs:ro", "--entrypoint", "sh", "xingdu-lab-amazon:latest", "/configs/test.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime acceptance: %v\n%s", err, output)
	}
	t.Log(string(output))
}
