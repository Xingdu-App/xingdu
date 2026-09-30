package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSELinuxDetectionAndDisabledHost(t *testing.T) {
	x, c, calls := executorFixture(t)
	x.selinuxEnforcePath = filepath.Join(t.TempDir(), "enforce")
	if enabled, err := x.selinuxEnabled(); enabled || err != nil {
		t.Fatal(enabled, err)
	}
	// A host without SELinux retains the ordinary deployment lifecycle.
	task := taskFixture(t)
	if r := x.apply(context.Background(), c, task); !r.Success {
		t.Fatal(r)
	}
	before := len(*calls)
	if code := x.repairOwnedRuntimePolicy(context.Background(), task.DeploymentID); code != "" || len(*calls) != before {
		t.Fatal(code, *calls)
	}
	for _, mode := range []string{"0", "1"} {
		os.WriteFile(x.selinuxEnforcePath, []byte(mode), 0600)
		if enabled, err := x.selinuxEnabled(); !enabled || err != nil {
			t.Fatal(enabled, err)
		}
	}
	os.WriteFile(x.selinuxEnforcePath, []byte("unexpected"), 0600)
	if _, err := x.selinuxEnabled(); err == nil {
		t.Fatal("unknown mode accepted")
	}
}
func TestSELinuxFailureStopsDeploymentBeforeStartingService(t *testing.T) {
	x, c, calls := executorFixture(t)
	x.selinuxEnforcePath = filepath.Join(t.TempDir(), "enforce")
	os.WriteFile(x.selinuxEnforcePath, []byte("1"), 0600)
	task := taskFixture(t)
	// The embedded policy may only label fixed owned paths. Test paths cannot
	// accidentally cause policy installation or root filesystem relabeling.
	r := x.apply(context.Background(), c, task)
	if r.Success || r.Code != "selinux_policy_failed" {
		t.Fatal(r)
	}
	for _, call := range *calls {
		if strings.Contains(call, "enable --now") || strings.Contains(call, "semodule") {
			t.Fatal(call)
		}
	}
	if _, err := os.Stat(filepath.Join(x.stateDir, task.DeploymentID)); !os.IsNotExist(err) {
		t.Fatal("failed candidate retained", err)
	}
}
func TestSELinuxDetectionFailureDoesNotStartOrChangePolicy(t *testing.T) {
	x, c, calls := executorFixture(t)
	x.selinuxEnforcePath = t.TempDir() // read error, not a disabled host
	r := x.apply(context.Background(), c, taskFixture(t))
	if r.Code != "selinux_detection_failed" || r.Success {
		t.Fatal(r)
	}
	for _, call := range *calls {
		if strings.Contains(call, "enable --now") || strings.Contains(call, "restorecon") {
			t.Fatal(call)
		}
	}
}

// Opt-in real SELinux host acceptance. The caller must name an existing owned
// node. This checks the same repair path as restart, without restarting traffic.
func TestSELinuxRepairOnOwnedNode(t *testing.T) {
	id := os.Getenv("XINGDU_SELINUX_TEST_NODE_ID")
	if id == "" {
		t.Skip("requires an explicitly selected owned node on a SELinux VM")
	}
	x := newProtocolExecutor()
	enabled, err := x.selinuxEnabled()
	if err != nil || !enabled || !x.root || !x.owned(id) {
		t.Fatal("requires root, SELinux and an owned node")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if code := x.repairOwnedRuntimePolicy(context.Background(), id); code != "" {
			t.Fatal(code)
		}
	}
	if !x.runtimePolicyReady(context.Background(), id) {
		t.Fatal("service must already be running in the dedicated domain")
	}
	if got := x.serviceStatus(context.Background(), id); got != "active" {
		t.Fatal(got)
	}
}
