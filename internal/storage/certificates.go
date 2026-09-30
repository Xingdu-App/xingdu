package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/certificates"
)

var ErrCertificatePaid = errors.New("certificate paid subscription required")
var ErrCertificateQuota = errors.New("certificate quota exceeded")

type ManagedCertificate struct {
	Platform         bool       `json:"platform"`
	HostID           string     `json:"host_id,omitempty"`
	Directory        string     `json:"directory"`
	ID               string     `json:"id"`
	Domain           string     `json:"domain"`
	ValidationTarget string     `json:"validation_target"`
	State            string     `json:"state"`
	ExpiresAt        *time.Time `json:"expires_at"`
	ErrorCode        string     `json:"error_code"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Encrypted        []byte     `json:"-"`
	Lease            string     `json:"-"`
}

const certificateColumns = `id,domain,validation_target,state,expires_at,error_code,updated_at,directory,platform,COALESCE(host_id,'')`

func scanCertificate(row scanner) (c ManagedCertificate, err error) {
	err = row.Scan(&c.ID, &c.Domain, &c.ValidationTarget, &c.State, &c.ExpiresAt, &c.ErrorCode, &c.UpdatedAt, &c.Directory, &c.Platform, &c.HostID)
	return c, mapError(err)
}
func certificateLimit(ctx context.Context, tx pgx.Tx) (int, error) {
	var b billing.Record
	err := tx.QueryRow(ctx, `SELECT status,plan,period_end FROM organization_billing WHERE organization_id=request_org_id()`).Scan(&b.Status, &b.Plan, &b.PeriodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return b.CertificateLimit(time.Now()), nil
}

// On downgrade, the oldest resources retain entitlement until excess resources
// are removed. Paused and pending resources still count toward the quota.
func certificateEntitlement(ctx context.Context, tx pgx.Tx, id string) error {
	limit, err := certificateLimit(ctx, tx)
	if err != nil {
		return err
	}
	if limit == 0 {
		return ErrCertificatePaid
	}
	var permitted bool
	err = tx.QueryRow(ctx, `SELECT id IN (SELECT id FROM managed_certificates WHERE state<>'deleted' ORDER BY created_at,id LIMIT $2) FROM managed_certificates WHERE id=$1 AND state<>'deleted'`, id, limit).Scan(&permitted)
	if err != nil {
		return mapError(err)
	}
	if !permitted {
		return ErrCertificateQuota
	}
	return nil
}
func (s *Store) Certificates(ctx context.Context) ([]ManagedCertificate, int, error) {
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	limit, err := certificateLimit(ctx, tx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, `SELECT `+certificateColumns+` FROM managed_certificates WHERE state<>'deleted' ORDER BY created_at DESC`)
	if err != nil {
		return nil, 0, err
	}
	out := []ManagedCertificate{}
	for rows.Next() {
		c, e := scanCertificate(rows)
		if e != nil {
			rows.Close()
			return nil, 0, e
		}
		out = append(out, c)
	}
	err = rows.Err()
	rows.Close()
	return out, limit, err
}
func (s *Store) CreateCertificate(ctx context.Context, domain, validationDomain string) (ManagedCertificate, error) {
	return s.createCertificate(ctx, domain, validationDomain, "", false)
}
func (s *Store) CreatePlatformCertificate(ctx context.Context, host, platformDomain, validationDomain string) (ManagedCertificate, error) {
	if !certificates.ValidDomain(platformDomain) {
		return ManagedCertificate{}, ErrInvalid
	}
	return s.createCertificate(ctx, NewID("cert")[5:]+"."+platformDomain, validationDomain, host, true)
}
func (s *Store) createCertificate(ctx context.Context, domain, validationDomain, host string, platform bool) (ManagedCertificate, error) {
	if !certificates.ValidDomain(domain) || !certificates.ValidDomain(validationDomain) {
		return ManagedCertificate{}, ErrInvalid
	}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return ManagedCertificate{}, err
	}
	defer tx.Rollback(ctx)
	limit, err := certificateLimit(ctx, tx)
	if err != nil {
		return ManagedCertificate{}, err
	}
	if limit == 0 {
		return ManagedCertificate{}, ErrCertificatePaid
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM managed_certificates WHERE state<>'deleted'`).Scan(&count); err != nil {
		return ManagedCertificate{}, err
	}
	if count >= limit {
		return ManagedCertificate{}, ErrCertificateQuota
	}
	if platform {
		var address string
		if err = tx.QueryRow(ctx, `SELECT address FROM hosts WHERE id=$1`, host).Scan(&address); err != nil {
			return ManagedCertificate{}, mapError(err)
		}
		if !certificates.PublicAddress(address) {
			return ManagedCertificate{}, ErrInvalid
		}
	}
	id := NewID("cert")
	if !platform {
		var retired string
		e := tx.QueryRow(ctx, `SELECT id FROM managed_certificates WHERE domain=$1 AND state='deleted' AND NOT platform`, domain).Scan(&retired)
		if e == nil {
			id = retired
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return ManagedCertificate{}, e
		}
	}
	target := (certificates.ManagedConfig{ValidationDomain: validationDomain}).Target(id)
	c, err := scanCertificate(tx.QueryRow(ctx, `INSERT INTO managed_certificates(id,organization_id,created_by,domain,validation_target,host_id,platform) VALUES($1,request_org_id(),request_user_id(),$2,$3,NULLIF($4,''),$5) ON CONFLICT(id) DO UPDATE SET created_by=request_user_id(),validation_target=EXCLUDED.validation_target,state='pending',encrypted=NULL,expires_at=NULL,directory='',error_code='',attempts=0,lease=NULL,lease_until=NULL,next_attempt_at=now(),updated_at=now() WHERE managed_certificates.organization_id=request_org_id() AND managed_certificates.state='deleted' RETURNING `+certificateColumns, id, domain, target, host, platform))
	if err != nil {
		return c, err
	}
	if platform {
		if _, err = tx.Exec(ctx, `UPDATE managed_certificates SET state='queued' WHERE id=$1`, id); err != nil {
			return c, err
		}
		c.State = "queued"
	}
	if err = certificateAudit(ctx, tx, id, "certificate_created"); err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}
func (s *Store) QueueCertificate(ctx context.Context, id, directory string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = certificateEntitlement(ctx, tx, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE managed_certificates SET state='queued',created_by=request_user_id(),error_code='',attempts=0,next_attempt_at=now(),updated_at=now() WHERE id=$1 AND state IN ('pending','failed','paused','issued') AND (state='pending' OR updated_at<now()-interval '1 minute') AND (expires_at IS NULL OR expires_at<now()+interval '30 days' OR directory<>$2)`, id, directory)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if err = certificateAudit(ctx, tx, id, "certificate_issuance_queued"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Deletion leaves the global domain claim reserved, preventing silent reassignment.
func (s *Store) DeleteCertificate(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE managed_certificates SET state='deleted',encrypted=NULL,lease=NULL,lease_until=NULL,updated_at=now() WHERE id=$1 AND state<>'deleted'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = certificateAudit(ctx, tx, id, "certificate_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) CertificateSecret(ctx context.Context, id string) (ManagedCertificate, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return ManagedCertificate{}, err
	}
	defer tx.Rollback(ctx)
	if err = certificateEntitlement(ctx, tx, id); err != nil {
		return ManagedCertificate{}, err
	}
	c, err := scanCertificate(tx.QueryRow(ctx, `SELECT `+certificateColumns+` FROM managed_certificates WHERE id=$1 AND state<>'deleted'`, id))
	if err != nil {
		return c, err
	}
	err = tx.QueryRow(ctx, `SELECT encrypted FROM managed_certificates WHERE id=$1 AND expires_at>now()`, id).Scan(&c.Encrypted)
	return c, mapError(err)
}
func (s *Store) claimCertificate(ctx context.Context, id string) (ManagedCertificate, error) {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return ManagedCertificate{}, err
	}
	defer tx.Rollback(ctx)
	entitlement := certificateEntitlement(ctx, tx, id)
	if errors.Is(entitlement, ErrCertificatePaid) || errors.Is(entitlement, ErrCertificateQuota) {
		code := "paid_subscription_required"
		if errors.Is(entitlement, ErrCertificateQuota) {
			code = "certificate_quota_exceeded"
		}
		_, err = tx.Exec(ctx, `UPDATE managed_certificates SET state='paused',error_code=$2,next_attempt_at=now()+interval '1 hour',updated_at=now(),lease=NULL,lease_until=NULL WHERE id=$1 AND state<>'deleted'`, id, code)
		if err != nil {
			return ManagedCertificate{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return ManagedCertificate{}, err
		}
		return ManagedCertificate{}, entitlement
	}
	if entitlement != nil {
		return ManagedCertificate{}, entitlement
	}
	c, err := scanCertificate(tx.QueryRow(ctx, `UPDATE managed_certificates SET state=CASE WHEN attempts>=5 THEN 'failed' ELSE 'issuing' END,error_code=CASE WHEN attempts>=5 THEN 'issuance_interrupted' ELSE error_code END,lease=CASE WHEN attempts>=5 THEN NULL ELSE $2 END,lease_until=CASE WHEN attempts>=5 THEN NULL ELSE now()+interval '12 minutes' END,attempts=LEAST(attempts+1,5),updated_at=now() WHERE id=$1 AND next_attempt_at<=now() AND (state='queued' OR (state='issuing' AND lease_until<now()) OR (state IN ('issued','paused') AND expires_at<now()+interval '30 days')) RETURNING `+certificateColumns, id, NewID("lease")))
	if err != nil {
		return c, err
	}
	if c.State == "failed" {
		if err = tx.Commit(ctx); err != nil {
			return c, err
		}
		return c, ErrConflict
	}
	err = tx.QueryRow(ctx, `SELECT lease FROM managed_certificates WHERE id=$1`, id).Scan(&c.Lease)
	if err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}
func (s *Store) finishCertificate(ctx context.Context, c ManagedCertificate, cipher []byte, expiry *time.Time, code string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	entitlement := certificateEntitlement(ctx, tx, c.ID)
	if entitlement != nil && !errors.Is(entitlement, ErrCertificatePaid) && !errors.Is(entitlement, ErrCertificateQuota) {
		return entitlement
	}
	state := "issued"
	if code != "" {
		state = "failed"
	}
	if entitlement != nil {
		state = "paused"
		code = "paid_subscription_required"
		if errors.Is(entitlement, ErrCertificateQuota) {
			code = "certificate_quota_exceeded"
		}
		cipher = nil
		expiry = nil
	}
	// Retries back off and stop after five attempts; renewal failure preserves the
	// still-valid previous bundle so callers can apply it while investigating.
	tag, err := tx.Exec(ctx, `UPDATE managed_certificates SET state=CASE WHEN $3='failed' AND attempts<5 AND $6<>'dns_delegation_required' THEN 'queued' ELSE $3 END,encrypted=COALESCE($4,encrypted),expires_at=COALESCE($5,expires_at),error_code=$6,directory=CASE WHEN $3='issued' THEN $7 ELSE directory END,lease=NULL,lease_until=NULL,next_attempt_at=now()+CASE WHEN $3='issued' THEN interval '1 day' WHEN $3='paused' THEN interval '1 hour' ELSE interval '1 hour'*LEAST(attempts,5) END,attempts=CASE WHEN $3='issued' THEN 0 ELSE attempts END,updated_at=now() WHERE id=$1 AND lease=$2 AND state='issuing' AND lease_until>now()`, c.ID, c.Lease, state, cipher, expiry, code, c.Directory)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if err = certificateAudit(ctx, tx, c.ID, "certificate_"+state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func certificateAudit(ctx context.Context, tx pgx.Tx, id, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO certificate_audit(organization_id,certificate_id,actor_id,event) VALUES(request_org_id(),$1,request_user_id(),$2)`, id, event)
	return err
}

func (s *Store) certificatePlatformAddress(ctx context.Context, c ManagedCertificate) (string, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var address string
	err = tx.QueryRow(ctx, `SELECT h.address FROM hosts h JOIN managed_certificates c ON c.host_id=h.id AND c.organization_id=h.organization_id WHERE c.id=$1 AND c.state='issuing' AND c.lease=$2`, c.ID, c.Lease).Scan(&address)
	return address, mapError(err)
}
