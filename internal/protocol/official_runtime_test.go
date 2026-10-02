package protocol

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Check managed configurations against unchanged official executables. Xray's
// Agent helper still supplies its private backend and frontend topology; this
// validates config acceptance, not native XHTTP forwarding or all 72 handshakes.
func TestOfficialManagedRuntimeConfigurations(t *testing.T) {
	assets := os.Getenv("XINGDU_TEST_OFFICIAL_RUNTIME_DIR")
	if assets == "" {
		t.Skip("requires official arm64 assets and the new Agent")
	}
	dir := t.TempDir()
	var script strings.Builder
	script.WriteString("set -eu\nmkdir /tmp/runtimes\ncp /assets/agent-linux-arm64 /tmp/runtimes/xray-" + XrayVersion + "\ncp /assets/xray-core-" + XrayVersion + " /tmp/runtimes/\n")
	for i, s := range matrixSpecs(t) {
		data, err := Render(s)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("case-%02d.json", i+1)
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		binary := "/assets/sing-box-linux-arm64"
		if s.UsesXray() {
			binary = "/tmp/runtimes/xray-" + XrayVersion
		}
		fmt.Fprintf(&script, "%s check -c /configs/%s\n", binary, name)
	}
	script.WriteString("/assets/agent-linux-arm64 --version\nprintf '72 official managed configs accepted\\n'\n")
	if err := os.WriteFile(filepath.Join(dir, "check.sh"), []byte(script.String()), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", "--platform", "linux/arm64", "-v", assets+":/assets:ro", "-v", dir+":/configs:ro", "--entrypoint", "sh", "xingdu-lab-amazon:latest", "/configs/check.sh")
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("official config check: %v\n%s", err, result)
	}
	t.Log(string(result))
}
