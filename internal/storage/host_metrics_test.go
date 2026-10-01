package storage

import (
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
)

func TestHostMetricsHeartbeatIsolationAndRevocation(t *testing.T) {
	f := newOAuthFixture(t)
	user, _ := f.local(t)
	orgs, err := f.store.Organizations(f.ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTenant(f.ctx, user.ID, orgs[0].ID)
	host, err := f.store.CreateHost(ctx, hosts.Input{Name: "System fixture", Address: "system.example.invalid", SSHUser: "root", SSHPort: 22, Tags: []string{}})
	if err != nil || host.Metrics != nil {
		t.Fatal("pending host metrics", host.Metrics, err)
	}
	token, identity := machine.Hash(machine.Token()), machine.Hash(machine.Token())
	if _, err = f.store.Enrollment(ctx, host.ID, "manage", token); err != nil {
		t.Fatal(err)
	}
	if err = f.store.EnrollAgent(f.ctx, token, identity, "manage"); err != nil {
		t.Fatal(err)
	}
	metrics := machine.Metrics{OS: "linux", Arch: "amd64", Version: machine.Version, CPUs: 2, MemoryTotal: 2147483648, MemoryAvailable: 1073741824, Load1: 0.25}
	if err = f.store.Heartbeat(f.ctx, identity, metrics); err != nil {
		t.Fatal(err)
	}
	listed, err := f.store.Hosts(ctx)
	if err != nil || len(listed) != 1 {
		t.Fatal("list host", listed, err)
	}
	if listed[0].Metrics == nil || *listed[0].Metrics != metrics {
		t.Fatal("heartbeat metrics lost", listed[0].Metrics)
	}
	updated, err := f.store.UpdateHost(ctx, host.ID, host.Input)
	if err != nil || updated.Metrics == nil || *updated.Metrics != metrics {
		t.Fatal("update metrics", updated.Metrics, err)
	}
	other, _ := f.local(t)
	otherOrgs, err := f.store.Organizations(f.ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := f.store.Hosts(WithTenant(f.ctx, other.ID, otherOrgs[0].ID))
	if err != nil || len(foreign) != 0 {
		t.Fatal("cross tenant metrics exposed", foreign, err)
	}
	if err = f.store.RevokeMachine(ctx, host.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = f.store.Hosts(ctx)
	if err != nil || len(listed) != 1 || listed[0].Metrics != nil {
		t.Fatal("revoked metrics retained", listed, err)
	}
}
