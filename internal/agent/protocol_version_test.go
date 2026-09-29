package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestInstalledRuntimeVersion(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	if got := x.runtimeVersion(task.DeploymentID); got != "" {
		t.Fatal("uninstalled runtime", got)
	}
	if r := x.apply(context.Background(), c, task); !r.Success {
		t.Fatal(r.Code)
	}
	if got := x.runtimeVersion(task.DeploymentID); got != protocol.RuntimeVersion {
		t.Fatal("installed runtime", got)
	}
	// Existing nodes may still use an older version after upgrading the Agent.
	old := filepath.Join(x.binaryDir, "sing-box-1.13.0")
	if err := os.WriteFile(old, []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(x.unitDir, serviceName(task.DeploymentID))
	writeUnit := func(binary string) {
		t.Helper()
		if err := os.WriteFile(unit, []byte(protocolUnit(task.DeploymentID, binary, "unused")), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeUnit(old)
	if got := x.runtimeVersion(task.DeploymentID); got != "1.13.0" {
		t.Fatal("used Agent build instead of installed version", got)
	}
	for _, path := range []string{filepath.Join(x.binaryDir, "sing-box-not-a-version"), filepath.Join(t.TempDir(), "sing-box-1.13.0")} {
		writeUnit(path)
		if got := x.runtimeVersion(task.DeploymentID); got != "" {
			t.Fatal("unsafe path reported", got)
		}
	}
	writeUnit(old)
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if got := x.runtimeVersion(task.DeploymentID); got != "" {
		t.Fatal("missing binary reported", got)
	}
	if err := os.Symlink(filepath.Join(x.binaryDir, "sing-box-"+protocol.RuntimeVersion), old); err != nil {
		t.Fatal(err)
	}
	if got := x.runtimeVersion(task.DeploymentID); got != "" {
		t.Fatal("symlink reported", got)
	}
}
