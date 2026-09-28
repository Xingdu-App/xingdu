package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/backup"
)

func TestRestoreRejectsBeforeDatabaseConnection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xdb")
	if err := os.WriteFile(path, []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	// No reachable server is needed: authentication must precede all DB access.
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGPORT", "1")
	if err := restore(context.Background(), path, make([]byte, 32), "test"); err != backup.ErrArchive {
		t.Fatal("archive did not fail closed", err)
	}
}
func TestPostgresBackupRestore(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_BACKUP_DATABASE_URL")
	if raw == "" {
		t.Skip("requires disposable PostgreSQL administration URL")
	}
	for _, binary := range []string{"pg_dump", "pg_restore"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skip("requires PostgreSQL client tools")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	config, err := pgx.ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	role := "xingdu_backup_role_" + suffix
	if _, err = admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE"); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	source := "xingdu_backup_src_" + suffix
	target := "xingdu_backup_dst_" + suffix
	for _, name := range []string{source, target} {
		if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	}
	c := config.Copy()
	c.Database = source
	src, err := pgx.ConnectConfig(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.Exec(ctx, `CREATE TABLE probe(id integer PRIMARY KEY, secret text NOT NULL); INSERT INTO probe VALUES(1,'fixture-only'); GRANT SELECT ON probe TO `+pgx.Identifier{role}.Sanitize()+`; ALTER TABLE probe ENABLE ROW LEVEL SECURITY; ALTER TABLE probe FORCE ROW LEVEL SECURITY; CREATE POLICY probe_role ON probe TO `+pgx.Identifier{role}.Sanitize()+` USING(id=1); CREATE FUNCTION fixture_definer() RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=public AS 'SELECT secret FROM probe WHERE id=1'; ALTER FUNCTION fixture_definer() OWNER TO `+pgx.Identifier{role}.Sanitize()+``)
	src.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PGHOST", config.Host)
	t.Setenv("PGPORT", fmtPort(config.Port))
	t.Setenv("PGUSER", config.User)
	t.Setenv("PGPASSWORD", config.Password)
	t.Setenv("PGDATABASE", source)
	t.Setenv("PGSSLMODE", "disable")
	path := filepath.Join(t.TempDir(), "archive.xdb")
	key := bytes.Repeat([]byte{3}, 32)
	if err = create(ctx, path, key); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup permissions", err)
	}
	if err = create(ctx, path, key); err == nil {
		t.Fatal("existing backup overwritten")
	}
	t.Setenv("PGDATABASE", target)
	if err = restore(ctx, path, key, target); err != nil {
		t.Fatal(err)
	}
	c.Database = target
	dst, err := pgx.ConnectConfig(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close(ctx)
	var value string
	if err = dst.QueryRow(ctx, "SELECT secret FROM probe WHERE id=1").Scan(&value); err != nil || value != "fixture-only" {
		t.Fatal("data not restored", err)
	}
	var owner string
	var definer, bypass bool
	if err = dst.QueryRow(ctx, "SELECT r.rolname,p.prosecdef,r.rolbypassrls FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.proname='fixture_definer'").Scan(&owner, &definer, &bypass); err != nil || owner != role || !definer || bypass {
		t.Fatal("SECURITY DEFINER owner boundary changed", err)
	}
	if _, err = dst.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if err = dst.QueryRow(ctx, "SELECT secret FROM probe WHERE id=1").Scan(&value); err != nil {
		t.Fatal("ACL/RLS not preserved", err)
	}
	if err = restore(ctx, path, key, target); err == nil {
		t.Fatal("nonempty target overwritten")
	}
}
func fmtPort(port uint16) string { return fmt.Sprint(port) }
