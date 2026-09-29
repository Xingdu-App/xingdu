package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
)

func TestAgentUpgradeQueue(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, e := Open(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	if e = admin.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	openRole := func(role string) *Store {
		u, _ := url.Parse(raw)
		q := u.Query()
		q.Set("options", "-crole="+role)
		u.RawQuery = q.Encode()
		s, e := Open(ctx, u.String())
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(s.Close)
		return s
	}
	s, w := openRole("xingdu_app"), openRole("xingdu_worker")
	user, err := s.Register(ctx, "upgrade_"+NewID("obj")[4:20], "unused", "Upgrade test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", user)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", user)
	}()
	orgs, _ := s.Organizations(ctx, user)
	scope := WithTenant(ctx, user, orgs[0].ID)
	host, err := s.CreateHost(scope, hosts.Input{Name: "Upgrade fixture", Address: "upgrade.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	token, hash := machine.Token(), machine.Hash(machine.Token())
	if _, e = s.Enrollment(scope, host.ID, "manage", machine.Hash(token)); e != nil {
		t.Fatal(e)
	}
	if e = s.EnrollAgent(ctx, machine.Hash(token), hash, "manage"); e != nil {
		t.Fatal(e)
	}
	if e = s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", Arch: "arm64", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	j := machine.Job{ID: NewID("job"), HostID: host.ID, Mode: "manage", Action: "upgrade", AgentHash: hash, Arch: "arm64", TargetVersion: machine.Version, ArtifactSHA256: strings.Repeat("a", 64), Encrypted: []byte("encrypted")}
	target := machine.Target{Address: host.Address, Port: 22, User: "root"}
	if e = s.QueueInstall(WithTenant(ctx, user, NewID("org")), j, "", nil, target); e == nil {
		t.Fatal("cross tenant queue allowed")
	}
	wrong := j
	wrong.AgentHash = machine.Hash(machine.Token())
	if e = s.QueueInstall(scope, wrong, "", nil, target); !errors.Is(e, ErrConflict) {
		t.Fatal("replacement identity accepted", e)
	}
	if e = s.QueueInstall(scope, j, "", nil, target); e != nil {
		t.Fatal(e)
	}
	duplicate := j
	duplicate.ID = NewID("job")
	if e = s.QueueInstall(scope, duplicate, "", nil, target); !errors.Is(e, ErrConflict) {
		t.Fatal("duplicate queue allowed", e)
	}
	if e = s.DeploymentPreflight(scope, host.ID, 443, "trojan"); !errors.Is(e, ErrConflict) {
		t.Fatal("protocol operation permitted during upgrade", e)
	}
	state, e := s.MachineState(scope, host.ID)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(state)
	if strings.Contains(string(encoded), hash) || strings.Contains(string(encoded), "encrypted") {
		t.Fatal("upgrade identity/ciphertext leak")
	}
	claim, e := w.ClaimMachineJob(ctx)
	if e != nil {
		t.Fatal(e)
	}
	claim, e = w.LoadMachineJob(scope, claim)
	if e != nil || claim.Action != "upgrade" || claim.TargetVersion != machine.Version {
		t.Fatal("upgrade claim lost binding", e)
	}
	if ready, e := w.UpgradeReady(scope, claim); e != nil || ready {
		t.Fatal("old heartbeat completes upgrade", e)
	}
	if e = s.Heartbeat(ctx, hash, machine.Metrics{Version: machine.Version, Arch: "arm64", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if ready, e := w.UpgradeReady(scope, claim); e != nil || !ready {
		t.Fatal("fresh target heartbeat missing", e)
	}
	if e = w.FinishMachineJob(scope, claim, "installed", "agent_upgraded"); e != nil {
		t.Fatal(e)
	}
	var cleared bool
	admin.Pool.QueryRow(ctx, "SELECT encrypted IS NULL FROM machine_jobs WHERE id=$1", j.ID).Scan(&cleared)
	if !cleared {
		t.Fatal("upgrade credentials retained")
	}
	j.ID = NewID("job")
	if e = s.QueueInstall(scope, j, "", nil, target); !errors.Is(e, ErrConflict) {
		t.Fatal("same version reinstall accepted", e)
	}
	if e = s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", Arch: "arm64", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if e = s.QueueInstall(scope, j, "", nil, target); e != nil {
		t.Fatal(e)
	}
	claim, e = w.ClaimMachineJob(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeMachine(scope, host.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = w.LoadMachineJob(scope, claim); e == nil {
		t.Fatal("revoked upgrade executed")
	}
}
