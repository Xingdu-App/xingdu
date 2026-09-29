package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
)

func TestSelfUpdateQueue(t *testing.T) {
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
	if _, e = s.Enrollment(scope, host.ID, "monitor", machine.Hash(token)); e != nil {
		t.Fatal(e)
	}
	if e = s.EnrollAgent(ctx, machine.Hash(token), hash, "monitor"); e != nil {
		t.Fatal(e)
	}
	if e = s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", Arch: "arm64", CPUs: 1}); e != nil {
		t.Fatal(e)
	}

	if task, err := s.ClaimSelfUpdate(ctx, hash); err != nil || task != nil {
		t.Fatal("updater registration", err)
	}
	state, err := s.MachineState(scope, host.ID)
	if err != nil || !state.Agent.SelfUpdate {
		t.Fatal("capability missing", err)
	}
	j := machine.Job{ID: NewID("job"), HostID: host.ID, Mode: "monitor", Action: "upgrade", Transport: "agent", AgentHash: hash, Arch: "arm64", TargetVersion: machine.Version, ArtifactSHA256: strings.Repeat("a", 64)}
	target := machine.Target{Address: host.Address, Port: 22, User: "root"}
	if e = s.QueueInstall(WithTenant(ctx, user, NewID("org")), j, "", nil, target); e == nil {
		t.Fatal("cross tenant queue allowed")
	}
	if e = s.QueueInstall(scope, j, "", nil, target); e != nil {
		t.Fatal(e)
	}
	if _, e = w.ClaimMachineJob(ctx); !errors.Is(e, ErrNotFound) {
		t.Fatal("SSH worker claimed agent job", e)
	}
	task, e := s.ClaimSelfUpdate(ctx, hash)
	if e != nil || task == nil || task.ID != j.ID {
		t.Fatal("claim", e)
	}
	if again, e := s.ClaimSelfUpdate(ctx, hash); e != nil || again != nil {
		t.Fatal("running job replayed", e)
	}
	if e = s.CheckSelfUpdate(ctx, machine.Hash(machine.Token()), task.ID, task.Lease, "check"); e == nil {
		t.Fatal("foreign identity authorized")
	}
	if e = s.CheckSelfUpdate(ctx, hash, task.ID, NewID("lease"), "check"); e == nil {
		t.Fatal("wrong lease authorized")
	}
	if e = s.CompleteSelfUpdate(ctx, hash, machine.Version); e != nil {
		t.Fatal(e)
	}
	state, _ = s.MachineState(scope, host.ID)
	if state.Jobs[0].State != "running" {
		t.Fatal("heartbeat before apply completed task")
	}
	if e = s.CheckSelfUpdate(ctx, hash, task.ID, task.Lease, "applied"); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSelfUpdate(ctx, hash, "0.7.0-dev"); e != nil {
		t.Fatal(e)
	}
	state, _ = s.MachineState(scope, host.ID)
	if state.Jobs[0].State != "running" {
		t.Fatal("old heartbeat completed task")
	}
	if e = s.Heartbeat(ctx, hash, machine.Metrics{Version: machine.Version, Arch: "arm64", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if e = s.CompleteSelfUpdate(ctx, hash, machine.Version); e != nil {
		t.Fatal(e)
	}
	state, _ = s.MachineState(scope, host.ID)
	if state.Jobs[0].State != "installed" || state.Jobs[0].Result != "agent_upgraded" {
		t.Fatal("new heartbeat did not complete task")
	}
	// Return to an older fixture version, then revoke before the next claim.
	s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", Arch: "arm64", CPUs: 1})

	// A queued job cannot retain the privileges of a removed administrator.
	other, e := s.Register(ctx, "updater_"+NewID("obj")[4:20], "unused", "Other updater")
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", other)
		admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", other)
	}()
	if _, e = admin.Pool.Exec(ctx, `INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'admin')`, orgs[0].ID, other); e != nil {
		t.Fatal(e)
	}
	j.ID = NewID("job")
	if e = s.QueueInstall(WithTenant(ctx, other, orgs[0].ID), j, "", nil, target); e != nil {
		t.Fatal(e)
	}
	claimed, e := s.ClaimSelfUpdate(ctx, hash)
	if e != nil || claimed == nil {
		t.Fatal("administrator claim", e)
	}
	if _, e = admin.Pool.Exec(ctx, `DELETE FROM memberships WHERE organization_id=$1 AND user_id=$2`, orgs[0].ID, other); e != nil {
		t.Fatal(e)
	}
	if e = s.CheckSelfUpdate(ctx, hash, claimed.ID, claimed.Lease, "check"); !errors.Is(e, ErrForbidden) {
		t.Fatal("removed administrator kept execution rights", e)
	}
	// Mark this uncertain job expired using fixture authority, then observe cleanup.
	if _, e = admin.Pool.Exec(ctx, `UPDATE machine_jobs SET lease_until=now()-interval '1 second' WHERE id=$1`, claimed.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimSelfUpdate(ctx, hash); e != nil {
		t.Fatal(e)
	}
	j.ID = NewID("job")
	if e = s.QueueInstall(scope, j, "", nil, target); e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeMachine(scope, host.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ClaimSelfUpdate(ctx, hash); e == nil {
		t.Fatal("revoked machine claimed upgrade")
	}
}
