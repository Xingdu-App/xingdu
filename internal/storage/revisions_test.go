package storage

import (
	"bytes"
	"errors"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
)

func TestRevisionAcknowledgementAndIsolation(t *testing.T) {
	f := newOAuthFixture(t)
	u, _ := f.local(t)
	orgs, _ := f.store.Organizations(f.ctx, u.ID)
	org := orgs[0].ID
	other, _ := f.local(t)
	others, _ := f.store.Organizations(f.ctx, other.ID)
	otherOrg := others[0].ID
	ctx := WithTenant(f.ctx, u.ID, org)
	foreign := WithTenant(f.ctx, other.ID, otherOrg)
	s := f.store
	h, err := s.CreateHost(ctx, hosts.Input{Name: "revision", Address: "revision.example.invalid", SSHUser: "root", SSHPort: 22, Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	token, identity := machine.Hash(machine.Token()), machine.Hash(machine.Token())
	if _, err = s.Enrollment(ctx, h.ID, "manage", token); err != nil {
		t.Fatal(err)
	}
	if err = s.EnrollAgent(f.ctx, token, identity, "manage"); err != nil {
		t.Fatal(err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: "0.12.0-dev", CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	d := Deployment{ID: NewID("node"), OperationID: NewID("op"), HostID: h.ID, Name: "original", Protocol: "shadowsocks", Port: 24499, Encrypted: []byte("old-ciphertext")}
	if err = s.QueueDeployment(ctx, d); err != nil {
		t.Fatal(err)
	}
	task, err := s.ClaimDeployment(f.ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, true, "deployed"); err != nil {
		t.Fatal(err)
	}
	previous := d.Encrypted
	d.Encrypted = []byte("candidate-ciphertext")
	d.Name = "new"
	if err = s.UpdateDeployment(foreign, d, previous); err == nil {
		t.Fatal("cross tenant update")
	}
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	active, err := s.DeploymentSecret(ctx, h.ID, d.ID)
	if err != nil || !bytes.Equal(active.Encrypted, previous) {
		t.Fatal("candidate exposed before acknowledgement", err)
	}
	if err = s.UpdateDeployment(ctx, d, previous); !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrConflict) {
		t.Fatal("concurrent update allowed", err)
	}
	task, err = s.ClaimDeployment(f.ctx, identity)
	if err != nil || !bytes.Equal(task.Encrypted, d.Encrypted) {
		t.Fatal("candidate not delivered", err)
	}
	if err = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, false, "update_rolled_back"); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	task, err = s.ClaimDeployment(f.ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, true, "updated"); err != nil {
		t.Fatal(err)
	}
	active, err = s.DeploymentSecret(ctx, h.ID, d.ID)
	if err != nil || !bytes.Equal(active.Encrypted, d.Encrypted) {
		t.Fatal("candidate not promoted", err)
	}
	if err = s.UpdateDeployment(ctx, d, previous); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit accepted", err)
	}
	revisions, err := s.Revisions(ctx, h.ID, d.ID)
	if err != nil || len(revisions) != 3 || !revisions[0].Current {
		t.Fatal(revisions, err)
	}
	if _, err = s.RevisionSecret(foreign, h.ID, d.ID, 1); err == nil {
		t.Fatal("cross tenant revision disclosure")
	}
	// A pending or active relay protects its exit, including host-level deletion.
	h2, err := s.CreateHost(ctx, hosts.Input{Name: "exit", Address: "exit.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	enrollment, exitIdentity := machine.Hash(machine.Token()), machine.Hash(machine.Token())
	if _, err = s.Enrollment(ctx, h2.ID, "manage", enrollment); err != nil {
		t.Fatal(err)
	}
	if err = s.EnrollAgent(f.ctx, enrollment, exitIdentity, "manage"); err != nil {
		t.Fatal(err)
	}
	if err = s.Heartbeat(f.ctx, exitIdentity, machine.Metrics{Version: machine.Version, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	exit := Deployment{ID: NewID("node"), OperationID: NewID("op"), HostID: h2.ID, Name: "exit", Protocol: "socks", Port: 24499, Encrypted: []byte("exit-cipher")}
	if err = s.QueueDeployment(ctx, exit); err != nil {
		t.Fatal(err)
	}
	task, err = s.ClaimDeployment(f.ctx, exitIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDeployment(f.ctx, exitIdentity, task.OperationID, task.Lease, true, "deployed"); err != nil {
		t.Fatal(err)
	}
	previous = d.Encrypted
	d.Encrypted = []byte("relay-cipher")
	d.RelayExitID = exit.ID
	d.ExitCipher = exit.Encrypted
	d.Port = 24498
	if err = s.UpdateDeployment(ctx, d, previous); !errors.Is(err, ErrConflict) {
		t.Fatal("old entry Agent accepted new exit protocol", err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: machine.Version, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveDeployment(ctx, h2.ID, exit.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("pending exit can be removed", err)
	}
	if err = s.DeleteHost(ctx, h2.ID); !errors.Is(err, ErrConflict) {
		t.Fatal("pending exit host can be deleted", err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: "0.13.0-dev", CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimDeployment(f.ctx, identity); !errors.Is(err, ErrConflict) {
		t.Fatal("downgraded entry claimed new exit protocol", err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: machine.Version, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	task, err = s.ClaimDeployment(f.ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, true, "updated"); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.Nodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.ID == d.ID && (n.RelayExitID != exit.ID || n.Port != 24498) {
			t.Fatal("relay or port not promoted")
		}
	}
	if err = s.UpdateDeployment(ctx, exit, exit.Encrypted); !errors.Is(err, ErrConflict) {
		t.Fatal("active exit can change", err)
	}
	if err = s.RecordProbe(ctx, h.ID, d.ID, ProbeReport{OK: true, ExitIP: "93.184.216.34", LatencyMS: 20, Revision: 1}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale probe accepted", err)
	}
	if err = s.RemoveDeployment(ctx, h.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	task, err = s.ClaimDeployment(f.ctx, identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, true, "removed"); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveDeployment(ctx, h2.ID, exit.ID); err != nil {
		t.Fatal("exit dependency not released", err)
	}

}
