package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/machine"
)

const updaterDir = "/etc/xingdu-agent"
const updaterConfig = updaterDir + "/updater.json"
const updaterUnit = `[Unit]
Description=Xingdu authorized agent updater
After=network-online.target
[Service]
Type=oneshot
User=root
UMask=0077
ExecStart=/usr/local/bin/xingdu-agent --self-update
TimeoutStartSec=240
StandardOutput=null
StandardError=journal
`
const updaterTimer = `[Unit]
Description=Check for authorized Xingdu agent upgrades
[Timer]
OnBootSec=30s
OnUnitInactiveSec=30s
AccuracySec=5s
[Install]
WantedBy=timers.target
`

func rootOwned(path string, directory bool) bool {
	st, err := os.Lstat(path)
	if err != nil {
		return false
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	return ok && owner.Uid == 0 && st.Mode().Perm()&0022 == 0 && ((directory && st.IsDir()) || (!directory && st.Mode().IsRegular()))
}

// The monitor user cannot edit this trust anchor or either root unit. In
// particular, never derive a root download origin from monitor-writable state.
func installUpdater(ctx context.Context, c Config) error {
	if os.Geteuid() != 0 || c.EnrollmentToken != "" {
		return errors.New("updater requires an enrolled root installation")
	}
	if err := os.Mkdir(updaterDir, 0700); err != nil && !os.IsExist(err) {
		return errors.New("updater directory unavailable")
	}
	if !rootOwned(updaterDir, true) {
		return errors.New("unsafe updater directory")
	}
	if _, err := os.Lstat(updaterConfig); err == nil {
		if !rootOwned(updaterConfig, false) {
			return errors.New("unsafe updater configuration")
		}
		old, err := Load(updaterConfig)
		if err != nil || old != c {
			return errors.New("updater identity mismatch")
		}
	} else if !os.IsNotExist(err) {
		return errors.New("updater configuration unavailable")
	} else if err = save(updaterConfig, c, true); err != nil {
		return err
	}
	for name, body := range map[string]string{"xingdu-agent-update.service": updaterUnit, "xingdu-agent-update.timer": updaterTimer} {
		path := filepath.Join("/etc/systemd/system", name)
		if _, err := os.Lstat(path); err == nil {
			old, err := os.ReadFile(path)
			if err != nil || !rootOwned(path, false) || string(old) != body {
				return errors.New("custom updater unit requires manual review")
			}
		} else if !os.IsNotExist(err) {
			return errors.New("updater unit unavailable")
		} else if err = os.WriteFile(path, []byte(body), 0644); err != nil {
			return err
		}
	}
	if err := exec.CommandContext(ctx, "systemctl", "daemon-reload").Run(); err != nil {
		return errors.New("updater service reload failed")
	}
	if err := exec.CommandContext(ctx, "systemctl", "enable", "--now", "xingdu-agent-update.timer").Run(); err != nil {
		return errors.New("updater timer start failed")
	}
	return nil
}

func SelfUpdate(ctx context.Context) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || !rootOwned(updaterDir, true) || !rootOwned(updaterConfig, false) {
		return errors.New("updater requires a trusted root installation")
	}
	c, err := Load(updaterConfig)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 210*time.Second)
	defer cancel()
	return selfUpdate(ctx, c, func(ctx context.Context, path string, expected UpgradeExpectation) error {
		b, _ := json.Marshal(expected)
		defer clear(b)
		cmd := exec.CommandContext(ctx, path, "--upgrade")
		cmd.Stdin = bytes.NewReader(b)
		// Neither a credential nor expectation goes into argv or logs.
		if cmd.Run() != nil {
			return errors.New("upgrade execution failed")
		}
		return nil
	}, "/usr/local/bin")
}

func selfUpdate(ctx context.Context, c Config, apply func(context.Context, string, UpgradeExpectation) error, dir string) error {
	fail := func() error { return errors.New("self update failed; inspect installation or retry from console") }
	req, err := http.NewRequestWithContext(ctx, "POST", c.Server+"/api/v1/agent/update/claim", strings.NewReader("{}"))
	if err != nil {
		return fail()
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return fail()
	}
	var envelope struct {
		Data *machine.UpdateTask `json:"data"`
	}
	err = json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&envelope)
	res.Body.Close()
	if res.StatusCode != 200 || err != nil {
		return fail()
	}
	t := envelope.Data
	if t == nil {
		return nil
	}
	if !id.Valid("job", t.ID) || !id.Valid("lease", t.Lease) {
		return fail()
	}
	report := func(phase string) error {
		return request(ctx, c, "/api/v1/agent/update/"+phase, map[string]string{"id": t.ID, "lease": t.Lease}, true)
	}
	success := false
	defer func() {
		if !success {
			_ = report("failed")
		}
	}()
	if t.Arch != runtime.GOARCH || !machine.ValidToken(t.SHA256) || !machine.VersionAtLeast(t.Version, machine.Version) || machine.VersionAtLeast(machine.Version, t.Version) {
		return fail()
	}
	stage, err := os.MkdirTemp(dir, ".xingdu-self-update-")
	if err != nil {
		return fail()
	}
	defer os.RemoveAll(stage)
	path := filepath.Join(stage, "agent")
	req, err = http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/agent/download/"+t.Arch, nil)
	if err != nil {
		return fail()
	}
	res, err = client.Do(req)
	if err != nil {
		return fail()
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fail()
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail()
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, (128<<20)+1))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err != nil || syncErr != nil || closeErr != nil || n == 0 || n > 128<<20 || hex.EncodeToString(h.Sum(nil)) != t.SHA256 {
		return fail()
	}
	if os.Chmod(path, 0700) != nil {
		return fail()
	}
	// Check after the potentially slow download, immediately before executing.
	if report("check") != nil {
		return fail()
	}
	if apply(ctx, path, UpgradeExpectation{Server: c.Server, TokenHash: machine.Hash(c.Token), Mode: c.Mode, Version: t.Version}) != nil {
		return fail()
	}
	if report("applied") != nil {
		return fail()
	}
	success = true
	return nil
}
