package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

// Retain a root-only rollback copy until the replacement is running. Never
// overwrite a backup left by a crash; that uncertainty requires recovery.
func (x *protocolExecutor) update(ctx context.Context, c Config, t protocol.Task) string {
	localID := x.localDeploymentID(t.DeploymentID)
	if !x.ownedRuntime(localID) {
		return "ownership_mismatch"
	}
	if protocol.ValidateSpec(t.Spec) != nil {
		return "invalid_spec"
	}
	if t.Spec.Input.RequiredAgentVersion() == "0.17.0-dev" && !x.requiresEgressFilter(t.DeploymentID) {
		return "runtime_change_requires_new_node"
	}
	dir := filepath.Join(x.stateDir, localID)
	path := filepath.Join(dir, "config.json")
	backup := filepath.Join(dir, "config.previous.json")
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		return "interrupted"
	}
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return "unsafe_state"
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return "write_failed"
	}
	defer clear(old)
	var current struct {
		Runtime   string `json:"runtime"`
		Endpoints []struct {
			Port int `json:"listen_port"`
		} `json:"endpoints"`
		Inbounds []struct {
			Port int `json:"listen_port"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(old, &current) != nil || (len(current.Inbounds) == 0 && len(current.Endpoints) != 1) || (len(current.Inbounds) > 0 && len(current.Endpoints) > 0) {
		return "unsafe_state"
	}
	if (current.Runtime == "trusttunnel") != (t.Spec.Protocol == "trusttunnel") || (current.Runtime == "xray") != t.Spec.UsesXray() {
		return "invalid_spec"
	}
	oldPort := 0
	if len(current.Inbounds) > 0 {
		oldPort = current.Inbounds[0].Port
	} else {
		oldPort = current.Endpoints[0].Port
	}
	if oldPort != t.Spec.Port && !portAvailable(t.Spec) {
		return "port_in_use"
	}

	config, err := protocol.Render(t.Spec)
	if err != nil {
		return "invalid_spec"
	}
	defer clear(config)
	candidate := filepath.Join(dir, "config.next.json")
	defer os.Remove(candidate)
	if atomicProtocolFile(candidate, config, 0600) != nil {
		return "write_failed"
	}
	binary, err := x.specBinary(ctx, c, t.Spec)
	if err != nil {
		return "runtime_unavailable"
	}
	if x.run(ctx, binary, "check", "-c", candidate) != nil {
		return "config_rejected"
	}
	if atomicProtocolFile(backup, old, 0600) != nil {
		return "write_failed"
	}
	if os.Rename(candidate, path) != nil {
		os.Remove(backup)
		return "write_failed"
	}
	if code := x.ensureRuntimePolicy(ctx, binary, path); code != "" {
		if atomicProtocolFile(path, old, 0600) != nil || x.run(ctx, "restorecon", path) != nil {
			return "rollback_failed"
		}
		os.Remove(backup)
		return code
	}
	healthy := func(ctx context.Context) bool {
		if x.run(ctx, "systemctl", "restart", serviceName(localID)) != nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
		if x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(localID)) != nil {
			return false
		}
		if !x.runtimePolicyReady(ctx, t.DeploymentID) {
			if x.requiresEgressFilter(t.DeploymentID) {
				_ = x.run(ctx, "systemctl", "stop", serviceName(localID))
			}
			return false
		}
		return true
	}
	if healthy(ctx) {
		if os.Remove(backup) != nil {
			return "interrupted"
		}
		return "updated"
	}
	recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if atomicProtocolFile(path, old, 0600) != nil || x.ensureRuntimePolicy(recovery, binary, path) != "" || !healthy(recovery) {
		return "rollback_failed"
	}
	if os.Remove(backup) != nil {
		return "rollback_failed"
	}
	return "update_rolled_back"
}
