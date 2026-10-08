package storage

import (
	"bytes"
	"errors"
	"testing"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
)

func TestExternalExitTenantRolesAndRevisionLifecycle(t *testing.T) {
	f := newOAuthFixture(t)
	s := f.store
	owner, _ := f.local(t)
	orgs, _ := s.Organizations(f.ctx, owner.ID)
	ctx := WithTenant(f.ctx, owner.ID, orgs[0].ID)
	stranger, _ := f.local(t)
	foreignOrgs, _ := s.Organizations(f.ctx, stranger.ID)
	foreign := WithTenant(f.ctx, stranger.ID, foreignOrgs[0].ID)
	member, _ := f.local(t)
	_, err := f.admin.Pool.Exec(f.ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'member')", orgs[0].ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	memberCtx := WithTenant(f.ctx, member.ID, orgs[0].ID)
	p := ExternalProxy{ID: NewID("ext"), Name: "ISP fixture", Address: "93.184.216.34", Port: 1080, Protocol: "socks", Encrypted: []byte("encrypted-fixture")}
	if _, err = s.SaveExternalProxy(memberCtx, p, nil, true); !errors.Is(err, ErrForbidden) {
		t.Fatal("member creates external exit", err)
	}
	created, err := s.SaveExternalProxy(ctx, p, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if created.Encrypted != nil {
		t.Fatal("mutation returned ciphertext")
	}
	if list, err := s.ExternalProxies(foreign); err != nil || len(list) != 0 {
		t.Fatal("foreign tenant inventory leak", err)
	}
	if _, err = s.ExternalProxySecret(foreign, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign tenant secret leak", err)
	}
	if _, err = s.ExternalProxySecret(memberCtx, p.ID); !errors.Is(err, ErrForbidden) {
		t.Fatal("member reads secret", err)
	}
	if list, err := s.ExternalProxies(memberCtx); err != nil || len(list) != 1 || list[0].Encrypted != nil {
		t.Fatal("member metadata read failed", err)
	}
	if err = s.DeleteExternalProxy(foreign, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign delete accepted", err)
	}
	if _, err = s.SaveExternalProxy(ctx, p, []byte("stale"), false); !errors.Is(err, ErrConflict) {
		t.Fatal("stale ciphertext edit accepted", err)
	}
	old := p.Encrypted
	p.Name = "Metadata edit"
	p.Encrypted = nil
	if _, err = s.SaveExternalProxy(ctx, p, old, false); err != nil {
		t.Fatal(err)
	}
	p, err = s.ExternalProxySecret(ctx, p.ID)
	if err != nil || !bytes.Equal(p.Encrypted, old) {
		t.Fatal("credential preservation failed", err)
	}
	h, err := s.CreateHost(ctx, hosts.Input{Name: "entry", Address: "entry.example.invalid", SSHUser: "root", SSHPort: 22, Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	enrollment, identity := machine.Hash(machine.Token()), machine.Hash(machine.Token())
	if _, err = s.Enrollment(ctx, h.ID, "manage", enrollment); err != nil {
		t.Fatal(err)
	}
	if err = s.EnrollAgent(f.ctx, enrollment, identity, "manage"); err != nil {
		t.Fatal(err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: "0.17.0", CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	d := Deployment{ID: NewID("node"), OperationID: NewID("op"), HostID: h.ID, Name: "Entry", Protocol: "shadowsocks", Port: 24443, Encrypted: []byte("direct-config")}
	if err = s.QueueDeployment(ctx, d); err != nil {
		t.Fatal(err)
	}
	finish := func(ok bool, code string) {
		t.Helper()
		task, e := s.ClaimDeployment(f.ctx, identity)
		if e != nil || task == nil {
			t.Fatal("claim", e)
		}
		if e = s.FinishDeployment(f.ctx, identity, task.OperationID, task.Lease, ok, code); e != nil {
			t.Fatal(e)
		}
	}
	finish(true, "deployed")
	previous := d.Encrypted
	d.Encrypted = []byte("external-config")
	d.ExternalExitID = p.ID
	d.ExternalCipher = p.Encrypted
	d.ExternalRevision = p.Revision
	if err = s.UpdateDeployment(ctx, d, previous); !errors.Is(err, ErrConflict) {
		t.Fatal("old Agent accepts external profile", err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: protocol.ExternalProxyAgentVersion, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateDeployment(foreign, d, previous); err == nil {
		t.Fatal("foreign entry update accepted")
	}
	d.ExternalRevision--
	if err = s.UpdateDeployment(ctx, d, previous); !errors.Is(err, ErrConflict) {
		t.Fatal("stale profile metadata accepted", err)
	}
	d.ExternalRevision = p.Revision
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	blocked := func() {
		t.Helper()
		if err := s.DeleteExternalProxy(ctx, p.ID); !errors.Is(err, ErrConflict) {
			t.Fatal("referenced exit deleted", err)
		}
		if _, err := s.SaveExternalProxy(ctx, p, p.Encrypted, false); !errors.Is(err, ErrConflict) {
			t.Fatal("referenced exit edited", err)
		}
	}
	blocked()
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: "0.17.0", CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimDeployment(f.ctx, identity); !errors.Is(err, ErrConflict) {
		t.Fatal("downgraded Agent claims external task", err)
	}
	if err = s.Heartbeat(f.ctx, identity, machine.Metrics{Version: protocol.ExternalProxyAgentVersion, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	finish(false, "update_rolled_back")
	active, err := s.DeploymentSecret(ctx, h.ID, d.ID)
	if err != nil || active.ExternalExitID != "" || !bytes.Equal(active.Encrypted, previous) {
		t.Fatal("failed candidate promoted", err)
	}
	// Failed first installation no longer references this profile. Retry and promote.
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	finish(true, "updated")
	blocked()
	active, err = s.DeploymentSecret(ctx, h.ID, d.ID)
	if err != nil || active.ExternalExitID != p.ID {
		t.Fatal("external metadata not promoted", err)
	}
	nodes, err := s.Nodes(ctx)
	if err != nil || len(nodes) != 1 || nodes[0].ExternalExitID != p.ID {
		t.Fatal("inventory metadata missing", err)
	}
	previous = d.Encrypted
	d.ExternalExitID = ""
	d.ExternalCipher = nil
	d.Encrypted = []byte("direct-again")
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	blocked()
	finish(false, "update_rolled_back")
	blocked()
	if err = s.UpdateDeployment(ctx, d, previous); err != nil {
		t.Fatal(err)
	}
	finish(true, "updated")
	if err = s.DeleteExternalProxy(ctx, p.ID); err != nil {
		t.Fatal("dependency not released", err)
	}
	if _, err = s.ExternalProxySecret(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted exit available for restore", err)
	}
	var events int
	if err = f.admin.Pool.QueryRow(f.ctx, "SELECT count(*) FROM external_proxy_audit WHERE proxy_id=$1", p.ID).Scan(&events); err != nil || events != 3 {
		t.Fatal("metadata audit incomplete", events, err)
	}
}
