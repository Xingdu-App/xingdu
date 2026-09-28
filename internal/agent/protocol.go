package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"xingdu.app/xingdu/internal/protocol"
)

// All paths and commands are selected locally; task input cannot supply either.
type protocolExecutor struct {
	stateDir, unitDir, binaryDir string
	run                          func(context.Context, string, ...string) error
	root                         bool
	arch                         string
	client                       *http.Client
}

func newProtocolExecutor() *protocolExecutor {
	return &protocolExecutor{stateDir: filepath.Join(StateDir, "protocols"), unitDir: "/etc/systemd/system", binaryDir: "/usr/local/lib/xingdu", root: os.Geteuid() == 0 && runtime.GOOS == "linux", arch: runtime.GOARCH, client: &http.Client{Timeout: 70 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, run: func(ctx context.Context, name string, args ...string) error {
		return exec.CommandContext(ctx, name, args...).Run()
	}}
}
func secureDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0077 != 0 {
		return errors.New("unsafe state directory")
	}
	return nil
}
func atomicProtocolFile(path string, b []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".xingdu-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (x *protocolExecutor) binary(ctx context.Context, c Config) (string, error) {
	expected, ok := protocol.RuntimeSHA256[x.arch]
	if !ok {
		return "", errors.New("unsupported runtime architecture")
	}
	// Binary parents are root-owned standard directories. Refuse a symlink at
	// the owned directory rather than replacing an unrelated runtime.
	if err := os.Mkdir(x.binaryDir, 0755); err != nil && !os.IsExist(err) {
		return "", err
	}
	st, err := os.Lstat(x.binaryDir)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0022 != 0 {
		return "", errors.New("unsafe binary directory")
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) {
		return "", errors.New("unsafe binary directory owner")
	}
	// The agent runs with UMask=0077. The runtime's DynamicUser must still be
	// able to traverse this root-owned directory; creation mode alone is masked.
	if err := os.Chmod(x.binaryDir, 0755); err != nil {
		return "", err
	}
	licensePath := filepath.Join(x.binaryDir, "sing-box-LICENSE")
	if st, e := os.Lstat(licensePath); e == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
			return "", errors.New("unsafe runtime license path")
		}
	} else if !os.IsNotExist(e) {
		return "", e
	}
	if err := atomicProtocolFile(licensePath, []byte(protocol.RuntimeLicense), 0644); err != nil {
		return "", err
	}
	path := filepath.Join(x.binaryDir, "sing-box-"+protocol.RuntimeVersion)
	if st, err := os.Lstat(path); err == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
			return "", errors.New("unsafe binary")
		}
		b, err := os.ReadFile(path)
		if err == nil {
			hash := sha256.Sum256(b)
			if hex.EncodeToString(hash[:]) == expected {
				return path, nil
			}
		}
		return "", errors.New("installed runtime checksum mismatch")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/agent/runtime/"+x.arch, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := x.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("runtime download failed")
	}
	f, err := os.CreateTemp(x.binaryDir, ".runtime-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, 128<<20+1))
	if err == nil && n > 128<<20 {
		err = errors.New("runtime too large")
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != expected {
		err = errors.New("runtime checksum mismatch")
	}
	if err == nil {
		err = f.Chmod(0755)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return "", err
	}
	if ce != nil {
		return "", ce
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}
func serviceName(id string) string { return "xingdu-protocol-" + id + ".service" }
func protocolUnit(id, binary, config string) string {
	// systemd opens the root-only file before dropping privileges. The special
	// sing-box "stdin" path reads that descriptor without reopening /dev/stdin.
	return fmt.Sprintf(`# Xingdu managed deployment %s
[Unit]
Description=Xingdu protocol instance
After=network.target
[Service]
Type=simple
DynamicUser=yes
StandardInput=file:%s
ExecStart=%s run -c stdin
Restart=on-failure
RestartSec=3
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
LockPersonality=yes
MemoryDenyWriteExecute=yes
LimitNOFILE=65536
[Install]
WantedBy=multi-user.target
`, id, config, binary)
}
func portAvailable(s protocol.Spec) bool {
	addr := fmt.Sprintf(":%d", s.Port)
	if protocol.IsQUIC(s.Protocol) {
		c, e := net.ListenPacket("udp", addr)
		if e != nil {
			return false
		}
		c.Close()
		return true
	}
	l, e := net.Listen("tcp", addr)
	if e != nil {
		return false
	}
	l.Close()
	return true
}
func (x *protocolExecutor) execute(ctx context.Context, c Config, t protocol.Task) (code string) {
	if !x.root || c.Mode != "manage" {
		return "manage_required"
	}
	if !protocol.ValidID(t.ID) || !protocol.ValidID(t.DeploymentID) {
		return "invalid_task"
	}
	if t.Action != "deploy" && t.Action != "remove" && t.Action != "restart" {
		return "invalid_task"
	}
	if err := secureDir(x.stateDir); err != nil {
		return "unsafe_state"
	}
	dir := filepath.Join(x.stateDir, t.DeploymentID)
	unitPath := filepath.Join(x.unitDir, serviceName(t.DeploymentID))
	marker := filepath.Join(dir, "owner")
	if t.Action == "restart" {
		if !x.owned(t.DeploymentID) {
			return "ownership_mismatch"
		}
		if x.run(ctx, "systemctl", "restart", serviceName(t.DeploymentID)) != nil {
			return "start_failed"
		}
		select {
		case <-ctx.Done():
			return "start_failed"
		case <-time.After(2 * time.Second):
		}
		if x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(t.DeploymentID)) != nil {
			return "start_failed"
		}
		return "restarted"
	}
	if t.Action == "remove" {
		st, e := os.Lstat(dir)
		if os.IsNotExist(e) {
			if _, ue := os.Lstat(unitPath); os.IsNotExist(ue) {
				return "removed"
			}
			return "ownership_mismatch"
		}
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return "ownership_mismatch"
		}
		owner, e := os.ReadFile(marker)
		if e != nil || string(owner) != t.DeploymentID {
			return "ownership_mismatch"
		}
		unit, e := os.ReadFile(unitPath)
		if e != nil && !os.IsNotExist(e) || e == nil && !strings.HasPrefix(string(unit), "# Xingdu managed deployment "+t.DeploymentID+"\n") {
			return "ownership_mismatch"
		}
		if e == nil {
			if x.run(ctx, "systemctl", "disable", "--now", serviceName(t.DeploymentID)) != nil {
				return "stop_failed"
			}
			if os.Remove(unitPath) != nil {
				return "remove_failed"
			}
		} else {
			// A previous removal may have stopped and unlinked the unit but
			// failed daemon-reload. Retained ownership allows safe recovery.
			if x.run(ctx, "systemctl", "stop", serviceName(t.DeploymentID)) != nil {
				// An unloaded/missing unit is already stopped. Require the
				// state query to confirm it is not running before cleanup.
				stateErr := x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(t.DeploymentID))
				var exitErr *exec.ExitError
				if !errors.As(stateErr, &exitErr) || (exitErr.ExitCode() != 3 && exitErr.ExitCode() != 4) {
					return "stop_failed"
				}
			}
		}
		if x.run(ctx, "systemctl", "daemon-reload") != nil {
			return "reload_failed"
		}
		if os.RemoveAll(dir) != nil {
			return "remove_failed"
		}
		return "removed"
	}
	if protocol.ValidateSpec(t.Spec) != nil {
		return "invalid_spec"
	}
	if !portAvailable(t.Spec) {
		return "port_in_use"
	}
	if _, e := os.Lstat(unitPath); !os.IsNotExist(e) {
		return "instance_exists"
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return "instance_exists"
	}
	completed := false
	defer func() {
		if !completed {
			os.RemoveAll(dir)
		}
	}()
	if err := atomicProtocolFile(marker, []byte(t.DeploymentID), 0600); err != nil {
		return "write_failed"
	}
	binary, err := x.binary(ctx, c)
	if err != nil {
		return "runtime_unavailable"
	}
	config, err := protocol.Render(t.Spec)
	if err != nil {
		return "invalid_spec"
	}
	defer clear(config)
	configPath := filepath.Join(dir, "config.json")
	if atomicProtocolFile(configPath, config, 0600) != nil {
		return "write_failed"
	}
	if x.run(ctx, binary, "check", "-c", configPath) != nil {
		return "config_rejected"
	}
	// O_EXCL ensures we never replace a service that appeared during preparation.
	unit, err := os.OpenFile(unitPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "instance_exists"
	}
	_, err = unit.WriteString(protocolUnit(t.DeploymentID, binary, configPath))
	ce := unit.Close()
	if err != nil || ce != nil {
		if os.Remove(unitPath) != nil {
			completed = true
			return "rollback_failed"
		}
		return "write_failed"
	}
	defer func() {
		if !completed {
			cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if x.run(cleanup, "systemctl", "disable", "--now", serviceName(t.DeploymentID)) != nil {
				// Preserve the owned unit/config so a new remove operation can
				// recover; do not report successful rollback of a live service.
				completed = true
				code = "rollback_failed"
				return
			}
			if os.Remove(unitPath) != nil || x.run(cleanup, "systemctl", "daemon-reload") != nil {
				completed = true
				code = "rollback_failed"
			}
		}
	}()
	if x.run(ctx, "systemctl", "daemon-reload") != nil {
		return "reload_failed"
	}
	if x.run(ctx, "systemctl", "enable", "--now", serviceName(t.DeploymentID)) != nil {
		return "start_failed"
	}
	select {
	case <-ctx.Done():
		return "start_failed"
	case <-time.After(2 * time.Second):
	}
	if x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(t.DeploymentID)) != nil {
		return "start_failed"
	}
	completed = true
	return "deployed"
}

type protocolJournal struct {
	Fingerprint string           `json:"fingerprint"`
	Result      *protocol.Result `json:"result,omitempty"`
}

func taskFingerprint(t protocol.Task) string {
	t.Lease = ""
	b, _ := json.Marshal(t)
	h := sha256.Sum256(b)
	clear(b)
	return hex.EncodeToString(h[:])
}
func (x *protocolExecutor) apply(ctx context.Context, c Config, t protocol.Task) protocol.Result {
	result := protocol.Result{ID: t.ID, Lease: t.Lease, Code: "invalid_task"}
	if !x.root || c.Mode != "manage" {
		result.Code = "manage_required"
		return result
	}
	if !protocol.ValidID(t.ID) || !protocol.ValidID(t.DeploymentID) {
		return result
	}
	journalDir := filepath.Join(x.stateDir, "outcomes")
	if secureDir(journalDir) != nil {
		result.Code = "unsafe_state"
		return result
	}
	path := filepath.Join(journalDir, t.ID+".json")
	fingerprint := taskFingerprint(t)
	if b, e := os.ReadFile(path); e == nil {
		var old protocolJournal
		if json.Unmarshal(b, &old) != nil || old.Fingerprint != fingerprint {
			result.Code = "journal_conflict"
			return result
		}
		if old.Result != nil {
			result = *old.Result
			result.Lease = t.Lease
			return result
		}
		result.Code = "interrupted"
		return result
	} else if !os.IsNotExist(e) {
		result.Code = "journal_unavailable"
		return result
	}
	journal := protocolJournal{Fingerprint: fingerprint}
	b, _ := json.Marshal(journal)
	if atomicProtocolFile(path, b, 0600) != nil {
		result.Code = "journal_unavailable"
		return result
	}
	result.Code = x.execute(ctx, c, t)
	result.Success = result.Code == "deployed" || result.Code == "removed" || result.Code == "restarted"
	journal.Result = &result
	b, _ = json.Marshal(journal)
	if atomicProtocolFile(path, b, 0600) != nil {
		result.Success = false
		result.Code = "journal_unavailable"
	}
	return result
}
func pollProtocols(ctx context.Context, c Config) error {
	if c.Mode != "manage" {
		return nil
	}
	x := newProtocolExecutor()
	if !x.root {
		return errors.New("managed agent must run as root on Linux")
	}
	unlock, acquired, err := lockProtocolWorker(x.stateDir)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer unlock()
	_ = x.reportServices(ctx, c)
	jobCtx, cancel := context.WithTimeout(ctx, 110*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(jobCtx, "POST", c.Server+"/api/v1/agent/deployments/claim", bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("deployment control plane unreachable")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return ErrRevoked
	}
	if res.StatusCode != 200 {
		return errors.New("deployment claim rejected")
	}
	var response struct {
		Data *protocol.Task `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(res.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&response) != nil {
		return errors.New("invalid deployment task")
	}
	if response.Data == nil {
		return nil
	}
	result := x.apply(jobCtx, c, *response.Data)
	return request(ctx, c, "/api/v1/agent/deployments/result", result, true)
}

// A single lock spans claim, execution and acknowledgement, including across
// separate foreground/systemd agent processes. An in-progress journal is only
// interpreted as an interrupted operation after exclusive ownership is held.
func lockProtocolWorker(dir string) (func(), bool, error) {
	if err := secureDir(dir); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".worker.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, false, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		f.Close()
		return nil, false, errors.New("unsafe worker lock")
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) {
		f.Close()
		return nil, false, errors.New("unsafe worker lock owner")
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, true, nil
}
