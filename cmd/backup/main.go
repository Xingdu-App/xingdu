// backup is an offline operator tool. Its PostgreSQL role is separate from the
// application's RLS-limited runtime role; run only against trusted databases.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/backup"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: backup create|restore|keygen --file PATH [--confirm-database NAME]")
	}
	action := os.Args[1]
	if action == "keygen" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		fmt.Println(base64.StdEncoding.EncodeToString(key))
		return nil
	}
	if action != "create" && action != "restore" {
		return errors.New("unknown backup action")
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	path := flags.String("file", "", "encrypted backup path")
	confirm := flags.String("confirm-database", "", "explicit empty destination database name for restore")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *path == "" || flags.NArg() != 0 {
		return errors.New("specify --file PATH")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("XINGDU_BACKUP_KEY"))
	if err != nil || len(key) != 32 {
		return errors.New("XINGDU_BACKUP_KEY must be base64 encoding of a separate 32-byte backup key")
	}
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,62}$`).MatchString(os.Getenv("PGDATABASE")) {
		return errors.New("set PGDATABASE to a plain database name (letters, numbers, underscore, hyphen), with connection credentials in PostgreSQL environment")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if action == "create" {
		return create(ctx, *path, key)
	}
	if *confirm == "" || *confirm != os.Getenv("PGDATABASE") {
		return errors.New("restore requires --confirm-database matching PGDATABASE; destination must be empty")
	}
	return restore(ctx, *path, key, *confirm)
}
func create(ctx context.Context, path string, key []byte) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("backup destination already exists")
	} else if !os.IsNotExist(err) {
		return errors.New("cannot inspect backup destination")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".xingdu-backup-*")
	if err != nil {
		return errors.New("cannot create private backup file")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-password")
	// libpq reads PGHOST/PGPORT/PGUSER/PGDATABASE/PGPASSWORD/PGPASSFILE from
	// the environment. Never put a URL/password in argv or print stderr.
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.New("cannot initialize pg_dump")
	}
	if err = cmd.Start(); err != nil {
		return errors.New("cannot start pg_dump; install a matching PostgreSQL client")
	}
	encryptErr := backup.Encrypt(file, stdout, key)
	if encryptErr != nil {
		_ = cmd.Process.Kill()
	}
	dumpErr := cmd.Wait()
	if encryptErr != nil || dumpErr != nil {
		return errors.New("backup failed; no completed archive was published")
	}
	if err = file.Sync(); err != nil {
		return errors.New("cannot sync backup archive")
	}
	if err = file.Close(); err != nil {
		return errors.New("cannot finish backup archive")
	}
	// Hard-link publication refuses to replace an existing path, including a
	// symlink created after preflight. The temp file is on the same filesystem.
	if err = os.Link(file.Name(), path); err != nil {
		return errors.New("cannot publish backup; destination must not already exist")
	}
	fmt.Println("Encrypted backup created.")
	return nil
}
func restore(ctx context.Context, path string, key []byte, database string) error {
	source, err := os.Open(path)
	if err != nil {
		return errors.New("cannot open backup archive")
	}
	defer source.Close()
	file, err := os.CreateTemp("", "xingdu-restore-*.dump")
	if err != nil {
		return errors.New("cannot create private restore file")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	// Authenticate the terminal record and EOF before even connecting to the
	// destination. Partial, reordered, appended or tampered archives never restore.
	if err = backup.Decrypt(file, source, key); err != nil {
		return backup.ErrArchive
	}
	if err = file.Close(); err != nil {
		return errors.New("cannot finish authenticated restore file")
	}
	config, err := pgx.ParseConfig("")
	if err != nil {
		return errors.New("invalid PostgreSQL environment")
	}
	db, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("cannot connect to restore destination")
	}
	defer db.Close(ctx)
	var name string
	var empty bool
	if err = db.QueryRow(ctx, `SELECT current_database(), NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT IN ('public','information_schema') AND nspname NOT LIKE 'pg_%') AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public') AND NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public') AND NOT EXISTS(SELECT 1 FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public')`).Scan(&name, &empty); err != nil {
		return errors.New("cannot verify restore destination")
	}
	if name != database || !empty {
		return errors.New("restore refused: destination must be the explicitly confirmed empty database")
	}
	cmd := exec.CommandContext(ctx, "pg_restore", "--dbname", database, "--single-transaction", "--exit-on-error", "--no-password", file.Name())
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Run(); err != nil {
		return errors.New("restore failed; PostgreSQL transaction rolled back; verify matching client, roles and empty target")
	}
	fmt.Println("Backup restored into the confirmed empty database.")
	return nil
}
