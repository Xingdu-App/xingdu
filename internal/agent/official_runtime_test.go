package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestOfficialRuntimeRequiresVerifiedPacketFilter(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	x.egressFilter = func(string) bool { return false }
	result := x.apply(context.Background(), c, task)
	if result.Success || result.Code != "runtime_policy_failed" {
		t.Fatal("unguarded upstream runtime started", result)
	}
	if _, err := os.Stat(filepath.Join(x.unitDir, serviceName(task.DeploymentID))); !os.IsNotExist(err) {
		t.Fatal("failed runtime not rolled back", err)
	}
}
func TestOfficialRuntimeUnitAndLegacyIsolation(t *testing.T) {
	for _, name := range []string{"sing-box-" + protocol.RuntimeVersion, "xray-" + protocol.XrayVersion} {
		unit := protocolUnit("fixture", "/runtime/"+name, "/state/config")
		for _, deny := range []string{"IPAddressDeny=", "10.0.0.0/8", "169.254.0.0/16", "168.63.129.16/32", "fc00::/7", "::ffff:0:0/96"} {
			if !strings.Contains(unit, deny) {
				t.Fatal("missing packet guard", name, deny)
			}
		}
	}
	legacy := protocolUnit("fixture", "/runtime/sing-box-"+protocol.HardenedRuntimeVersion, "/state/config")
	if strings.Contains(legacy, "official runtime") {
		t.Fatal("legacy unit reclassified")
	}
}
