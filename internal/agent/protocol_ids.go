package agent

import (
	"os"
	"path/filepath"
	"strings"
	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

// Legacy UUID filenames are a local compatibility detail only. Renaming a live
// directory would break the systemd unit's StandardInput path on its next start.
// New remote IDs never accept UUIDs, and new installations always use node_ IDs.
func legacyResourceUUID(prefix, value string) string {
	if !resourceid.Valid(prefix, value) {
		return ""
	}
	h := strings.TrimPrefix(value, prefix+"_")
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
func pathPresent(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err) // Errors are fail-closed: do not fall back to another resource.
}
func (x *protocolExecutor) localDeploymentID(node string) string {
	if !resourceid.Valid("node", node) {
		return ""
	}
	if pathPresent(filepath.Join(x.stateDir, node)) || pathPresent(filepath.Join(x.unitDir, serviceName(node))) {
		return node
	}
	legacy := legacyResourceUUID("node", node)
	if pathPresent(filepath.Join(x.stateDir, legacy)) || pathPresent(filepath.Join(x.unitDir, serviceName(legacy))) {
		return legacy
	}
	return node
}

// Database migrations deterministically map UUID operation IDs. Reuse their
// durable outcomes rather than executing a migrated operation for a second time.
func (x *protocolExecutor) readJournal(task protocol.Task, path, fingerprint string) ([]byte, string, error) {
	raw, err := os.ReadFile(path)
	if !os.IsNotExist(err) {
		return raw, fingerprint, err
	}
	legacy := task
	legacy.ID = legacyResourceUUID("op", task.ID)
	legacy.DeploymentID = legacyResourceUUID("node", task.DeploymentID)
	raw, err = os.ReadFile(filepath.Join(filepath.Dir(path), legacy.ID+".json"))
	return raw, taskFingerprint(legacy), err
}
