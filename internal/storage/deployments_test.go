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
	"xingdu.app/xingdu/internal/protocol"
)

func TestDeploymentTenantAgentLifecycle(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("options", "-crole=xingdu_app")
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Exercise the full lifecycle on the free one-server cloud tier.
	s.CloudBilling = true
	users := []string{}
	defer func() {
		for _, id := range users {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range users {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	user := func() (string, string) {
		id, e := s.Register(ctx, "deploy_"+NewID("obj")[4:12], "unused", "Protocol test")
		if e != nil {
			t.Fatal(e)
		}
		users = append(users, id)
		orgs, _ := s.Organizations(ctx, id)
		return id, orgs[0].ID
	}
	a, org := user()
	b, other := user()
	ca, cb := WithTenant(ctx, a, org), WithTenant(ctx, b, other)
	h, err := s.CreateHost(ca, hosts.Input{Name: "protocol", Address: "protocol.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	enroll := func(mode string) string {
		token, identity := machine.Token(), machine.Hash(machine.Token())
		if _, e := s.Enrollment(ca, h.ID, mode, machine.Hash(token)); e != nil {
			t.Fatal(e)
		}
		if e := s.EnrollAgent(ctx, machine.Hash(token), identity, mode); e != nil {
			t.Fatal(e)
		}
		if e := s.Heartbeat(ctx, identity, machine.Metrics{Version: "0.7.0-dev", CPUs: 1}); e != nil {
			t.Fatal(e)
		}
		return identity
	}
	hash := enroll("monitor")
	next := func(port int) Deployment {
		return Deployment{ID: NewID("node"), OperationID: NewID("op"), HostID: h.ID, Name: "test", Protocol: "trojan", Port: port, ServerName: "protocol.example.invalid", Encrypted: []byte("encrypted-only")}
	}
	d := next(443)
	if e := s.QueueDeployment(ca, d); !errors.Is(e, ErrConflict) {
		t.Fatal("monitor allowed", e)
	}
	if _, e := s.ClaimDeployment(ctx, hash); !errors.Is(e, ErrNotFound) {
		t.Fatal("monitor claim", e)
	}
	hash = enroll("manage")
	admin.Pool.Exec(ctx, "UPDATE hosts SET last_seen_at=now()-interval '91 seconds' WHERE id=$1", h.ID)
	if e := s.QueueDeployment(ca, d); !errors.Is(e, ErrConflict) {
		t.Fatal("stale agent accepted", e)
	}
	s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.4.0-dev", CPUs: 1})
	if e := s.QueueDeployment(ca, d); !errors.Is(e, ErrConflict) {
		t.Fatal("old agent accepted", e)
	}
	s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.8.0-dev", CPUs: 1})
	if e := s.QueueDeployment(ca, d); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Deployments(cb, h.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("cross tenant list", e)
	}
	tx, _ := s.Pool.Begin(ctx)
	setScope(ctx, tx, b, other)
	var count int
	if e := tx.QueryRow(ctx, "SELECT count(*) FROM protocol_deployments").Scan(&count); e != nil || count != 0 {
		t.Fatal("RLS leak", count, e)
	}
	tx.Rollback(ctx)
	if e := s.QueueDeployment(ca, next(8443)); !errors.Is(e, ErrConflict) {
		t.Fatal("parallel operation accepted", e)
	}
	assertNodes := func(scope context.Context, count int, state string) []Node {
		t.Helper()
		nodes, e := s.Nodes(scope)
		if e != nil || len(nodes) != count {
			t.Fatalf("nodes count: got %d want %d: %v", len(nodes), count, e)
		}
		if count > 0 && (nodes[0].State != state || nodes[0].InstalledAt.IsZero() || nodes[0].HostName != h.Name || nodes[0].Address != h.Address) {
			t.Fatalf("incorrect node metadata: %+v", nodes[0])
		}
		encoded, _ := json.Marshal(nodes)
		if strings.Contains(string(encoded), "encrypted") || strings.Contains(string(encoded), "agent_hash") || strings.Contains(string(encoded), "credential") {
			t.Fatal("node secrets exposed")
		}
		return nodes
	}
	assertNodes(ca, 0, "") // Queued installation is not yet a node.
	if e := s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.6.0-dev", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ClaimDeployment(ctx, hash); !errors.Is(e, ErrConflict) {
		t.Fatal("old agent claimed prefixed-ID work", e)
	}
	if e := s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	claimed, e := s.ClaimDeployment(ctx, hash)
	if e != nil || claimed == nil {
		t.Fatal("claim", e)
	}
	var remaining float64
	if err := admin.Pool.QueryRow(ctx, "SELECT extract(epoch FROM lease_until-now()) FROM protocol_deployments WHERE id=$1", d.ID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining < 9*60 || remaining > 10*60 {
		t.Fatalf("download lease has insufficient or unbounded budget: %v", remaining)
	}
	again, e := s.ClaimDeployment(ctx, hash)
	if e != nil || again.Lease != claimed.Lease || again.OperationID != claimed.OperationID {
		t.Fatal("reclaim changed immutable operation", e)
	}
	if e := s.FinishDeployment(ctx, machine.Hash(machine.Token()), d.OperationID, claimed.Lease, true, "deployed"); !errors.Is(e, ErrNotFound) {
		t.Fatal("foreign identity", e)
	}
	if e := s.FinishDeployment(ctx, hash, d.OperationID, NewID("lease"), true, "deployed"); !errors.Is(e, ErrConflict) {
		t.Fatal("foreign lease", e)
	}
	if e := s.FinishDeployment(ctx, hash, d.OperationID, claimed.Lease, true, "removed"); !errors.Is(e, ErrInvalid) {
		t.Fatal("mismatched success", e)
	}
	if e := s.FinishDeployment(ctx, hash, d.OperationID, claimed.Lease, true, "deployed"); e != nil {
		t.Fatal(e)
	}
	nodes := assertNodes(ca, 1, "succeeded")
	installedAt := nodes[0].InstalledAt
	if e := s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.6.0-dev", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if e := s.RestartDeployment(ca, h.ID, d.ID); !errors.Is(e, ErrConflict) {
		t.Fatal("old agent restart", e)
	}
	if e := s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	if e := s.ReportServices(ctx, hash, []protocol.ServiceStatus{{ID: d.ID, Status: "active"}}); e != nil {
		t.Fatal(e)
	}
	checked := assertNodes(ca, 1, "succeeded")[0]
	if checked.ServiceStatus != "active" || checked.ServiceCheckedAt == nil {
		t.Fatal("missing service report")
	}
	if e := s.ReportServices(ctx, hash, []protocol.ServiceStatus{{ID: d.ID, Status: "arbitrary output"}}); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded report", e)
	}
	if e := s.ReportServices(ctx, hash, []protocol.ServiceStatus{{ID: NewID("node"), Status: "active"}}); !errors.Is(e, ErrNotFound) {
		t.Fatal("foreign node report", e)
	}
	if e := s.RestartDeployment(cb, h.ID, d.ID); e == nil {
		t.Fatal("cross tenant restart")
	}
	if e := s.RestartDeployment(ca, h.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	restarted, e := s.ClaimDeployment(ctx, hash)
	if e != nil || restarted == nil || restarted.Action != "restart" {
		t.Fatal("restart claim", e)
	}
	if e := s.FinishDeployment(ctx, hash, restarted.OperationID, restarted.Lease, true, "deployed"); !errors.Is(e, ErrInvalid) {
		t.Fatal("restart wrong result", e)
	}
	if e := s.FinishDeployment(ctx, hash, restarted.OperationID, restarted.Lease, true, "restarted"); e != nil {
		t.Fatal(e)
	}
	if e := s.FinishDeployment(ctx, hash, restarted.OperationID, restarted.Lease, true, "restarted"); e != nil {
		t.Fatal("restart ack replay", e)
	}
	if nodes := assertNodes(ca, 1, "succeeded"); nodes[0].Action != "deploy" || !nodes[0].InstalledAt.Equal(installedAt) {
		t.Fatal("restart changed node identity")
	}

	assertNodes(cb, 0, "")
	admin.Pool.Exec(ctx, "UPDATE hosts SET last_seen_at=now()-interval '91 seconds' WHERE id=$1", h.ID)
	if nodes := assertNodes(ca, 1, "succeeded"); nodes[0].HostStatus != "offline" {
		t.Fatal("stale agent shown online")
	}
	s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1})
	list, e := s.Deployments(ca, h.ID)
	if e != nil || len(list) != 1 || list[0].State != "succeeded" {
		t.Fatal("state", list, e)
	}
	encoded, _ := json.Marshal(list)
	if strings.Contains(string(encoded), "encrypted-only") || strings.Contains(string(encoded), "agent_hash") {
		t.Fatal("list secret leak")
	}
	admin.Pool.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'member')", org, b)
	member := WithTenant(ctx, b, org)
	assertNodes(member, 1, "succeeded")
	admin.Pool.Exec(ctx, "UPDATE memberships SET role='viewer' WHERE organization_id=$1 AND user_id=$2", org, b)
	assertNodes(member, 1, "succeeded")
	admin.Pool.Exec(ctx, "UPDATE memberships SET role='member' WHERE organization_id=$1 AND user_id=$2", org, b)
	if _, e := s.Deployments(member, h.ID); e != nil {
		t.Fatal("member read", e)
	}
	if _, e := s.DeploymentSecret(member, h.ID, d.ID); !errors.Is(e, ErrForbidden) {
		t.Fatal("member secret", e)
	}
	if e := s.RemoveDeployment(member, h.ID, d.ID); !errors.Is(e, ErrForbidden) {
		t.Fatal("member remove", e)
	}
	if e := s.QueueDeployment(member, next(8443)); !errors.Is(e, ErrForbidden) {
		t.Fatal("member create", e)
	}
	if e := s.QueueDeployment(ca, next(443)); !errors.Is(e, ErrConflict) {
		t.Fatal("reserved port", e)
	}
	// Revoking management access does not uninstall the already deployed service.
	if e := s.RevokeMachine(ca, h.ID); e != nil {
		t.Fatal(e)
	}
	if nodes := assertNodes(ca, 1, "succeeded"); nodes[0].HostStatus != "offline" {
		t.Fatal("revoked agent shown online")
	}
	hash = enroll("manage")
	if e := s.RemoveDeployment(ca, h.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	assertNodes(ca, 1, "queued")
	rem, e := s.ClaimDeployment(ctx, hash)
	if e != nil || rem.OperationID == claimed.OperationID || rem.Action != "remove" {
		t.Fatal("remove not new operation", e)
	}
	assertNodes(ca, 1, "running")
	if e := s.FinishDeployment(ctx, hash, rem.OperationID, rem.Lease, false, "runtime_failed"); e != nil {
		t.Fatal(e)
	}
	if nodes := assertNodes(ca, 1, "failed"); !nodes[0].InstalledAt.Equal(installedAt) {
		t.Fatal("removal changed installation time")
	}
	if e := s.RemoveDeployment(ca, h.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	rem, e = s.ClaimDeployment(ctx, hash)
	if e != nil || rem == nil {
		t.Fatal("retry remove", e)
	}
	admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET lease_until=now()-interval '1 second' WHERE id=$1", d.ID)
	if _, e := s.ClaimDeployment(ctx, hash); e != nil {
		t.Fatal(e)
	}
	assertNodes(ca, 1, "interrupted")
	if e := s.RemoveDeployment(ca, h.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	rem, e = s.ClaimDeployment(ctx, hash)
	if e != nil || rem == nil {
		t.Fatal("retry interrupted remove", e)
	}
	if e := s.FinishDeployment(ctx, hash, rem.OperationID, rem.Lease, true, "removed"); e != nil {
		t.Fatal(e)
	}
	assertNodes(ca, 0, "")
	var cleared bool
	if e := admin.Pool.QueryRow(ctx, "SELECT encrypted IS NULL FROM protocol_deployments WHERE id=$1", d.ID).Scan(&cleared); e != nil || !cleared {
		t.Fatal("removed secret retained", e)
	}
	expired := next(443)
	if e := s.QueueDeployment(ca, expired); e != nil {
		t.Fatal(e)
	}
	admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET queued_at=now()-interval '31 minutes' WHERE id=$1", expired.ID)
	if x, e := s.ClaimDeployment(ctx, hash); e != nil || x != nil {
		t.Fatal("expired queued claimed", e)
	}
	running := next(443)
	if e := s.QueueDeployment(ca, running); e != nil {
		t.Fatal(e)
	}
	r, e := s.ClaimDeployment(ctx, hash)
	if e != nil {
		t.Fatal(e)
	}
	admin.Pool.Exec(ctx, "UPDATE protocol_deployments SET lease_until=now()-interval '1 second' WHERE id=$1", running.ID)
	if x, e := s.ClaimDeployment(ctx, hash); e != nil || x != nil {
		t.Fatal("expired lease reclaimed", e)
	}
	if e := s.FinishDeployment(ctx, hash, r.OperationID, r.Lease, true, "deployed"); !errors.Is(e, ErrConflict) {
		t.Fatal("stale completion", e)
	}
	cancelled := next(8443)
	if e := s.QueueDeployment(ca, cancelled); e != nil {
		t.Fatal(e)
	}
	if e := s.RevokeMachine(ca, h.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ClaimDeployment(ctx, hash); !errors.Is(e, ErrNotFound) {
		t.Fatal("revoked claim", e)
	}
	if e := admin.Pool.QueryRow(ctx, "SELECT encrypted IS NULL FROM protocol_deployments WHERE id=$1", cancelled.ID).Scan(&cleared); e != nil || !cleared {
		t.Fatal("queued revoked secret retained", e)
	}
	// A new identity cannot claim work bound to the previous enrollment.
	hash = enroll("manage")
	unauthorized := next(9443)
	if e := s.QueueDeployment(ca, unauthorized); e != nil {
		t.Fatal(e)
	}
	hash = enroll("manage")
	if x, e := s.ClaimDeployment(ctx, hash); e != nil || x != nil {
		t.Fatal("replacement identity claimed old work", e)
	}
	// Authority is checked again at claim, after a committed role change.
	admin.Pool.Exec(ctx, "UPDATE memberships SET role='admin' WHERE organization_id=$1 AND user_id=$2", org, b)
	demoted := next(10443)
	if e := s.QueueDeployment(member, demoted); e != nil {
		t.Fatal(e)
	}
	admin.Pool.Exec(ctx, "UPDATE memberships SET role='member' WHERE organization_id=$1 AND user_id=$2", org, b)
	if x, e := s.ClaimDeployment(ctx, hash); e != nil || x != nil {
		t.Fatal("demoted initiator claimed", e)
	}
	// New protocols require a capable Agent at both queue and claim time.
	for i, kind := range []string{"shadowsocks", "shadowsocks2022"} {
		candidate := next(24440 + i)
		candidate.Protocol, candidate.ServerName = kind, ""
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1})
		if e := s.DeploymentPreflight(ca, h.ID, candidate.Port, kind); !errors.Is(e, ErrConflict) {
			t.Fatal("old agent preflight accepted", kind, e)
		}
		if e := s.QueueDeployment(ca, candidate); !errors.Is(e, ErrConflict) {
			t.Fatal("old agent accepted new protocol", kind, e)
		}
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.8.0-dev", CPUs: 1})
		if e := s.DeploymentPreflight(ca, h.ID, candidate.Port, kind); e != nil {
			t.Fatal("capable agent preflight", e)
		}
		if e := s.QueueDeployment(ca, candidate); e != nil {
			t.Fatal("new protocol queue", e)
		}
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1})
		if _, e := s.ClaimDeployment(ctx, hash); !errors.Is(e, ErrConflict) {
			t.Fatal("downgraded agent claimed new protocol", e)
		}
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.9.0", CPUs: 1})
		claimed, e := s.ClaimDeployment(ctx, hash)
		if e != nil || claimed == nil || claimed.Protocol != kind {
			t.Fatal("newer agent claim", e)
		}
		if e = s.FinishDeployment(ctx, hash, claimed.OperationID, claimed.Lease, true, "deployed"); e != nil {
			t.Fatal(e)
		}
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.7.0-dev", CPUs: 1})
		if e = s.RestartDeployment(ca, h.ID, candidate.ID); !errors.Is(e, ErrConflict) {
			t.Fatal("old Agent restarted SS", e)
		}
		if e = s.RemoveDeployment(ca, h.ID, candidate.ID); !errors.Is(e, ErrConflict) {
			t.Fatal("old Agent removed SS", e)
		}
		s.Heartbeat(ctx, hash, machine.Metrics{Version: "0.9.0", CPUs: 1})
		if e = s.RestartDeployment(ca, h.ID, candidate.ID); e != nil {
			t.Fatal("newer Agent restart", e)
		}
		restarted, e := s.ClaimDeployment(ctx, hash)
		if e != nil || restarted == nil {
			t.Fatal("restart claim", e)
		}
		if e = s.FinishDeployment(ctx, hash, restarted.OperationID, restarted.Lease, true, "restarted"); e != nil {
			t.Fatal(e)
		}
		if e = s.RemoveDeployment(ca, h.ID, candidate.ID); e != nil {
			t.Fatal(e)
		}
		removal, e := s.ClaimDeployment(ctx, hash)
		if e != nil || removal == nil {
			t.Fatal("SS removal", e)
		}
		if e = s.FinishDeployment(ctx, hash, removal.OperationID, removal.Lease, true, "removed"); e != nil {
			t.Fatal(e)
		}
	}

}
