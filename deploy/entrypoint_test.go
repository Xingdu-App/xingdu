package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIEntrypoint(t *testing.T) {
	for _, tc := range []struct {
		name, mode, admin      string
		failMigration, success bool
		expected               string
	}{
		{"migrate before server", "true", "migration-fixture", false, true, "migrate\nserver\n"},
		{"default migration", "", "migration-fixture", false, true, "migrate\nserver\n"},
		{"migration failure blocks server", "true", "migration-fixture", true, false, "migrate\n"},
		{"missing owner blocks server", "true", "", false, false, ""},
		{"invalid switch blocks server", "yes", "migration-fixture", false, false, ""},
		{"external release migration", "false", "migration-fixture", false, true, "server\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "order")
			write := func(name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nset -eu\n"+body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write("migrate", `[ "$DATABASE_URL" = "migration-fixture" ]
[ "$XINGDU_APP_DATABASE_PASSWORD" = "app-fixture" ]
printf 'migrate\n' >> "$MARKER"
[ "$FAIL_MIGRATION" != "true" ]
`)
			write("server", `[ "$DATABASE_URL" = "runtime-fixture" ]
[ "${XINGDU_MIGRATION_DATABASE_URL+x}" != x ]
[ "${XINGDU_APP_DATABASE_PASSWORD+x}" != x ]
[ "${XINGDU_WORKER_DATABASE_PASSWORD+x}" != x ]
[ "$1" = "forwarded-argument" ]
printf 'server\n' >> "$MARKER"
`)
			c := exec.Command("sh", "api-entrypoint.sh", "server", "forwarded-argument")
			c.Env = []string{"PATH=" + dir + ":" + os.Getenv("PATH"), "MARKER=" + marker, "DATABASE_URL=runtime-fixture", "XINGDU_MIGRATION_DATABASE_URL=" + tc.admin, "XINGDU_APP_DATABASE_PASSWORD=app-fixture", "XINGDU_WORKER_DATABASE_PASSWORD=worker-fixture"}
			if tc.mode != "" {
				c.Env = append(c.Env, "XINGDU_AUTO_MIGRATE="+tc.mode)
			}
			if tc.failMigration {
				c.Env = append(c.Env, "FAIL_MIGRATION=true")
			} else {
				c.Env = append(c.Env, "FAIL_MIGRATION=false")
			}
			out, err := c.CombinedOutput()
			if (err == nil) != tc.success {
				t.Fatalf("unexpected exit: %v %s", err, out)
			}
			for _, secret := range []string{"migration-fixture", "runtime-fixture", "app-fixture", "worker-fixture"} {
				if strings.Contains(string(out), secret) {
					t.Fatal("credential leaked")
				}
			}
			b, e := os.ReadFile(marker)
			if e != nil && !os.IsNotExist(e) {
				t.Fatal(e)
			}
			if string(b) != tc.expected {
				t.Fatalf("startup order = %q", b)
			}
		})
	}
}
