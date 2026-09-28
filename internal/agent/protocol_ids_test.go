package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

func TestAgentRejectsUntypedAndWrongResourceIDs(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	cases := []protocol.Task{task, task, task, task}
	cases[0].ID = "00000000-0000-4000-8000-000000000001"
	cases[1].DeploymentID = resourceid.New("srv")
	cases[2].Lease = resourceid.New("op")
	cases[3].DeploymentID = "node_../../other"
	for _, invalid := range cases {
		if r := x.apply(context.Background(), c, invalid); r.Code != "invalid_task" {
			t.Fatal(r.Code)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("invalid task reached execution")
	}
	if x.owned("00000000-0000-4000-8000-000000000002") {
		t.Fatal("public UUID accepted")
	}
}

func legacyInstance(t *testing.T, x *protocolExecutor, node string) string {
	t.Helper()
	old := legacyResourceUUID("node", node)
	if err := os.MkdirAll(filepath.Join(x.stateDir, old), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(x.stateDir, old, "owner"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(x.stateDir, old, "config.json"), []byte("existing config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(x.unitDir, serviceName(old)), []byte(protocolUnit(old, "/trusted/runtime", filepath.Join(x.stateDir, old, "config.json"))), 0644); err != nil {
		t.Fatal(err)
	}
	return old
}
func TestMigratedNodeKeepsLegacyServiceWithoutRenaming(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	old := legacyInstance(t, x, task.DeploymentID)
	if got := x.serviceStatus(context.Background(), task.DeploymentID); got != "active" {
		t.Fatal(got)
	}
	task.Action = "restart"
	task.Spec = protocol.Spec{}
	if got := x.apply(context.Background(), c, task); !got.Success || got.Code != "restarted" {
		t.Fatal(got)
	}
	if got := strings.Join(*calls, "\n"); !strings.Contains(got, "restart "+serviceName(old)) || strings.Contains(got, serviceName(task.DeploymentID)) {
		t.Fatal("did not target legacy service")
	}
	if raw, err := os.ReadFile(filepath.Join(x.stateDir, old, "config.json")); err != nil || string(raw) != "existing config" {
		t.Fatal("legacy config changed")
	}
	if _, err := os.Lstat(filepath.Join(x.stateDir, task.DeploymentID)); !os.IsNotExist(err) {
		t.Fatal("created duplicate node directory")
	}
	task.ID = resourceid.New("op")
	task.Action = "remove"
	if got := x.apply(context.Background(), c, task); !got.Success || got.Code != "removed" {
		t.Fatal(got)
	}
	if _, err := os.Lstat(filepath.Join(x.stateDir, old)); !os.IsNotExist(err) {
		t.Fatal("legacy instance retained after confirmed removal")
	}
}
func TestNewPathNeverFallsBackToLegacyOwnedInstance(t *testing.T) {
	x, c, calls := executorFixture(t)
	task := taskFixture(t)
	old := legacyInstance(t, x, task.DeploymentID)
	if err := os.Mkdir(filepath.Join(x.stateDir, task.DeploymentID), 0700); err != nil {
		t.Fatal(err)
	}
	task.Action = "remove"
	if got := x.apply(context.Background(), c, task); got.Code != "ownership_mismatch" {
		t.Fatal(got)
	}
	if len(*calls) != 0 {
		t.Fatal("ambiguous new path changed legacy service")
	}
	if _, err := os.Stat(filepath.Join(x.stateDir, old, "owner")); err != nil {
		t.Fatal("legacy ownership removed")
	}
}
func TestMigratedOutcomeReplayDoesNotExecuteAgain(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "interrupted"}[pending], func(t *testing.T) {
			x, c, calls := executorFixture(t)
			task := taskFixture(t)
			legacy := task
			legacy.ID = legacyResourceUUID("op", task.ID)
			legacy.DeploymentID = legacyResourceUUID("node", task.DeploymentID)
			journal := protocolJournal{Fingerprint: taskFingerprint(legacy)}
			if !pending {
				journal.Result = &protocol.Result{ID: legacy.ID, Lease: "legacy-lease", Success: true, Code: "deployed"}
			}
			dir := filepath.Join(x.stateDir, "outcomes")
			if err := secureDir(dir); err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(journal)
			if err := os.WriteFile(filepath.Join(dir, legacy.ID+".json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			result := x.apply(context.Background(), c, task)
			if result.ID != task.ID || result.Lease != task.Lease || (pending && result.Code != "interrupted") || (!pending && !result.Success) {
				t.Fatal(result)
			}
			if len(*calls) != 0 {
				t.Fatal("legacy outcome replay executed task again")
			}
			task.Spec.Name = "changed"
			if got := x.apply(context.Background(), c, task); got.Code != "journal_conflict" {
				t.Fatal("legacy fingerprint was not checked", got)
			}
		})
	}
}
