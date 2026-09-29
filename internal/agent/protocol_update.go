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
		Inbounds []struct {
			Port int `json:"listen_port"`
		} `json:"inbounds"`
	}
	if json.Unmarshal(old, &current) != nil || len(current.Inbounds) != 1 {
		return "unsafe_state"
	}
	if current.Inbounds[0].Port != t.Spec.Port && !portAvailable(t.Spec) {
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
	binary, err := x.binary(ctx, c)
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
	healthy := func(ctx context.Context) bool {
		if x.run(ctx, "systemctl", "restart", serviceName(localID)) != nil {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
		return x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(localID)) == nil
	}
	if healthy(ctx) {
		if os.Remove(backup) != nil {
			return "interrupted"
		}
		return "updated"
	}
	recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if atomicProtocolFile(path, old, 0600) != nil || !healthy(recovery) {
		return "rollback_failed"
	}
	if os.Remove(backup) != nil {
		return "rollback_failed"
	}
	return "update_rolled_back"
}
