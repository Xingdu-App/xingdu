package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func matrixSpecs(t *testing.T) []Spec {
	t.Helper()
	b, e := os.ReadFile("testdata/compatibility72.json")
	if e != nil {
		t.Fatal(e)
	}
	var inputs []Input
	if e = json.Unmarshal(b, &inputs); e != nil {
		t.Fatal(e)
	}
	if len(inputs) != 72 {
		t.Fatal("matrix must contain 72 original cases")
	}
	specs := make([]Spec, 0, 72)
	for i, in := range inputs {
		if in.NeedsCertificate() {
			cert := inputFixture(t)
			in.ServerName = cert.ServerName
			in.Certificate = cert.Certificate
			in.PrivateKey = cert.PrivateKey
		}
		s, e := NewSpec(in)
		if e != nil {
			t.Fatalf("case %d: %v", i+1, e)
		}
		specs = append(specs, s)
	}
	return specs
}
func TestCompatibility72Render(t *testing.T) {
	for i, s := range matrixSpecs(t) {
		if _, e := Render(s); e != nil {
			t.Fatalf("case %d: %v", i+1, e)
		}
	}
}
func singClient(s Spec, port int) []byte {
	out := map[string]any{"type": s.Protocol, "tag": "proxy", "server": "127.0.0.1", "server_port": s.Port}
	switch s.Protocol {
	case "socks":
		out["version"] = "5"
		out["username"] = "xingdu"
		out["password"] = s.Credential
	case "tuic":
		out["uuid"] = s.Credential
		out["password"] = s.Password
		if s.QUIC != nil && s.QUIC.Congestion != "" {
			out["congestion_control"] = s.QUIC.Congestion
		}
	case "hysteria":
		out["auth_str"] = s.Credential
		out["up_mbps"] = s.QUIC.UpMbps
		out["down_mbps"] = s.QUIC.DownMbps
	default:
		out["password"] = s.Credential
	}
	if s.NeedsCertificate() {
		out["tls"] = map[string]any{"enabled": true, "server_name": s.ServerName, "certificate": strings.Split(strings.TrimSpace(s.Certificate), "\n")}
		if s.QUIC != nil && len(s.QUIC.ALPN) > 0 {
			out["tls"].(map[string]any)["alpn"] = s.QUIC.ALPN
		}
	}
	if s.QUIC != nil && s.QUIC.Salamander {
		out["obfs"] = map[string]any{"type": "salamander", "password": s.ObfsPassword}
	}
	cfg := map[string]any{"log": map[string]any{"level": "error"}, "inbounds": []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": port}}, "outbounds": []any{out}, "route": map[string]any{"final": "proxy"}}
	if s.Protocol == "wireguard" {
		delete(cfg, "outbounds")
		cfg["endpoints"] = []any{map[string]any{"type": "wireguard", "tag": "proxy", "system": false, "mtu": s.WireGuard.MTU, "address": []string{"10.77.0.2/32"}, "private_key": s.WireGuardKeys.ClientPrivate, "peers": []any{map[string]any{"address": "127.0.0.1", "port": s.Port, "public_key": wgPublic(s.WireGuardKeys.ServerPrivate), "pre_shared_key": s.WireGuardKeys.Preshared, "allowed_ips": []string{"0.0.0.0/0"}, "persistent_keepalive_interval": s.WireGuard.Keepalive, "reserved": s.WireGuard.Reserved}}}}
	}
	b, _ := json.Marshal(cfg)
	return b
}
func TestCompatibility72Runtime(t *testing.T) {
	dir := os.Getenv("XINGDU_TEST_MATRIX_RUNTIME")
	if dir == "" {
		t.Skip("requires directory with pinned arm64 Xray, sing-box and launcher")
	}
	cases := matrixSpecs(t)
	if os.Getenv("XINGDU_TEST_MATRIX_EXTRA") == "httpupgrade" {
		var extras []Spec
		for _, enabled := range []bool{false, true} {
			for _, original := range cases[:2] {
				in := original.Input
				opts := *in.V2Ray
				opts.Network, opts.Path, opts.TLS = "httpupgrade", "/upgrade", &enabled
				in.V2Ray = &opts
				if !enabled {
					in.ServerName, in.Certificate, in.PrivateKey = "", "", ""
				}
				spec, err := NewSpec(in)
				if err != nil {
					t.Fatal(err)
				}
				extras = append(extras, spec)
			}
		}
		cases = extras
	}
	config := t.TempDir()
	for _, name := range []string{"udp_probe.py", "udp_target.py", "http_target.py"} {
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(config, name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var script strings.Builder
	script.WriteString("set -u\nip link set lo up\nip addr add 93.184.216.34/32 dev lo\nip addr add 93.184.216.35/32 dev lo\nip route add default dev lo\nmkdir /tmp/target\nprintf xingdu-matrix-ok > /tmp/target/probe\npython3 /configs/http_target.py >/tmp/target.log 2>&1 &\npython3 /configs/udp_target.py >/tmp/udp-target.log 2>&1 &\nfailed=0\n")
	executed := 0
	for i, s := range cases {
		index := i + 1
		if only := os.Getenv("XINGDU_TEST_MATRIX_CASE"); only != "" && only != fmt.Sprint(index) {
			continue
		}
		executed++
		udpCheck := "true"
		if s.Protocol != "anytls" {
			udpCheck = "python3 /configs/udp_probe.py >/tmp/udp.log 2>&1"
		}
		server, e := Render(s)
		if e != nil {
			t.Fatal(e)
		}
		client := singClient(s, 21900)
		serverRun := "/runtimes/sing-box run -c"
		clientRun := serverRun
		if s.UsesXray() {
			serverRun = "/runtimes/xray-" + XrayVersion + " run -c"
		}
		if s.V2Ray != nil && s.UsesXray() {
			client, e = XrayClient(s, "127.0.0.1", 21900)
			if e != nil {
				t.Fatal(e)
			}
			clientRun = "/runtimes/xray-reference run -config"
		}
		for file, b := range map[string][]byte{fmt.Sprintf("server-%d.json", index): server, fmt.Sprintf("client-%d.json", index): client} {
			if e = os.WriteFile(filepath.Join(config, file), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
		fmt.Fprintf(&script, `%s /configs/server-%d.json >/tmp/server.log 2>&1 &
server=$!
%s /configs/client-%d.json >/tmp/client.log 2>&1 &
client=$!
sleep 0.25
ok=0
for attempt in 1 2 3; do
 if test "$(curl -fsS --max-time 6 --socks5-hostname 127.0.0.1:21900 http://93.184.216.34:18081/probe 2>/tmp/curl.log)" = xingdu-matrix-ok; then ok=1; break; fi
 sleep 0.1
done
if test "$ok" = 1; then
 for repeat in 1 2 3 4 5; do
  if ! test "$(curl -fsS --max-time 6 --socks5-hostname 127.0.0.1:21900 http://93.184.216.34:18081/probe 2>/tmp/curl.log)" = xingdu-matrix-ok; then ok=0; break; fi
 done
 if ! %s; then ok=0; cat /tmp/udp.log; fi
fi
if test "$ok" = 1; then echo 'case %02d PASS'; else echo 'case %02d FAIL'; cat /tmp/curl.log /tmp/server.log /tmp/client.log; failed=$((failed+1)); fi
kill "$client" "$server" 2>/dev/null || true
wait "$client" 2>/dev/null || true
wait "$server" 2>/dev/null || true
`, serverRun, index, clientRun, index, udpCheck, index, index)
	}
	fmt.Fprintf(&script, "echo \"failures=$failed total=%d\"\ntest \"$failed\" = 0\n", executed)
	if e := os.WriteFile(filepath.Join(config, "test.sh"), []byte(script.String()), 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	name := fmt.Sprintf("xingdu-matrix-%d", time.Now().UnixNano())
	defer exec.Command("docker", "rm", "-f", name).Run()
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none", "--cap-add", "NET_ADMIN", "--platform", "linux/arm64", "-v", dir+":/runtimes:ro", "-v", config+":/configs:ro", "--entrypoint", "sh", "xingdu-lab-amazon:latest", "/configs/test.sh")
	b, e := command.CombinedOutput()
	t.Log(string(b))
	if e != nil {
		t.Fatal(e)
	}
}
