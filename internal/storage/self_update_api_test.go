package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
)

func TestSelfUpdateAPIKeyAuthorization(t *testing.T) {
	for _, phase := range []string{"claim", "execution"} {
		for _, change := range []string{"scope", "revoked", "expired"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
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
				s := openRole("xingdu_app")
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

				key, err := s.CreateAPIKey(scope, "Upgrade key", machine.Hash(machine.Token()), "xd_key_test", []string{"agents:upgrade"}, time.Now().Add(time.Hour))
				if err != nil {
					t.Fatal(err)
				}
				if e = s.QueueInstall(WithAPIKeyTenant(ctx, key), j, "", nil, target); e != nil {
					t.Fatal(e)
				}
				var task *machine.UpdateTask
				if phase == "execution" {
					task, e = s.ClaimSelfUpdate(ctx, hash)
					if e != nil || task == nil {
						t.Fatal("claim", e)
					}
					if e = s.CheckSelfUpdate(ctx, hash, task.ID, task.Lease, "check"); e != nil {
						t.Fatal("valid key denied", e)
					}
				}
				switch change {
				case "scope":
					_, e = s.UpdateAPIKeyScopes(scope, key.ID, []string{"hosts:read"})
				case "revoked":
					e = s.RevokeAPIKey(scope, key.ID)
				case "expired":
					_, e = admin.Pool.Exec(ctx, "UPDATE api_keys SET expires_at=now()-interval '1 second' WHERE id=$1", key.ID)
				}
				if e != nil {
					t.Fatal(e)
				}
				if phase == "claim" {
					task, e = s.ClaimSelfUpdate(ctx, hash)
					if e != nil || task != nil {
						t.Fatal("invalid key claimed job", e)
					}
				} else if e = s.CheckSelfUpdate(ctx, hash, task.ID, task.Lease, "check"); !errors.Is(e, ErrForbidden) {
					t.Fatal("invalid key authorized execution", e)
				}

			})
		}
	}
}
