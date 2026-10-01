package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"xingdu.app/xingdu/internal/protocol"
)

func (x *protocolExecutor) specBinary(ctx context.Context, c Config, s protocol.Spec) (string, error) {
	if s.Protocol != "trusttunnel" {
		return x.binary(ctx, c)
	}
	// Fail closed until this second runtime has a verified SELinux policy.
	enabled, err := x.selinuxEnabled()
	if err != nil || enabled {
		return "", errors.New("TrustTunnel SELinux policy unavailable")
	}
	if _, err = x.runtimeBinary(ctx, c, "trusttunnel-endpoint", protocol.TrustTunnelVersion, protocol.TrustTunnelLicense, protocol.TrustTunnelSHA256, "/api/v1/agent/runtime/trusttunnel/"); err != nil {
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(self)
	if err != nil {
		return "", err
	}
	path := filepath.Join(x.binaryDir, "trusttunnel-"+protocol.TrustTunnelVersion)
	if st, err := os.Lstat(path); err == nil && (!st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0) {
		return "", errors.New("unsafe launcher path")
	}
	// The helper is this trusted agent itself, copied atomically. It rechecks the
	// pinned endpoint hash before every launch, including service restarts.
	if err = atomicProtocolFile(path, b, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func (x *protocolExecutor) trustTunnelUnit(id string) bool {
	unit, err := os.ReadFile(filepath.Join(x.unitDir, serviceName(x.localDeploymentID(id))))
	if err != nil {
		return false
	}
	prefix := "ExecStart=" + filepath.Join(x.binaryDir, "trusttunnel-")
	for _, line := range strings.Split(string(unit), "\n") {
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, " run -c stdin") {
			return protocol.ValidRuntimeVersion(strings.TrimSuffix(strings.TrimPrefix(line, prefix), " run -c stdin"))
		}
	}
	return false
}
func (x *protocolExecutor) runtimePolicyFailure(id string) string {
	if x.trustTunnelUnit(id) {
		return "runtime_policy_failed"
	}
	return "selinux_domain_failed"
}
