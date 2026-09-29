package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
)

func upgradeFixture(t *testing.T) (UpgradeExpectation, upgradePaths, upgradeCommand) {
	t.Helper()
	dir := t.TempDir()
	p := upgradePaths{filepath.Join(dir, "agent"), filepath.Join(dir, "unit"), filepath.Join(dir, "config.json"), filepath.Join(dir, "new")}
	c := Config{Server: "https://control.example.invalid", Token: machine.Token(), Mode: "manage"}
	if err := save(p.config, c, true); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{p.binary: "old binary", p.source: "new binary", p.unit: ServiceUnit("root", "manage")} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == p.binary {
			return []byte("xingdu-agent 0.8.0-dev\n"), nil
		}
		if len(args) > 2 && args[0] == "show" && args[3] == "FragmentPath" {
			return []byte(p.unit), nil
		}
		return nil, nil
	}
	return UpgradeExpectation{Server: c.Server, TokenHash: machine.Hash(c.Token), Mode: c.Mode, Version: machine.Version}, p, run
}
func TestUpgradePreservesIdentityAndUnit(t *testing.T) {
	expected, p, run := upgradeFixture(t)
	before, _ := os.ReadFile(p.config)
	unit, _ := os.ReadFile(p.unit)
	if err := upgrade(context.Background(), expected, p, run); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p.config)
	afterUnit, _ := os.ReadFile(p.unit)
	binary, _ := os.ReadFile(p.binary)
	if string(before) != string(after) || string(unit) != string(afterUnit) || string(binary) != "new binary" {
		t.Fatal("upgrade changed identity/unit or did not replace binary")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(p.binary), ".xingdu-upgrade-*"))
	if len(files) != 0 {
		t.Fatal("upgrade temporary files retained")
	}
}
func TestUpgradeRejectsForeignOrCustomizedInstall(t *testing.T) {
	for _, change := range []string{"identity", "origin", "mode", "version", "unit", "dropin", "downgrade", "symlink"} {
		t.Run(change, func(t *testing.T) {
			e, p, run := upgradeFixture(t)
			switch change {
			case "identity":
				e.TokenHash = machine.Hash(machine.Token())
			case "origin":
				e.Server = "https://other.example.invalid"
			case "mode":
				e.Mode = "monitor"
			case "version":
				e.Version = "0.10.0"
			case "unit":
				os.WriteFile(p.unit, []byte("custom unit"), 0600)
			case "dropin":
				base := run
				run = func(c context.Context, n string, a ...string) ([]byte, error) {
					if len(a) > 3 && a[3] == "DropInPaths" {
						return []byte("override.conf"), nil
					}
					return base(c, n, a...)
				}
			case "downgrade":
				base := run
				run = func(c context.Context, n string, a ...string) ([]byte, error) {
					if n == p.binary {
						return []byte("xingdu-agent 1.0.0"), nil
					}
					return base(c, n, a...)
				}
			case "symlink":
				os.Remove(p.source)
				os.Symlink(p.binary, p.source)
			}
			if err := upgrade(context.Background(), e, p, run); err == nil {
				t.Fatal("unsafe upgrade allowed")
			}
			b, _ := os.ReadFile(p.binary)
			if string(b) != "old binary" {
				t.Fatal("rejected upgrade changed binary")
			}
		})
	}
}
func TestUpgradeRollsBackFailedService(t *testing.T) {
	e, p, base := upgradeFixture(t)
	restarts := 0
	run := func(c context.Context, n string, a ...string) ([]byte, error) {
		if len(a) > 0 && a[0] == "restart" {
			restarts++
			if restarts == 1 {
				return nil, errors.New("failed start")
			}
		}
		return base(c, n, a...)
	}
	err := upgrade(context.Background(), e, p, run)
	if err == nil || !strings.Contains(err.Error(), "previous executable restored") || restarts != 2 {
		t.Fatal("rollback missing", err)
	}
	b, _ := os.ReadFile(p.binary)
	if string(b) != "old binary" {
		t.Fatal("old executable not restored")
	}
}
