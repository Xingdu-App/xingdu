package storage

import (
	"bytes"
	"context"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

type ExternalProxy struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	Revision  int       `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
	Encrypted []byte    `json:"-"`
}

const externalColumns = "id,name,address,port,protocol,revision,updated_at"

func scanExternal(row pgx.Row) (p ExternalProxy, err error) {
	err = row.Scan(&p.ID, &p.Name, &p.Address, &p.Port, &p.Protocol, &p.Revision, &p.UpdatedAt)
	return
}
func (s *Store) ExternalProxies(ctx context.Context) ([]ExternalProxy, error) {
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT "+externalColumns+" FROM external_proxies ORDER BY updated_at DESC,id")
	if err != nil {
		return nil, err
	}
	out := []ExternalProxy{}
	for rows.Next() {
		p, e := scanExternal(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) ExternalProxySecret(ctx context.Context, proxy string) (ExternalProxy, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return ExternalProxy{}, err
	}
	defer tx.Rollback(ctx)
	p, err := scanExternal(tx.QueryRow(ctx, "SELECT "+externalColumns+" FROM external_proxies WHERE id=$1", proxy))
	if err != nil {
		return p, mapError(err)
	}
	if err = tx.QueryRow(ctx, "SELECT encrypted FROM external_proxies WHERE id=$1", proxy).Scan(&p.Encrypted); err != nil {
		return p, mapError(err)
	}
	return p, tx.Commit(ctx)
}
func externalAudit(ctx context.Context, tx pgx.Tx, proxy, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO external_proxy_audit(organization_id,proxy_id,actor_id,event) VALUES(request_org_id(),$1,request_user_id(),$2)`, proxy, event)
	return err
}
func checkExternalDependents(ctx context.Context, tx pgx.Tx, proxy string) error {
	var used bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM protocol_deployments d LEFT JOIN protocol_revisions r ON r.node_id=d.id AND r.organization_id=d.organization_id AND r.revision=d.pending_revision WHERE d.state<>'removed' AND (d.external_exit_id=$1 OR r.external_exit_id=$1))`, proxy).Scan(&used)
	if err != nil {
		return err
	}
	if used {
		return ErrConflict
	}
	return nil
}
func checkExternalExit(ctx context.Context, tx pgx.Tx, d Deployment) error {
	if d.RelayExitID != "" {
		return ErrInvalid
	}
	var cipher []byte
	var revision int
	err := tx.QueryRow(ctx, `SELECT encrypted,revision FROM external_proxies WHERE id=$1 FOR UPDATE`, d.ExternalExitID).Scan(&cipher, &revision)
	if err != nil {
		return mapError(err)
	}
	if !bytes.Equal(cipher, d.ExternalCipher) || revision != d.ExternalRevision {
		return ErrConflict
	}
	return nil
}

// Ciphertext is generated in the API and bound to tenant and proxy identity.
// Metadata edits preserve credentials unless the caller supplies new ciphertext.
func (s *Store) SaveExternalProxy(ctx context.Context, p ExternalProxy, previous []byte, create bool) (ExternalProxy, error) {
	p.Name = strings.TrimSpace(p.Name)
	if !id.Valid("ext", p.ID) || p.Name == "" || utf8.RuneCountInString(p.Name) > 64 || !protocol.ValidExternalEndpoint(p.Address, p.Port) || (p.Protocol != "socks" && p.Protocol != "http") {
		return p, ErrInvalid
	}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return p, err
	}
	defer tx.Rollback(ctx)
	if create {
		if len(p.Encrypted) == 0 {
			return p, ErrInvalid
		}
		var count int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM external_proxies").Scan(&count); err != nil {
			return p, err
		}
		if count >= 100 {
			return p, ErrConflict
		}
		p, err = scanExternal(tx.QueryRow(ctx, `INSERT INTO external_proxies(id,organization_id,name,address,port,protocol,encrypted) VALUES($1,request_org_id(),$2,$3,$4,$5,$6) RETURNING `+externalColumns, p.ID, p.Name, p.Address, p.Port, p.Protocol, p.Encrypted))
	} else {
		if err = checkExternalDependents(ctx, tx, p.ID); err != nil {
			return p, err
		}
		var old []byte
		if err = tx.QueryRow(ctx, "SELECT encrypted FROM external_proxies WHERE id=$1 FOR UPDATE", p.ID).Scan(&old); err != nil {
			return p, mapError(err)
		}
		if !bytes.Equal(old, previous) {
			return p, ErrConflict
		}
		if len(p.Encrypted) == 0 {
			p.Encrypted = old
		}
		p, err = scanExternal(tx.QueryRow(ctx, `UPDATE external_proxies SET name=$2,address=$3,port=$4,protocol=$5,encrypted=$6,revision=revision+1,updated_at=now() WHERE id=$1 RETURNING `+externalColumns, p.ID, p.Name, p.Address, p.Port, p.Protocol, p.Encrypted))
	}
	if err != nil {
		return p, mapError(err)
	}
	event := "external_proxy_updated"
	if create {
		event = "external_proxy_created"
	}
	if err = externalAudit(ctx, tx, p.ID, event); err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
func (s *Store) DeleteExternalProxy(ctx context.Context, proxy string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = checkExternalDependents(ctx, tx, proxy); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "DELETE FROM external_proxies WHERE id=$1", proxy)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = externalAudit(ctx, tx, proxy, "external_proxy_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
