package storage

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/vault"
)

// preparePrefixedIDEncryption runs in the same transaction as the ID migration.
// Authentication data remains bound to the exact organization, server and object.
// A missing/wrong key aborts the transaction before IDs or ciphertext can diverge.
func preparePrefixedIDEncryption(ctx context.Context, tx pgx.Tx, key string) error {
	// Fail rather than silently resealing an RLS-filtered subset of credentials.
	if _, err := tx.Exec(ctx, "SET LOCAL row_security = off"); err != nil {
		return err
	}
	// Block API/worker writes before taking the encryption snapshot. Migration's
	// advisory lock only coordinates other migrators, not normal application traffic.
	tables, err := tx.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	if err != nil {
		return err
	}
	var names []string
	for tables.Next() {
		var name string
		if err = tables.Scan(&name); err != nil {
			tables.Close()
			return err
		}
		names = append(names, pgx.Identifier{"public", name}.Sanitize())
	}
	err = tables.Err()
	tables.Close()
	if err != nil {
		return err
	}
	if len(names) > 0 {
		if _, err = tx.Exec(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
			return err
		}
	}
	type source struct{ table, column, prefix, namespace string }
	sources := []source{
		{"machine_credentials", "host_id", "", "xingdu-ssh-v1"},
		{"machine_jobs", "id", "job", "xingdu-ssh-v1"},
		{"protocol_deployments", "id", "node", "xingdu-protocol-v1"},
	}
	var v *vault.Vault
	for _, src := range sources {
		rows, err := tx.Query(ctx, "SELECT "+src.column+"::text,organization_id::text,host_id::text,encrypted FROM "+src.table+" WHERE encrypted IS NOT NULL")
		if err != nil {
			return err
		}
		type record struct {
			object, org, host string
			cipher            []byte
		}
		var records []record
		for rows.Next() {
			var r record
			if err = rows.Scan(&r.object, &r.org, &r.host, &r.cipher); err != nil {
				rows.Close()
				return err
			}
			records = append(records, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, r := range records {
			if v == nil {
				v, err = vault.New(key)
				if err != nil {
					return errors.New("resource ID migration requires the existing XINGDU_CREDENTIAL_KEY")
				}
			}
			oldObject, newObject := r.object, id.FromUUID(src.prefix, r.object)
			if src.prefix == "" {
				oldObject, newObject = "saved", "saved"
			}
			newOrg, newHost := id.FromUUID("org", r.org), id.FromUUID("srv", r.host)
			if newOrg == "" || newHost == "" || newObject == "" {
				return errors.New("resource ID migration encountered an invalid source identifier")
			}
			plain, err := v.Open(r.cipher, strings.Join([]string{src.namespace, r.org, r.host, oldObject}, ":"))
			if err != nil {
				return errors.New("resource ID migration could not authenticate encrypted credentials")
			}
			ciphertext := v.Seal(plain, strings.Join([]string{src.namespace, newOrg, newHost, newObject}, ":"))
			clear(plain)
			if _, err = tx.Exec(ctx, "UPDATE "+src.table+" SET encrypted=$1 WHERE "+src.column+"=$2", ciphertext, r.object); err != nil {
				return err
			}
		}
	}
	return nil
}
