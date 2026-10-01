package protocol

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"
)

func TestCompatibility72PublicReferenceProjection(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("requires Python for remote acceptance tooling")
	}
	var connections []any
	var expected [][]byte
	for _, s := range matrixSpecs(t) {
		connections = append(connections, map[string]any{"protocol": s.Protocol, "server": "127.0.0.1", "port": s.Port, "server_name": s.ServerName, "credential": s.Credential, "password": s.Password, "username": Username(s.Protocol), "certificate": s.Certificate, "v2ray": s.V2Ray, "encryption": s.ClientEncryption(), "quic": s.QUIC, "obfs_password": s.ObfsPassword, "wireguard": s.WireGuardClient()})
		b := singClient(s, 21900)
		if s.V2Ray != nil && s.UsesXray() {
			var err error
			b, err = XrayClient(s, "127.0.0.1", 21900)
			if err != nil {
				t.Fatal(err)
			}
		}
		expected = append(expected, b)
	}
	input, _ := json.Marshal(connections)
	code := `import importlib.util,json,sys
s=importlib.util.spec_from_file_location('remote72','../../tools/compatibility/remote72.py')
m=importlib.util.module_from_spec(s);s.loader.exec_module(m)
print(json.dumps([m.reference_client(c)[1] for c in json.load(sys.stdin)]))`
	cmd := exec.Command("python3", "-c", code)
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		t.Fatal("reference projection failed", err)
	}
	var actual []json.RawMessage
	if json.Unmarshal(output, &actual) != nil || len(actual) != 72 {
		t.Fatal("invalid reference output")
	}
	// Empty optional arrays/objects have the same engine defaults. Preserve
	// booleans, numbers and non-empty values, which affect wire behavior.
	var normalize func(any) any
	normalize = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "log")
			for k, y := range x {
				n := normalize(y)
				if n == nil {
					delete(x, k)
				} else {
					x[k] = n
				}
			}
			if len(x) == 0 {
				return nil
			}
			return x
		case []any:
			if len(x) == 0 {
				return nil
			}
			for i, y := range x {
				x[i] = normalize(y)
			}
			return x
		default:
			return v
		}
	}
	for i, want := range expected {
		var a, b any
		json.Unmarshal(actual[i], &a)
		json.Unmarshal(want, &b)
		if i == 6 || i == 71 {
			// Public WireGuard probes must not leak resolution to a host fake-IP
			// resolver. This DNS path is additional to the isolated IP-only lab.
			var dns any
			json.Unmarshal([]byte(`{"servers":[{"type":"https","tag":"remote","server":"1.1.1.1","path":"/dns-query","detour":"proxy","tls":{"enabled":true,"server_name":"cloudflare-dns.com"}}],"final":"remote"}`), &dns)
			if !reflect.DeepEqual(a.(map[string]any)["dns"], dns) {
				t.Fatal("WireGuard public probe must resolve DNS through its tunnel")
			}
			delete(a.(map[string]any), "dns")
		}
		if !reflect.DeepEqual(normalize(a), normalize(b)) {
			t.Fatalf("case %d public projection differs from runtime-tested client", i+1)
		}
	}
}
