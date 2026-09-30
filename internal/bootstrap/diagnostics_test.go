package bootstrap

import (
	"errors"
	"testing"
)

func TestInstallationErrorDiscardsRemoteSecrets(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{"agent state directory already exists; inspect the existing installation before replacing it\n", "agent_installation_exists"},
		{"systemd reload failed\n", "agent_systemd_reload_failed"},
		{"enrollment incomplete; config retained for recovery: control plane unreachable\n", "agent_enrollment_unreachable"},
		{"service start failed; inspect journalctl -u xingdu-agent on VPS\n", "agent_service_start_failed"},
		{"password=secret token=secret private-key=secret", "install_failed_check_vps"},
		{"systemd reload failed token=secret", "install_failed_check_vps"},
		{"password=secret\nsystemd reload failed\ntoken=secret", "agent_systemd_reload_failed"},
	} {
		if got := installationError(errors.New("remote failure"), tc.output, false).Error(); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
	if got := installationError(errors.New("remote failure"), "unknown secret", true).Error(); got != "upgrade_failed_check_vps" {
		t.Fatal(got)
	}
}
