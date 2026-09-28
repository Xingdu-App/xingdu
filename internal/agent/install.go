package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"xingdu.app/xingdu/internal/machine"
)

const StateDir = "/var/lib/xingdu-agent"

func Install(ctx context.Context, server, mode, token string) error {
	if machine.Origin(server) != nil || !machine.ValidMode(mode) || !machine.ValidToken(token) {
		return errors.New("invalid installation input")
	}
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return errors.New("installation requires Linux and root (or passwordless sudo)")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemd is required; use --init and foreground mode on other systems")
	}
	if _, err := os.Lstat(StateDir); !os.IsNotExist(err) {
		return errors.New("agent state directory already exists; inspect the existing installation before replacing it")
	}
	for _, path := range []string{"/usr/local/bin/xingdu-agent", "/etc/systemd/system/xingdu-agent.service"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return errors.New("agent installation files already exist; inspect before replacing")
		}
	}
	uid, gid := 0, 0
	serviceUser := "root"
	if mode == "monitor" {
		serviceUser = "xingdu-agent"
		u, err := user.Lookup(serviceUser)
		if err != nil {
			if e := exec.CommandContext(ctx, "useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", serviceUser).Run(); e != nil {
				return errors.New("could not create dedicated agent user")
			}
			u, err = user.Lookup(serviceUser)
		}
		if err != nil {
			return errors.New("agent user unavailable")
		}
		uid, err = strconv.Atoi(u.Uid)
		if err != nil || uid <= 0 {
			return errors.New("dedicated agent user must not be root")
		}
		gid, err = strconv.Atoi(u.Gid)
		if err != nil || gid <= 0 {
			return errors.New("dedicated agent group must not be root")
		}
	}
	if err := os.Mkdir(StateDir, 0700); err != nil {
		return errors.New("could not create agent state directory")
	}
	config := filepath.Join(StateDir, "agent.json")
	if err := Initialize(ctx, config, server, mode, token); err != nil {
		return fmt.Errorf("enrollment incomplete; config retained for recovery: %w", err)
	}
	if err := os.Chown(config, uid, gid); err != nil {
		return err
	}
	if err := os.Chown(StateDir, uid, gid); err != nil {
		return err
	}
	source, err := os.Executable()
	if err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp("/usr/local/bin", ".xingdu-agent-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err = out.Chmod(0755); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, "/usr/local/bin/xingdu-agent"); err != nil {
		return err
	}
	unit := ServiceUnit(serviceUser, mode)
	if err = os.WriteFile("/etc/systemd/system/xingdu-agent.service", []byte(unit), 0644); err != nil {
		return err
	}
	if err = exec.CommandContext(ctx, "systemctl", "daemon-reload").Run(); err != nil {
		return errors.New("systemd reload failed")
	}
	if err = exec.CommandContext(ctx, "systemctl", "enable", "--now", "xingdu-agent.service").Run(); err != nil {
		return errors.New("service start failed; inspect journalctl -u xingdu-agent on VPS")
	}
	return nil
}
func ServiceUnit(user, mode string) string {
	s := `[Unit]
Description=Xingdu machine agent
After=network.target
[Service]
Type=simple
ExecStart=/usr/local/bin/xingdu-agent --config /var/lib/xingdu-agent/agent.json
Restart=on-failure
RestartSec=10
UMask=0077
User=` + user + `
[Install]
WantedBy=multi-user.target
`
	if mode == "monitor" {
		s += `[Service]
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
CapabilityBoundingSet=
ReadWritePaths=/var/lib/xingdu-agent
`
	}
	return s
}
