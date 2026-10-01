package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func taskFixture(t *testing.T) protocol.Task {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"proxy.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, _ := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	kb, _ := x509.MarshalPKCS8PrivateKey(key)
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	s, e := protocol.NewSpec(protocol.Input{Name: "test", Protocol: "trojan", Port: port, ServerName: "proxy.example.com", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}))})
	if e != nil {
		t.Fatal(e)
	}
	return protocol.Task{ID: "op_00000000000040008000000000000001", DeploymentID: "node_00000000000040008000000000000002", Lease: "lease_00000000000040008000000000000005", Action: "deploy", Spec: s}
}
func executorFixture(t *testing.T) (*protocolExecutor, Config, *[]string) {
	t.Helper()
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "units"), 0700)
	calls := []string{}
	body := []byte("verified-test-runtime")
	h := sha256.Sum256(body)
	protocol.RuntimeSHA256["test"] = hex.EncodeToString(h[:])
	t.Cleanup(func() { delete(protocol.RuntimeSHA256, "test") })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/runtime/test" {
			t.Error("wrong runtime path")
		}
		if r.Header.Get("Authorization") != "Bearer machine" {
			t.Error("missing machine auth")
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	x := &protocolExecutor{stateDir: filepath.Join(root, "state"), unitDir: filepath.Join(root, "units"), binaryDir: filepath.Join(root, "bin"), root: true, arch: "test", client: srv.Client(), run: func(_ context.Context, name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}}
	return x, Config{Server: srv.URL, Token: "machine", Mode: "manage"}, &calls
}
func TestProtocolApplyIdempotentAndRemove(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	r := x.apply(context.Background(), c, task)
	if !r.Success {
		t.Fatalf("deploy: %s", r.Code)
	}
	n := len(*calls)
	task.Lease = "lease_00000000000040008000000000000006"
	r = x.apply(context.Background(), c, task)
	if !r.Success || r.Lease != "lease_00000000000040008000000000000006" || len(*calls) != n {
		t.Fatal("replayed task executed again")
	}
	path := filepath.Join(x.stateDir, task.DeploymentID, "config.json")
	st, e := os.Stat(path)
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("config not private")
	}
	task.Action = "remove"
	task.ID = "op_00000000000040008000000000000003"
	r = x.apply(context.Background(), c, task)
	if !r.Success || r.Code != "removed" {
		t.Fatalf("remove: %s", r.Code)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("secret config remains")
	}
}
func TestProtocolRollbackAndJournal(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	run := x.run
	x.run = func(ctx context.Context, name string, args ...string) error {
		run(ctx, name, args...)
		if len(args) > 0 && args[0] == "enable" {
			return errors.New("start failed")
		}
		return nil
	}
	r := x.apply(context.Background(), c, task)
	if r.Success || r.Code != "start_failed" {
		t.Fatalf("unexpected: %+v", r)
	}
	for _, path := range []string{filepath.Join(x.stateDir, task.DeploymentID), filepath.Join(x.unitDir, serviceName(task.DeploymentID))} {
		if _, e := os.Stat(path); !os.IsNotExist(e) {
			t.Fatal("failed install remains")
		}
	}
	n := len(*calls)
	r = x.apply(context.Background(), c, task)
	if r.Code != "start_failed" || len(*calls) != n {
		t.Fatal("failed task replayed")
	}
	task.Spec.Name = "changed"
	r = x.apply(context.Background(), c, task)
	if r.Code != "journal_conflict" {
		t.Fatal("mutated job accepted")
	}
}
func TestProtocolRejectMonitorAndInterruptedTask(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	c.Mode = "monitor"
	if r := x.apply(context.Background(), c, task); r.Code != "manage_required" || len(*calls) != 0 {
		t.Fatal("monitor executed job")
	}
	c.Mode = "manage"
	dir := filepath.Join(x.stateDir, "outcomes")
	secureDir(dir)
	b, _ := json.Marshal(protocolJournal{Fingerprint: taskFingerprint(task)})
	os.WriteFile(filepath.Join(dir, task.ID+".json"), b, 0600)
	if r := x.apply(context.Background(), c, task); r.Code != "interrupted" || len(*calls) != 0 {
		t.Fatal("interrupted job replayed")
	}
}
func TestProtocolRejectOccupiedPortAndUnownedService(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	l, e := net.Listen("tcp", ":0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	task.Spec.Port = l.Addr().(*net.TCPAddr).Port
	if code := x.execute(context.Background(), c, task); code != "port_in_use" || len(*calls) != 0 {
		t.Fatal("occupied port accepted")
	}
	task.Action = "remove"
	dir := filepath.Join(x.stateDir, task.DeploymentID)
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "owner"), []byte("other"), 0600)
	if code := x.execute(context.Background(), c, task); code != "ownership_mismatch" {
		t.Fatal("foreign service removed")
	}
}
func TestProtocolRuntimeChecksum(t *testing.T) {
	x, c, _ := executorFixture(t)
	protocol.RuntimeSHA256["test"] = strings.Repeat("0", 64)
	if _, e := x.binary(context.Background(), c); e == nil {
		t.Fatal("tampered runtime accepted")
	}
	files, _ := os.ReadDir(x.binaryDir)
	for _, f := range files {
		if f.Name() != "sing-box-LICENSE" {
			t.Fatal("bad download remains")
		}
	}
	license, err := os.ReadFile(filepath.Join(x.binaryDir, "sing-box-LICENSE"))
	if err != nil || string(license) != protocol.RuntimeLicense {
		t.Fatal("runtime license missing")
	}
}
func TestMonitorDoesNotPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("monitor polled tasks") }))
	defer srv.Close()
	if e := pollProtocols(context.Background(), Config{Mode: "monitor", Server: srv.URL}); e != nil {
		t.Fatal(e)
	}
}

func TestRemoveRecoversAfterReloadFailure(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	task.Action = "remove"
	dir := filepath.Join(x.stateDir, task.DeploymentID)
	secureDir(dir)
	os.WriteFile(filepath.Join(dir, "owner"), []byte(task.DeploymentID), 0600)
	unit := filepath.Join(x.unitDir, serviceName(task.DeploymentID))
	os.WriteFile(unit, []byte("# Xingdu managed deployment "+task.DeploymentID+"\n"), 0644)
	run := x.run
	failed := false
	x.run = func(ctx context.Context, name string, args ...string) error {
		run(ctx, name, args...)
		if args[0] == "daemon-reload" && !failed {
			failed = true
			return errors.New("reload unavailable")
		}
		return nil
	}
	if code := x.execute(context.Background(), c, task); code != "reload_failed" {
		t.Fatalf("got %s", code)
	}
	if _, e := os.Stat(filepath.Join(dir, "owner")); e != nil {
		t.Fatal("lost ownership")
	}
	if code := x.execute(context.Background(), c, task); code != "removed" {
		t.Fatalf("retry: %s", code)
	}
}
func TestRemoveStopFailurePreservesOwnership(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	task.Action = "remove"
	dir := filepath.Join(x.stateDir, task.DeploymentID)
	secureDir(dir)
	os.WriteFile(filepath.Join(dir, "owner"), []byte(task.DeploymentID), 0600)
	unit := filepath.Join(x.unitDir, serviceName(task.DeploymentID))
	os.WriteFile(unit, []byte("# Xingdu managed deployment "+task.DeploymentID+"\n"), 0644)
	x.run = func(context.Context, string, ...string) error { return errors.New("bus unavailable") }
	if code := x.execute(context.Background(), c, task); code != "stop_failed" {
		t.Fatalf("got %s", code)
	}
	if _, e := os.Stat(unit); e != nil {
		t.Fatal("removed unit despite stop failure")
	}
	if _, e := os.Stat(filepath.Join(dir, "owner")); e != nil {
		t.Fatal("lost owner")
	}
}
func TestStartRollbackFailurePreservesOwnedService(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	x.run = func(_ context.Context, _ string, args ...string) error {
		if args[0] == "enable" || args[0] == "disable" {
			return errors.New("systemd failure")
		}
		return nil
	}
	if r := x.apply(context.Background(), c, task); r.Code != "rollback_failed" {
		t.Fatalf("got %s", r.Code)
	}
	if _, e := os.Stat(filepath.Join(x.stateDir, task.DeploymentID, "owner")); e != nil {
		t.Fatal("lost recovery owner")
	}
}
func TestStaleProtocolResultDoesNotRevokeAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer srv.Close()
	err := request(context.Background(), Config{Server: srv.URL}, "/api/v1/agent/deployments/result", protocol.Result{}, true)
	if err == nil || errors.Is(err, ErrRevoked) {
		t.Fatal("stale result treated as revocation")
	}
}

func TestRuntimeLicenseRejectsSymlink(t *testing.T) {
	x, c, _ := executorFixture(t)
	os.MkdirAll(x.binaryDir, 0755)
	target := filepath.Join(t.TempDir(), "unrelated")
	os.WriteFile(target, []byte("preserve"), 0600)
	os.Symlink(target, filepath.Join(x.binaryDir, "sing-box-LICENSE"))
	if _, e := x.binary(context.Background(), c); e == nil {
		t.Fatal("symlink license accepted")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "preserve" {
		t.Fatal("unrelated file overwritten")
	}
}

func TestRuntimeDirectoryTraversableWithRestrictiveUmask(t *testing.T) {
	x, c, _ := executorFixture(t)
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	path, err := x.binary(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{x.binaryDir, path} {
		st, err := os.Stat(p)
		if err != nil || st.Mode().Perm() != 0755 {
			t.Fatalf("runtime path is not traversable/executable: %s", p)
		}
	}
	unit := protocolUnit("node_00000000000040008000000000000002", path, "/root/config.json")
	if strings.Contains(unit, "network-online.target") {
		t.Fatal("listener blocks on external network readiness")
	}
	if !strings.Contains(unit, "StandardInput=file:/root/config.json") || !strings.Contains(unit, "run -c stdin") || strings.Contains(unit, "LoadCredential=") {
		t.Fatal("runtime must consume the already-open private config descriptor")
	}

}

func TestProtocolWorkerExclusiveLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	unlock, acquired, err := lockProtocolWorker(dir)
	if err != nil || !acquired {
		t.Fatal("first worker failed to acquire lock")
	}
	second, acquired, err := lockProtocolWorker(dir)
	if second != nil || acquired || err != nil {
		t.Fatal("concurrent worker acquired lock")
	}
	unlock()
	third, acquired, err := lockProtocolWorker(dir)
	if err != nil || !acquired {
		t.Fatal("lock not released")
	}
	third()
}
func TestRollbackReloadFailurePreservesOwnership(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	reloads := 0
	x.run = func(_ context.Context, _ string, args ...string) error {
		if args[0] == "enable" {
			return errors.New("start failed")
		}
		if args[0] == "daemon-reload" {
			reloads++
			if reloads > 1 {
				return errors.New("reload failed")
			}
		}
		return nil
	}
	if r := x.apply(context.Background(), c, task); r.Code != "rollback_failed" {
		t.Fatalf("got %s", r.Code)
	}
	if _, e := os.Stat(filepath.Join(x.stateDir, task.DeploymentID, "owner")); e != nil {
		t.Fatal("lost recovery owner")
	}
	x.run = func(context.Context, string, ...string) error { return nil }
	task.Action = "remove"
	if code := x.execute(context.Background(), c, task); code != "removed" {
		t.Fatalf("recovery: %s", code)
	}
}
func TestRollbackUnlinkFailurePreservesOwnership(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	unit := filepath.Join(x.unitDir, serviceName(task.DeploymentID))
	x.run = func(_ context.Context, _ string, args ...string) error {
		if args[0] == "enable" {
			os.Remove(unit)
			os.Mkdir(unit, 0700)
			os.WriteFile(filepath.Join(unit, "block-unlink"), nil, 0600)
			return errors.New("start failed")
		}
		return nil
	}
	if r := x.apply(context.Background(), c, task); r.Code != "rollback_failed" {
		t.Fatalf("got %s", r.Code)
	}
	if _, e := os.Stat(filepath.Join(x.stateDir, task.DeploymentID, "owner")); e != nil {
		t.Fatal("lost recovery owner")
	}
}

func TestControlledRestartAndServiceStatus(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	if got := x.apply(context.Background(), c, task); !got.Success {
		t.Fatal(got.Code)
	}
	if got := x.serviceStatus(context.Background(), task.DeploymentID); got != "active" {
		t.Fatal(got)
	}
	task.ID = "op_00000000000040008000000000000003"
	task.Action = "restart"
	task.Spec = protocol.Spec{}
	if got := x.apply(context.Background(), c, task); !got.Success || got.Code != "restarted" {
		t.Fatal(got)
	}
	count := len(*calls)
	if got := x.apply(context.Background(), c, task); !got.Success || len(*calls) != count {
		t.Fatal("restart replay executed")
	}
	os.WriteFile(filepath.Join(x.unitDir, serviceName(task.DeploymentID)), []byte("unowned service"), 0644)
	task.ID = "op_00000000000040008000000000000004"
	if got := x.apply(context.Background(), c, task); got.Success || got.Code != "ownership_mismatch" {
		t.Fatal(got)
	}
	if len(*calls) != count {
		t.Fatal("unowned service modified")
	}
	if got := x.serviceStatus(context.Background(), task.DeploymentID); got != "missing" {
		t.Fatal(got)
	}
}

func TestShadowsocksDoesNotReserveUDPPort(t *testing.T) {
	task := taskFixture(t)
	listener, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for _, kind := range []string{"shadowsocks", "shadowsocks2022"} {
		task.Spec, err = protocol.NewSpec(protocol.Input{Name: "SS", Protocol: kind, Port: listener.LocalAddr().(*net.UDPAddr).Port})
		if err != nil {
			t.Fatal(err)
		}
		if !portAvailable(task.Spec) {
			t.Fatal("TCP-only SS incorrectly reserves UDP", kind)
		}
	}
}

func TestRuntimeCancelledTransferLeavesNoExecutable(t *testing.T) {
	x, _, _ := executorFixture(t)
	received := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		close(received)
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := x.binary(ctx, Config{Server: srv.URL, Token: "machine"}); done <- err }()
	<-received
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled download accepted")
	}
	files, err := os.ReadDir(x.binaryDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name() != "sing-box-LICENSE" {
			t.Fatal("partial download retained", f.Name())
		}
	}
}

func TestRuntimeOperationBudgetFitsLease(t *testing.T) {
	if newProtocolExecutor().client.Timeout <= 70*time.Second {
		t.Fatal("cold downloads still use the short timeout")
	}
	if protocol.DeploymentTimeout < protocol.RuntimeDownloadTimeout+30*time.Second {
		t.Fatal("no installation budget after download")
	}
	if protocol.DeploymentLease < protocol.DeploymentTimeout+30*time.Second {
		t.Fatal("no acknowledgement budget before lease expiry")
	}
	if protocol.RuntimeResponseTimeout < protocol.RuntimeDownloadTimeout {
		t.Fatal("server expires download before client")
	}
}

func TestTrustTunnelRefusesUnverifiedSELinuxPolicy(t *testing.T) {
	x, c, _ := executorFixture(t)
	x.selinuxEnforcePath = filepath.Join(t.TempDir(), "enforce")
	if err := os.WriteFile(x.selinuxEnforcePath, []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	task := taskFixture(t)
	task.Spec.Protocol = "trusttunnel"
	if _, err := x.specBinary(context.Background(), c, task.Spec); err == nil {
		t.Fatal("unconfined TrustTunnel allowed")
	}
}
func TestTrustTunnelUnitGuard(t *testing.T) {
	unit := protocolUnit("node_test", "/usr/local/lib/xingdu/trusttunnel-1.1.0", "/private/config.json")
	for _, want := range []string{"DynamicUser=yes", "StandardInput=file:/private/config.json", "IPAddressDeny=168.63.129.16/32", "run -c stdin", "NoNewPrivileges=yes"} {
		if !strings.Contains(unit, want) {
			t.Fatal("missing guard", want)
		}
	}
}

func TestTrustTunnelDeployUpdateRollbackAndRemove(t *testing.T) {
	x, c, _ := executorFixture(t)
	x.egressFilter = func(string) bool { return true }
	body := []byte("verified-trusttunnel-fixture")
	sum := sha256.Sum256(body)
	protocol.TrustTunnelSHA256["test"] = hex.EncodeToString(sum[:])
	t.Cleanup(func() { delete(protocol.TrustTunnelSHA256, "test") })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/runtime/trusttunnel/test" || r.Header.Get("Authorization") != "Bearer machine" {
			t.Error("wrong artifact request")
		}
		w.Write(body)
	}))
	defer srv.Close()
	c.Server = srv.URL
	x.client = srv.Client()
	task := taskFixture(t)
	task.Spec.Protocol = "trusttunnel"
	ctx := context.Background()
	if r := x.apply(ctx, c, task); !r.Success {
		t.Fatal(r.Code)
	}
	if x.runtimeVersion(task.DeploymentID) != protocol.TrustTunnelVersion {
		t.Fatal("incorrect runtime version")
	}
	path := filepath.Join(x.stateDir, task.DeploymentID, "config.json")
	before, _ := os.ReadFile(path)
	task.Action = "update"
	task.ID = "op_00000000000040008000000000000009"
	task.Spec.Name = "Updated"
	run := x.run
	restarts := 0
	x.run = func(ctx context.Context, name string, args ...string) error {
		if len(args) > 0 && args[0] == "restart" {
			restarts++
			if restarts == 1 {
				return errors.New("fixture restart failure")
			}
		}
		return run(ctx, name, args...)
	}
	if r := x.apply(ctx, c, task); r.Code != "update_rolled_back" {
		t.Fatal(r.Code)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("failed update lost prior revision")
	}
	task.ID = "op_0000000000004000800000000000000a"
	if r := x.apply(ctx, c, task); !r.Success {
		t.Fatal(r.Code)
	}
	x.egressFilter = func(string) bool { return false }
	if x.serviceStatus(ctx, task.DeploymentID) != "policy_required" {
		t.Fatal("missing BPF reported active")
	}
	task.ID = "op_0000000000004000800000000000000c"
	task.Action = "restart"
	stopped := false
	priorRun := x.run
	x.run = func(ctx context.Context, name string, args ...string) error {
		if name == "systemctl" && len(args) > 0 && args[0] == "stop" {
			stopped = true
		}
		return priorRun(ctx, name, args...)
	}
	if r := x.apply(ctx, c, task); r.Code != "runtime_policy_failed" || !stopped {
		t.Fatal("unfiltered restart was not stopped", r.Code)
	}
	task.ID = "op_0000000000004000800000000000000b"
	task.Action = "remove"
	if r := x.apply(ctx, c, task); !r.Success {
		t.Fatal(r.Code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("secret revision remains")
	}
}
