package probe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestProbeRejectsUntrustedRuntimeAndEndpoint(t *testing.T) {
	p := filepath.Join(t.TempDir(), "runtime")
	os.WriteFile(p, []byte("untrusted"), 0700)
	if VerifyRuntime(p) == nil {
		t.Fatal("untrusted runtime accepted")
	}
	s, _ := protocol.NewSpec(protocol.Input{Name: "probe", Protocol: "shadowsocks", Port: 443})
	peer := protocol.Peer{Address: "93.184.216.34", Protocol: s.Protocol, Port: 443, Credential: s.Credential}
	for _, endpoint := range []string{"http://example.com", "https://user:pass@example.com", "file:///etc/passwd"} {
		if _, _, err := Run(context.Background(), p, endpoint, peer); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
