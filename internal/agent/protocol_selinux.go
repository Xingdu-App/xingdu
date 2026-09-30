package agent

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed selinux/xingdu_runtime.te
var runtimePolicyTE []byte

//go:embed selinux/xingdu_runtime.fc
var runtimePolicyFC []byte

// Repair is only called from explicitly authorized deploy/update/restart tasks.
// Inventory reporting never installs packages, relabels files or restarts units.
func (x *protocolExecutor) selinuxEnabled() (bool, error) {
	path := x.selinuxEnforcePath
	if path == "" {
		path = "/sys/fs/selinux/enforce"
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if v := strings.TrimSpace(string(b)); v != "0" && v != "1" {
		return false, errors.New("invalid SELinux mode")
	}
	return true, nil
}
func (x *protocolExecutor) commandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	if x.output != nil {
		return x.output(ctx, name, args...)
	}
	return exec.CommandContext(ctx, name, args...).Output()
}
func (x *protocolExecutor) ensureRuntimePolicy(ctx context.Context, binary, config string) string {
	enabled, err := x.selinuxEnabled()
	if err != nil {
		return "selinux_detection_failed"
	}
	if !enabled {
		return ""
	}
	// Embedded file contexts are deliberately fixed, not tenant-selected paths.
	if x.binaryDir != "/usr/local/lib/xingdu" || x.stateDir != "/var/lib/xingdu-agent/protocols" || filepath.Dir(binary) != x.binaryDir || !strings.HasPrefix(filepath.Base(binary), "sing-box-") || filepath.Base(config) != "config.json" || filepath.Dir(filepath.Dir(config)) != x.stateDir {
		return "selinux_policy_failed"
	}
	dir := filepath.Join(filepath.Dir(x.stateDir), "selinux")
	if secureDir(dir) != nil {
		return "unsafe_state"
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(append(append([]byte{}, runtimePolicyTE...), runtimePolicyFC...)))
	stamp := filepath.Join(dir, "installed.sha256")
	previous, _ := os.ReadFile(stamp)
	modules, moduleErr := x.commandOutput(ctx, "semodule", "-l")
	installed := false
	for _, line := range strings.Split(string(modules), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "xingdu_runtime" {
			installed = true
		}
	}
	if string(previous) != digest || moduleErr != nil || !installed {
		// Only packages from the host's already configured repositories.
		// Never add repositories or disable package signature checks.
		// Installing the development tools does not change SELinux enforcement.
		if _, e := os.Stat("/usr/share/selinux/devel/Makefile"); e != nil {
			manager := ""
			for _, candidate := range []string{"dnf", "yum"} {
				if _, e := exec.LookPath(candidate); e == nil {
					manager = candidate
					break
				}
			}
			if manager == "" {
				return "selinux_tools_missing"
			}
			if x.run(ctx, manager, "-y", "install", "selinux-policy-devel", "policycoreutils") != nil {
				return "selinux_tools_install_failed"
			}
		}
		for name, data := range map[string][]byte{"xingdu_runtime.te": runtimePolicyTE, "xingdu_runtime.fc": runtimePolicyFC} {
			if atomicProtocolFile(filepath.Join(dir, name), data, 0600) != nil {
				return "selinux_policy_failed"
			}
		}
		if x.run(ctx, "make", "-f", "/usr/share/selinux/devel/Makefile", "-C", dir, "xingdu_runtime.pp") != nil || x.run(ctx, "semodule", "-i", filepath.Join(dir, "xingdu_runtime.pp")) != nil {
			return "selinux_policy_failed"
		}
		if atomicProtocolFile(stamp, []byte(digest), 0600) != nil {
			return "selinux_policy_failed"
		}
	}
	// Respect administrator overrides: never replace local fcontext mappings.
	if x.run(ctx, "restorecon", binary, config) != nil {
		return "selinux_label_failed"
	}
	for path, expected := range map[string]string{binary: "xingdu_runtime_exec_t", config: "xingdu_runtime_conf_t"} {
		label, e := x.commandOutput(ctx, "stat", "-c", "%C", "--", path)
		parts := strings.Split(strings.TrimSpace(string(label)), ":")
		if e != nil || len(parts) < 3 || parts[2] != expected {
			return "selinux_label_conflict"
		}
	}
	return ""
}
func (x *protocolExecutor) runtimePolicyReady(ctx context.Context, id string) bool {
	enabled, err := x.selinuxEnabled()
	if err != nil {
		return false
	}
	if !enabled {
		return true
	}
	pidBytes, err := x.commandOutput(ctx, "systemctl", "show", serviceName(x.localDeploymentID(id)), "-p", "MainPID", "--value")
	if err != nil {
		return false
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidBytes)), 10, 32)
	if err != nil || pid == 0 {
		return false
	}
	label, err := os.ReadFile(fmt.Sprintf("/proc/%d/attr/current", pid))
	if err != nil {
		return false
	}
	parts := strings.Split(strings.TrimSpace(strings.TrimRight(string(label), "\x00")), ":")
	return len(parts) >= 3 && parts[2] == "xingdu_runtime_t"
}
func (x *protocolExecutor) repairOwnedRuntimePolicy(ctx context.Context, id string) string {
	enabled, err := x.selinuxEnabled()
	if err != nil {
		return "selinux_detection_failed"
	}
	if !enabled {
		return ""
	}
	version := x.runtimeVersion(id)
	if version == "" {
		return "runtime_unavailable"
	}
	return x.ensureRuntimePolicy(ctx, filepath.Join(x.binaryDir, "sing-box-"+version), filepath.Join(x.stateDir, x.localDeploymentID(id), "config.json"))
}
