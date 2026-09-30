package bootstrap

import (
	"errors"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Fixed exit codes distinguish prerequisites without storing remote shell output.
const installPreflight = `command -v sh >/dev/null 2>&1 || exit 71
command -v systemctl >/dev/null 2>&1 || exit 72
test "$(id -u)" = 0 || exit 73
if ! command -v tar >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get -o Acquire::Retries=0 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15 update >/dev/null 2>&1 &&
      apt-get -o Acquire::Retries=0 -o Acquire::http::Timeout=15 -o Acquire::https::Timeout=15 install -y --no-install-recommends tar >/dev/null 2>&1 || exit 75
  elif command -v dnf >/dev/null 2>&1; then
    dnf -y --setopt=timeout=15 --setopt=retries=0 install tar >/dev/null 2>&1 || exit 75
  elif command -v yum >/dev/null 2>&1; then
    yum -y --setopt=timeout=15 --setopt=retries=0 install tar >/dev/null 2>&1 || exit 75
  elif command -v zypper >/dev/null 2>&1; then
    zypper --non-interactive install --no-recommends tar >/dev/null 2>&1 || exit 75
  else
    exit 70
  fi
  command -v tar >/dev/null 2>&1 || exit 75
fi`

func privilegedCommand(user, command string) string {
	if user != "root" {
		return "sudo -n sh -c " + shellQuote(command)
	}
	return "sh -c " + shellQuote(command)
}

func preflightCommand(user string) string {
	command := privilegedCommand(user, installPreflight)
	if user != "root" {
		return "sudo -n true >/dev/null 2>&1 || exit 74; " + command
	}
	return command
}

func preflightError(err error) error {
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitStatus() {
		case 70:
			return errors.New("ssh_tar_missing")
		case 75:
			return errors.New("ssh_tar_install_failed")
		case 71:
			return errors.New("ssh_shell_missing")
		case 72:
			return errors.New("ssh_systemd_missing")
		case 73:
			return errors.New("ssh_root_required")
		case 74:
			return errors.New("ssh_sudo_required")
		}
	}
	return errors.New("ssh_preflight_failed")
}

// Only exact, known installer messages become persisted codes. Never return raw
// stderr: it may contain credentials, URLs, or arbitrary output from a host.
func installationError(err error, stderr string, upgrading bool) error {
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		switch exit.ExitStatus() {
		case 80:
			return errors.New("ssh_install_directory_failed")
		case 81:
			return errors.New("ssh_bundle_extract_failed")
		}
	}
	known := map[string]string{
		"enrollment incomplete; config retained for recovery: control plane unreachable":                                       "agent_enrollment_unreachable",
		"enrollment incomplete; config retained for recovery: control plane rejected request":                                  "agent_enrollment_rejected",
		"enrollment incomplete; config retained for recovery: agent authorization unavailable; request a new enrollment token": "agent_enrollment_revoked",
		"agent state directory already exists; inspect the existing installation before replacing it":                          "agent_installation_exists",
		"agent installation files already exist; inspect before replacing":                                                     "agent_installation_exists",
		"could not create agent state directory":                                                                               "agent_state_directory_failed",
		"systemd reload failed":                                                                                                "agent_systemd_reload_failed",
		"service start failed; inspect journalctl -u xingdu-agent on VPS":                                                      "agent_service_start_failed",
		"systemd is required; use --init and foreground mode on other systems":                                                 "ssh_systemd_missing",
		"installation requires Linux and root (or passwordless sudo)":                                                          "ssh_root_required",
		"could not create dedicated agent user":                                                                                "agent_user_creation_failed",
	}
	for _, line := range strings.Split(stderr, "\n") {
		if code, ok := known[strings.TrimSpace(line)]; ok {
			return errors.New(code)
		}
	}
	if upgrading {
		return errors.New("upgrade_failed_check_vps")
	}
	return errors.New("install_failed_check_vps")
}

func runInstallation(client *ssh.Client, command string, input io.Reader, upgrading bool) error {
	session, err := client.NewSession()
	if err != nil {
		return errors.New("ssh_install_session_failed")
	}
	defer session.Close()
	var stderr limitedOutput
	defer func() { clear(stderr.buf.Bytes()) }()
	session.Stdin, session.Stdout, session.Stderr = input, io.Discard, &stderr
	err = session.Run(command)
	if err == nil {
		return nil
	}
	return installationError(err, stderr.buf.String(), upgrading)
}
