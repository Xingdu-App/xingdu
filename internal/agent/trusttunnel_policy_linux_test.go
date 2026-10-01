//go:build linux

package agent

import (
	"os"
	"testing"
)

func TestTrustTunnelRealBPFQuery(t *testing.T) {
	path := os.Getenv("XINGDU_TEST_BPF_CGROUP")
	if path == "" {
		t.Skip("requires an isolated systemd service with IPAddressDeny")
	}
	for i := 0; i < 100; i++ {
		if !hasEgressFilter(path) {
			t.Fatal("service egress filter missing")
		}
	}
	if hasEgressFilter(path + "/missing") {
		t.Fatal("missing cgroup accepted")
	}
}
