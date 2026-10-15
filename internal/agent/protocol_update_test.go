package agent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestUpdateRestoresRunningConfiguration(t *testing.T) {
	x, c, _ := executorFixture(t)
	task := taskFixture(t)
	ctx := context.Background()
	if r := x.apply(ctx, c, task); !r.Success {
		t.Fatal(r.Code)
	}
	path := filepath.Join(x.stateDir, task.DeploymentID, "config.json")
	before, _ := os.ReadFile(path)
	task.Action = "update"
	task.ID = "op_00000000000040008000000000000009"
	task.Spec.ServerName = "invalid.example"
	if r := x.apply(ctx, c, task); r.Code != "invalid_spec" {
		t.Fatal(r.Code)
	}
	task = taskFixture(t)
	task.Action = "update"
	task.ID = "op_0000000000004000800000000000000a"
	task.Spec.Credential = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	restarts := 0
	run := x.run
	x.run = func(ctx context.Context, name string, args ...string) error {
		if len(args) > 0 && args[0] == "restart" {
			restarts++
			if restarts == 1 {
				return errors.New("failed")
			}
		}
		return run(ctx, name, args...)
	}
	if r := x.apply(ctx, c, task); r.Success || r.Code != "update_rolled_back" {
		t.Fatal(r.Code)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) || restarts != 2 {
		t.Fatal("old config not restored")
	}
	task.ID = "op_0000000000004000800000000000000b"
	if r := x.apply(ctx, c, task); !r.Success || r.Code != "updated" {
		t.Fatal(r.Code)
	}
	after, _ = os.ReadFile(path)
	if bytes.Equal(before, after) {
		t.Fatal("configuration unchanged")
	}
	n := restarts
	if r := x.apply(ctx, c, task); !r.Success || restarts != n {
		t.Fatal("update replayed")
	}
}

func TestUpdateRecognizesEndpointAndMultiInboundLayouts(t *testing.T) {
	for _, kind := range []string{"wireguard", "shadowtls"} {
		t.Run(kind, func(t *testing.T) {
			x, c, _ := executorFixture(t)
			task := taskFixture(t)
			in := protocol.Input{Name: "fixture", Protocol: kind, Port: task.Spec.Port}
			if kind == "wireguard" {
				in.WireGuard = &protocol.WireGuardOptions{MTU: 1380, Preshared: true}
			} else {
				in.ServerName = "www.microsoft.com"
			}
			var err error
			task.Spec, err = protocol.NewSpec(in)
			if err != nil {
				t.Fatal(err)
			}
			if r := x.apply(context.Background(), c, task); !r.Success {
				t.Fatal(r.Code)
			}
			task.Action = "update"
			task.ID = "op_00000000000040008000000000000009"
			if kind == "wireguard" {
				task.Spec.WireGuard.MTU = 1400
			} else {
				task.Spec.Name = "renamed"
			}
			if r := x.apply(context.Background(), c, task); !r.Success || r.Code != "updated" {
				t.Fatal(r.Code)
			}
		})
	}
}

func TestSnellUDPUpdateRequiresGuardedService(t *testing.T) {
	for _, guarded := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "guarded"}[guarded], func(t *testing.T) {
			x, c, _ := executorFixture(t)
			task := taskFixture(t)
			var err error
			task.Spec, err = protocol.NewSpec(protocol.Input{Name: "Snell", Protocol: "snell6", Port: task.Spec.Port})
			if err != nil {
				t.Fatal(err)
			}
			if r := x.apply(context.Background(), c, task); !r.Success {
				t.Fatal(r.Code)
			}
			config := filepath.Join(x.stateDir, task.DeploymentID, "config.json")
			before, _ := os.ReadFile(config)
			if !guarded {
				unit := filepath.Join(x.unitDir, serviceName(task.DeploymentID))
				data, err := os.ReadFile(unit)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(strings.ReplaceAll(string(data), "# Xingdu official runtime egress guard\nIPAddressDeny="+officialDeniedNetworks+"\n", ""))
				if err = os.WriteFile(unit, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			task.Action = "update"
			task.ID = "op_00000000000040008000000000000009"
			task.Spec.UDPEnabled = true
			result := x.apply(context.Background(), c, task)
			if guarded {
				if !result.Success {
					t.Fatal(result.Code)
				}
			} else {
				if result.Code != "runtime_change_requires_new_node" {
					t.Fatal(result.Code)
				}
				after, _ := os.ReadFile(config)
				if !bytes.Equal(before, after) {
					t.Fatal("unsafe update changed configuration")
				}
			}
		})
	}
}
