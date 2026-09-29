package agent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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
