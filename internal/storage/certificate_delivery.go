package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/vault"
)

type CertificateIssuer interface {
	Issue(context.Context, string, string) (certificates.Bundle, error)
}

func CertificateAAD(org, id string) string { return "certificate:" + org + ":" + id }
func (s *Store) RunCertificateDelivery(ctx context.Context, issuer CertificateIssuer, v *vault.Vault) {
	if issuer == nil || v == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deliverCertificates(ctx, issuer, v)
		}
	}
}
func (s *Store) deliverCertificates(ctx context.Context, issuer CertificateIssuer, v *vault.Vault) {
	rows, err := s.Pool.Query(ctx, `SELECT id,organization_id,created_by FROM pending_certificate_jobs()`)
	if err != nil {
		return
	}
	type job struct{ id, org, user string }
	jobs := []job{}
	for rows.Next() {
		var j job
		if rows.Scan(&j.id, &j.org, &j.user) != nil {
			rows.Close()
			return
		}
		jobs = append(jobs, j)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	// Sequential CA requests bound load. Each job reacquires its own tenant scope.
	for _, j := range jobs {
		if ctx.Err() != nil {
			return
		}
		s.deliverCertificate(WithTenant(ctx, j.user, j.org), j.id, j.org, issuer, v)
	}
}
func (s *Store) deliverCertificate(ctx context.Context, id, org string, issuer CertificateIssuer, v *vault.Vault) {
	c, err := s.claimCertificate(ctx, id)
	if err != nil {
		return
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Minute)
	var bundle certificates.Bundle
	if c.Platform {
		var address string
		address, err = s.certificatePlatformAddress(work, c)
		if err == nil {
			p, ok := issuer.(interface {
				IssuePlatform(context.Context, string, string) (certificates.Bundle, error)
			})
			if ok {
				bundle, err = p.IssuePlatform(work, c.Domain, address)
			} else {
				err = errors.New("provider_unavailable")
			}
		}
	} else {
		bundle, err = issuer.Issue(work, c.Domain, c.ValidationTarget)
	}
	cancel()
	var cipher []byte
	var expiry *time.Time
	code := ""
	if err != nil {
		code = "issuance_failed"
		if err.Error() == "dns_delegation_required" {
			code = "dns_delegation_required"
		}
	} else {
		c.Directory = bundle.Directory
		expiry = protocol.CertificateExpiry(bundle.Certificate)
		if expiry == nil || !expiry.After(time.Now()) || protocol.ValidateInput(protocol.Input{Protocol: "trojan", Name: "certificate", Port: 443, ServerName: c.Domain, Certificate: bundle.Certificate, PrivateKey: bundle.PrivateKey}) != nil {
			code = "invalid_certificate"
		} else {
			plain, _ := json.Marshal(bundle)
			cipher = v.Seal(plain, CertificateAAD(org, id))
			clear(plain)
		}
	}
	// A separate bounded context releases work after provider timeouts; shutdown
	// or a crash leaves a recoverable persisted lease rather than losing a job.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.finishCertificate(finish, c, cipher, expiry, code)
}
