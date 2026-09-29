package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/machine"
)

// UpgradeExpectation is delivered through private stdin, never command arguments.
type UpgradeExpectation struct {
	Server    string `json:"server"`
	TokenHash string `json:"token_hash"`
	Mode      string `json:"mode"`
	Version   string `json:"version"`
}

type upgradePaths struct{ binary, unit, config, source string }
type upgradeCommand func(context.Context, string, ...string) ([]byte, error)

func Upgrade(ctx context.Context, expected UpgradeExpectation) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return errors.New("upgrade requires Linux and root")
	}
	source, err := os.Executable()
	if err != nil {
		return errors.New("upgrade source unavailable")
	}
	err = upgrade(ctx, expected, upgradePaths{"/usr/local/bin/xingdu-agent", "/etc/systemd/system/xingdu-agent.service", StateDir + "/agent.json", source}, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	})
	if err != nil {
		return err
	}
	c, err := Load(StateDir + "/agent.json")
	if err != nil || c.Server != expected.Server || c.Mode != expected.Mode || machine.Hash(c.Token) != expected.TokenHash || c.EnrollmentToken != "" {
		return errors.New("updater identity changed")
	}
	return installUpdater(ctx, c)
}

func upgrade(ctx context.Context, expected UpgradeExpectation, p upgradePaths, run upgradeCommand) error {
	fail := func() error { return errors.New("upgrade preflight failed; existing installation retained") }
	if expected.Version != machine.Version || !machine.ValidToken(expected.TokenHash) {
		return fail()
	}
	c, err := Load(p.config)
	if err != nil || c.EnrollmentToken != "" || c.Server != expected.Server || c.Mode != expected.Mode || machine.Hash(c.Token) != expected.TokenHash {
		return fail()
	}
	for _, path := range []string{p.binary, p.unit, p.config, p.source} {
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
			return fail()
		}
		if path != p.config || c.Mode == "manage" {
			owner, ok := st.Sys().(*syscall.Stat_t)
			if !ok || owner.Uid != uint32(os.Geteuid()) {
				return fail()
			}
		}
	}
	unit, err := os.ReadFile(p.unit)
	if err != nil {
		return fail()
	}
	user := "root"
	if c.Mode == "monitor" {
		user = "xingdu-agent"
	}
	if string(unit) != ServiceUnit(user, c.Mode) {
		return fail()
	}
	fragment, err := run(ctx, "systemctl", "show", "xingdu-agent.service", "-p", "FragmentPath", "--value")
	if err != nil || strings.TrimSpace(string(fragment)) != p.unit {
		return fail()
	}
	dropins, err := run(ctx, "systemctl", "show", "xingdu-agent.service", "-p", "DropInPaths", "--value")
	if err != nil || strings.TrimSpace(string(dropins)) != "" {
		return fail()
	}
	old, err := run(ctx, p.binary, "--version")
	version := strings.TrimPrefix(strings.TrimSpace(string(old)), "xingdu-agent ")
	if err != nil || !machine.VersionAtLeast(machine.Version, version) || machine.VersionAtLeast(version, machine.Version) {
		return fail()
	}
	// The new executable checks the existing private identity before replacing
	// anything. No configuration, service unit or protocol state is rewritten.
	replacement, err := copyUpgradeFile(p.source, filepath.Dir(p.binary))
	if err != nil {
		return fail()
	}
	defer os.Remove(replacement)
	backup, err := copyUpgradeFile(p.binary, filepath.Dir(p.binary))
	if err != nil {
		return fail()
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			os.Remove(backup)
		}
	}()
	if err = os.Rename(replacement, p.binary); err != nil {
		return fail()
	}
	rollback := func() error {
		if os.Rename(backup, p.binary) != nil {
			keepBackup = true
			return errors.New("upgrade failed; rollback failed")
		}
		recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := run(recovery, "systemctl", "restart", "xingdu-agent.service"); err != nil {
			return errors.New("upgrade rolled back; service recovery failed")
		}
		return errors.New("upgrade failed; previous executable restored")
	}
	if _, err = run(ctx, "systemctl", "restart", "xingdu-agent.service"); err != nil {
		return rollback()
	}
	// Type=simple reports activation before a process can fail. Observe it across
	// a short window; the control plane separately requires a fresh target heartbeat.
	for i := 0; i < 3; i++ {
		if _, err = run(ctx, "systemctl", "is-active", "--quiet", "xingdu-agent.service"); err != nil {
			return rollback()
		}
		select {
		case <-ctx.Done():
			return rollback()
		case <-time.After(time.Second):
		}
	}
	return nil
}

func copyUpgradeFile(source, dir string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, ".xingdu-upgrade-*")
	if err != nil {
		return "", err
	}
	name := out.Name()
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return "", err
	}
	if err = out.Chmod(0755); err != nil {
		return "", err
	}
	if err = out.Sync(); err != nil {
		return "", err
	}
	if err = out.Close(); err != nil {
		return "", err
	}
	ok = true
	return name, nil
}
