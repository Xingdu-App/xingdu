package subscription

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"xingdu.app/xingdu/internal/protocol"
)

// The handshake origin and destination are isolated container fixtures. This
// verifies the generated client against official engines, not public VPS DNS.
func TestRealityOfficialForwarding(t *testing.T) {
	runtimeDir := os.Getenv("XINGDU_TEST_OFFICIAL_RUNTIME_DIR")
	if runtimeDir == "" {
		t.Skip("set XINGDU_TEST_OFFICIAL_RUNTIME_DIR for official Linux arm64 engines")
	}
	dir := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "www.microsoft.com"}, DNSNames: []string{"www.microsoft.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	write("origin.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))
	write("origin.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	write("origin.py", []byte(`import socket,ssl
c=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
c.minimum_version=ssl.TLSVersion.TLSv1_3
c.maximum_version=ssl.TLSVersion.TLSv1_3
c.load_cert_chain('/configs/origin.pem','/configs/origin.key')
s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('93.184.216.35',18443));s.listen()
while True:
 x,_=s.accept()
 try:
  x.settimeout(3)
  with c.wrap_socket(x,server_side=True) as y:
   y.recv(4096)
   y.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok')
 except Exception: x.close()
`))
	for i, engine := range []string{"sing-box", "xray"} {
		n := realityNode(t)
		n.Server = "127.0.0.1"
		n.Spec.Port = 20000 + i
		n.Spec.V2Ray.Engine = engine
		var server []byte
		if engine == "xray" {
			server, err = protocol.XrayServer(n.Spec, "")
		} else {
			server, err = protocol.Render(n.Spec)
		}
		if err != nil {
			t.Fatal(err)
		}
		var cfg map[string]any
		if err = json.Unmarshal(server, &cfg); err != nil {
			t.Fatal(err)
		}
		if engine == "xray" {
			cfg["log"] = map[string]any{"loglevel": "debug"}
		}
		inbound := cfg["inbounds"].([]any)[0].(map[string]any)
		if engine == "xray" {
			inbound["streamSettings"].(map[string]any)["realitySettings"].(map[string]any)["target"] = "93.184.216.35:18443"
		} else {
			handshake := inbound["tls"].(map[string]any)["reality"].(map[string]any)["handshake"].(map[string]any)
			handshake["server"], handshake["server_port"] = "93.184.216.35", 18443
			delete(handshake, "domain_resolver")
		}
		server, _ = json.Marshal(cfg)
		write(engine+".json", server)
		if engine == "xray" {
			data, e := protocol.XrayClient(n.Spec, n.Server, 21000+i)
			if e != nil {
				t.Fatal(e)
			}
			write(engine+"-client.json", data)
			var bad map[string]any
			if e = json.Unmarshal(data, &bad); e != nil {
				t.Fatal(e)
			}
			bad["inbounds"].([]any)[0].(map[string]any)["port"] = 22000 + i
			bad["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)["realitySettings"].(map[string]any)["shortId"] = "0000000000000000"
			data, _ = json.Marshal(bad)
			write(engine+"-bad.json", data)
			continue
		}
		out, err := singboxOutbound(n, "proxy")
		if err != nil {
			t.Fatal(err)
		}
		client := map[string]any{"inbounds": []any{map[string]any{"type": "mixed", "listen": "127.0.0.1", "listen_port": 21000 + i}}, "outbounds": []any{out}}
		data, _ := json.Marshal(client)
		write(engine+"-client.json", data)
		out["tls"].(map[string]any)["reality"].(map[string]any)["short_id"] = "0000000000000000"
		client["inbounds"].([]any)[0].(map[string]any)["listen_port"] = 22000 + i
		data, _ = json.Marshal(client)
		write(engine+"-bad.json", data)
	}
	script := `set -eu
ip link set lo up
ip addr add 93.184.216.34/32 dev lo
ip addr add 93.184.216.35/32 dev lo
mkdir /tmp/target
printf reality-forwarding-ok > /tmp/target/probe
python3 -m http.server 18081 --bind 0.0.0.0 --directory /tmp/target >/tmp/http.log 2>&1 &
python3 /configs/origin.py >/tmp/origin.log 2>&1 &
/runtimes/sing-box-linux-arm64 run -c /configs/sing-box.json >/tmp/singbox.log 2>&1 &
/runtimes/xray-core-26.9.9 run -config /configs/xray.json >/tmp/xray.log 2>&1 &
for engine in sing-box; do
 /runtimes/sing-box-linux-arm64 run -c /configs/$engine-client.json >/tmp/$engine-client.log 2>&1 &
 /runtimes/sing-box-linux-arm64 run -c /configs/$engine-bad.json >/tmp/$engine-bad.log 2>&1 &
done
/runtimes/xray-core-26.9.9 run -config /configs/xray-client.json >/tmp/xray-client.log 2>&1 &
/runtimes/xray-core-26.9.9 run -config /configs/xray-bad.json >/tmp/xray-bad.log 2>&1 &
sleep 2
for port in 21000 21001; do
 test "$(curl -fsS --max-time 8 --socks5-hostname 127.0.0.1:$port http://93.184.216.34:18081/probe)" = reality-forwarding-ok || { echo "failed port $port"; cat /tmp/singbox.log /tmp/xray.log /tmp/sing-box-client.log /tmp/xray-client.log /tmp/origin.log; exit 1; }
done
for port in 22000 22001; do
 if curl -fsS --max-time 4 --socks5-hostname 127.0.0.1:$port http://93.184.216.34:18081/probe >/dev/null 2>&1; then echo 'invalid short ID accepted'; exit 1; fi
done
echo 'official sing-box and Xray REALITY + Vision forwarding and wrong short-ID rejection passed'
`
	write("test.sh", []byte(script))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := fmt.Sprintf("xingdu-reality-%d", time.Now().UnixNano())
	defer exec.Command("docker", "rm", "-f", name).Run()
	output, err := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none", "--cap-add", "NET_ADMIN", "--platform", "linux/arm64", "-v", runtimeDir+":/runtimes:ro", "-v", dir+":/configs:ro", "--entrypoint", "sh", "xingdu-lab-amazon:latest", "/configs/test.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("REALITY forwarding: %v\n%s", err, output)
	}
	t.Log(string(output))
}
