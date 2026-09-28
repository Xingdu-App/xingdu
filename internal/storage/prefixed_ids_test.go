package storage

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/vault"
)

func TestPrefixedIDMigrationPreservesDataPoliciesAndEncryptedAAD(t *testing.T) {
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
	name := "xingdu_ids_" + NewID("obj")[4:]
	if _, err = admin.Pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	uri, _ := url.Parse(raw)
	uri.Path = "/" + name
	s, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Pool.Exec(ctx, "CREATE TABLE schema_migrations(version text PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "021_prefixed_ids.sql" {
			continue
		}
		body, _ := migrations.ReadFile("migrations/" + entry.Name())
		if _, err = s.Pool.Exec(ctx, string(body)); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		if _, err = s.Pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name()); err != nil {
			t.Fatal(err)
		}
	}
	usr, org, host, node, op, lease, session, invite, job, sub, attempt := legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID(), legacyTestUUID()
	key := strings.Repeat("ac", 32)
	v, _ := vault.New(key)
	plain := []byte(`{"fixture":"encrypted credential material"}`)
	seal := func(namespace, object string) []byte {
		return v.Seal(plain, strings.Join([]string{namespace, org, host, object}, ":"))
	}
	execute := func(query string, args ...any) {
		t.Helper()
		if _, e := s.Pool.Exec(ctx, query, args...); e != nil {
			t.Fatal(e)
		}
	}
	execute("INSERT INTO users(id,username,password_hash) VALUES($1,'migration-fixture','fixture-hash')", usr)
	execute("INSERT INTO organizations(id,name,created_by) VALUES($1,'Migration fixture',$2)", org, usr)
	execute("INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')", org, usr)
	execute("INSERT INTO hosts(id,organization_id,name,address) VALUES($1,$2,'fixture','fixture.example.invalid')", host, org)
	execute("INSERT INTO sessions(id,token_hash,user_id,expires_at) VALUES($1,'migration-session',$2,now()+interval '1 hour')", session, usr)
	execute("INSERT INTO invitations(id,organization_id,created_by,role,token_hash,expires_at) VALUES($1,$2,$3,'viewer','migration-invite',now()+interval '1 hour')", invite, org, usr)
	execute("INSERT INTO machine_agents(host_id,organization_id,token_hash,mode,metrics) VALUES($1,$2,'migration-agent','manage','{\"version\":\"0.7.0-dev\"}')", host, org)
	execute("INSERT INTO machine_credentials(host_id,organization_id,method,fingerprint,encrypted) VALUES($1,$2,'password','fixture',$3)", host, org, seal("xingdu-ssh-v1", "saved"))
	execute("INSERT INTO machine_jobs(id,organization_id,host_id,created_by,mode,state,encrypted,lease) VALUES($1,$2,$3,$4,'manage','queued',$5,$6)", job, org, host, usr, seal("xingdu-ssh-v1", job), lease)
	execute("INSERT INTO protocol_deployments(id,organization_id,host_id,created_by,name,protocol,port,server_name,state,operation_id,agent_hash,encrypted,lease,installed_at) VALUES($1,$2,$3,$4,'fixture','trojan',443,'fixture.example.invalid','succeeded',$5,'migration-agent',$6,$7,now())", node, org, host, usr, op, seal("xingdu-protocol-v1", node), lease)
	execute("INSERT INTO subscriptions(id,organization_id,name,final_action,token_hash) VALUES($1,$2,'fixture','proxy','migration-sub')", sub, org)
	execute("INSERT INTO subscription_nodes(organization_id,subscription_id,node_id,position) VALUES($1,$2,$3,0)", org, sub, node)
	execute("INSERT INTO oauth_identities(provider,subject,user_id,email) VALUES('google','migration-subject',$1,'fixture@example.invalid')", usr)
	execute("INSERT INTO oauth_states(state_hash,browser_hash,provider,mode,encrypted) VALUES($1,$1,'google','login',$2)", strings.Repeat("b", 64), []byte("transient"))
	execute("INSERT INTO organization_billing(organization_id,attempt) VALUES($1,$2)", org, attempt)
	var oldPolicies []string
	if err = s.Pool.QueryRow(ctx, "SELECT array_agg(polrelid::regclass::text || '.' || polname ORDER BY polrelid::regclass::text,polname) FROM pg_policy").Scan(&oldPolicies); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err == nil {
		t.Fatal("encrypted migration accepted missing key")
	}
	var stillUUID bool
	if err = s.Pool.QueryRow(ctx, "SELECT udt_name='uuid' FROM information_schema.columns WHERE table_name='users' AND column_name='id'").Scan(&stillUUID); err != nil || !stillUUID {
		t.Fatal("failed migration changed IDs", err)
	}
	if err = s.Migrate(ctx, strings.Repeat("bd", 32)); err == nil {
		t.Fatal("encrypted migration accepted wrong key")
	}
	if err = s.Migrate(ctx, key); err != nil {
		t.Fatal(err)
	}
	var remainingUUID, policies, states int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND udt_name='uuid'").Scan(&remainingUUID); err != nil || remainingUUID != 0 {
		t.Fatal("UUID columns remain", remainingUUID, err)
	}
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_policy WHERE polrelid::regclass::text || '.' || polname = ANY($1)", oldPolicies).Scan(&policies)
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM oauth_states").Scan(&states)
	if policies != len(oldPolicies) || states != 0 {
		t.Fatal("policy lost or OAuth stale state retained", oldPolicies, policies, states)
	}
	userID, orgID, hostID, nodeID := resourceid.FromUUID("usr", usr), resourceid.FromUUID("org", org), resourceid.FromUUID("srv", host), resourceid.FromUUID("node", node)
	var joined int
	err = s.Pool.QueryRow(ctx, `SELECT count(*) FROM users u JOIN organizations o ON o.created_by=u.id JOIN hosts h ON h.organization_id=o.id JOIN protocol_deployments n ON n.host_id=h.id JOIN subscription_nodes sn ON sn.node_id=n.id JOIN subscriptions sub ON sub.id=sn.subscription_id WHERE u.id=$1 AND o.id=$2 AND h.id=$3 AND n.id=$4`, userID, orgID, hostID, nodeID).Scan(&joined)
	if err != nil || joined != 1 {
		t.Fatal("relationships not preserved", joined, err)
	}
	for _, fixture := range []struct{ table, column, prefix, old, namespace string }{{"machine_credentials", "host_id", "srv", host, "xingdu-ssh-v1"}, {"machine_jobs", "id", "job", job, "xingdu-ssh-v1"}, {"protocol_deployments", "id", "node", node, "xingdu-protocol-v1"}} {
		var cipher []byte
		newID := resourceid.FromUUID(fixture.prefix, fixture.old)
		if err = s.Pool.QueryRow(ctx, "SELECT encrypted FROM "+fixture.table+" WHERE "+fixture.column+"=$1", newID).Scan(&cipher); err != nil {
			t.Fatal(err)
		}
		object := newID
		if fixture.table == "machine_credentials" {
			object = "saved"
		}
		decoded, e := v.Open(cipher, strings.Join([]string{fixture.namespace, orgID, hostID, object}, ":"))
		if e != nil || string(decoded) != string(plain) {
			t.Fatal("ciphertext AAD was not migrated", fixture.table, e)
		}
	}
	// Every security-definer function keeps its narrowly scoped owner and grants.
	var owner string
	if err = s.Pool.QueryRow(ctx, "SELECT pg_get_userbyid(proowner) FROM pg_proc WHERE oid='tenant_role(text)'::regprocedure").Scan(&owner); err != nil || owner != "xingdu_policy" {
		t.Fatal("policy function owner changed", owner, err)
	}
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	app, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	account, err := app.Session(ctx, "migration-session")
	if err != nil || account.ID != userID {
		t.Fatal("session relationship lost", account, err)
	}
	nodes, err := app.Nodes(WithTenant(ctx, userID, orgID))
	if err != nil || len(nodes) != 1 || nodes[0].ID != nodeID {
		t.Fatal("RLS scoped node access lost", err)
	}
	if err = app.NewVerifiedSession(ctx, "prefixed-new-session", userID, "fixture-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var generated string
	if err = s.Pool.QueryRow(ctx, "SELECT id FROM sessions WHERE token_hash='prefixed-new-session'").Scan(&generated); err != nil || !resourceid.Valid("ses", generated) {
		t.Fatal("new session uses wrong ID", generated, err)
	}
	workerURI := *uri
	workerQuery := workerURI.Query()
	workerQuery.Set("options", "-crole=xingdu_worker")
	workerURI.RawQuery = workerQuery.Encode()
	worker, e := Open(ctx, workerURI.String())
	if e != nil {
		t.Fatal(e)
	}
	defer worker.Close()
	var claimedJob, claimedOrg, claimedUser, claimedLease string
	if e = worker.Pool.QueryRow(ctx, `SELECT id,organization_id,created_by,lease FROM claim_machine_job()`).Scan(&claimedJob, &claimedOrg, &claimedUser, &claimedLease); e != nil || claimedJob != resourceid.FromUUID("job", job) || claimedOrg != orgID || claimedUser != userID || !resourceid.Valid("lease", claimedLease) {
		t.Fatal("worker function signature/privileges/lease generation lost", e)
	}
	if _, err = s.Pool.Exec(ctx, "INSERT INTO hosts(id,organization_id,name) VALUES($1,$2,'wrong prefix')", NewID("usr"), orgID); err == nil {
		t.Fatal("typed column accepted wrong prefix")
	}
}
