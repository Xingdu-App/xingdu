package storage

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/vault"
)

func TestCertificatePaidQuotaIsolationAndLeases(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	if err = admin.ConfigureRuntime(ctx, "certificate-fixture-password"); err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword("xingdu_app", "certificate-fixture-password")
	s, err := OpenRuntime(ctx, u.String(), "cloud")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CloudBilling = true
	user, err := s.Register(ctx, "cert_"+NewID("obj")[4:12], "unused", "Certificate fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(ctx, `DELETE FROM organizations WHERE created_by=$1`, user)
		admin.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user)
	}()
	orgs, _ := s.Organizations(ctx, user)
	org := orgs[0].ID
	tenant := WithTenant(ctx, user, org)
	if _, err = s.CreateCertificate(tenant, "node.example.com", "validation.example.com"); !errors.Is(err, ErrCertificatePaid) {
		t.Fatal("free user accepted", err)
	}
	if err = s.EnsureBilling(tenant); err != nil {
		t.Fatal(err)
	}
	if err = s.MutateBilling(tenant, "", "", func(r *billing.Record) error {
		r.Status = "active"
		r.Plan = "start"
		r.PeriodEnd = time.Now().Add(time.Hour).Unix()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	root := NewID("obj")[4:12]
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := s.CreateCertificate(tenant, fmt.Sprintf("node%d.%s.example.com", i, root), "validation.example.com")
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	success, quota := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, ErrCertificateQuota) {
			quota++
		} else {
			t.Fatal(e)
		}
	}
	if success != 10 || quota != 2 {
		t.Fatalf("quota race: %d successes %d blocked", success, quota)
	}
	list, limit, err := s.Certificates(tenant)
	if err != nil || limit != 10 || len(list) != 10 {
		t.Fatal("inventory", limit, len(list), err)
	}
	c := list[0]
	other, err := s.Register(ctx, "cert_other_"+NewID("obj")[4:12], "unused", "Other certificate fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(ctx, `DELETE FROM organizations WHERE created_by=$1`, other)
		admin.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, other)
	}()
	otherOrgs, _ := s.Organizations(ctx, other)
	foreign := WithTenant(ctx, other, otherOrgs[0].ID)
	if inventory, _, e := s.Certificates(foreign); e != nil || len(inventory) != 0 {
		t.Fatal("cross-tenant inventory leaked", e)
	}
	if e := s.DeleteCertificate(foreign, c.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("cross-tenant delete accepted", e)
	}
	// Platform domains bind to a tenant-owned host and reject private addresses.
	if err = s.DeleteCertificate(tenant, list[1].ID); err != nil {
		t.Fatal(err)
	}
	privateHost, err := s.CreateHost(tenant, hosts.Input{Name: "Private fixture", Address: "192.168.1.2", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreatePlatformCertificate(tenant, privateHost.ID, "nodes.example.com", "validation.example.com"); !errors.Is(err, ErrInvalid) {
		t.Fatal("private allocation accepted", err)
	}
	publicHost, err := s.CreateHost(tenant, hosts.Input{Name: "Public IP fixture", Address: "8.8.8.8", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	plat, err := s.CreatePlatformCertificate(tenant, publicHost.ID, "nodes.example.com", "validation.example.com")
	if err != nil || plat.HostID != publicHost.ID || !plat.Platform || plat.State != "queued" {
		t.Fatal("platform allocation", plat, err)
	}
	if _, _, err = s.Certificates(WithTenant(ctx, user, NewID("org"))); !errors.Is(err, ErrForbidden) {
		t.Fatal("cross tenant accepted", err)
	}
	if err = s.QueueCertificate(tenant, c.ID, certificates.Staging); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.claimCertificate(tenant, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.claimCertificate(tenant, c.ID); err == nil {
		t.Fatal("duplicate lease acquired")
	}
	wrong := claimed
	wrong.Lease = NewID("lease")
	expiry := time.Now().Add(80 * 24 * time.Hour)
	claimed.Directory = certificates.Staging
	if err = s.finishCertificate(tenant, wrong, []byte("encrypted-fixture"), &expiry, ""); !errors.Is(err, ErrConflict) {
		t.Fatal("stale lease accepted", err)
	}
	if err = s.finishCertificate(tenant, claimed, []byte("encrypted-fixture"), &expiry, ""); err != nil {
		t.Fatal(err)
	}
	list, _, err = s.Certificates(tenant)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range list {
		if len(row.Encrypted) > 0 {
			t.Fatal("inventory leaks ciphertext")
		}
	}
	// Force renewal due, then verify discovery through its restricted service role.
	if _, err = admin.Pool.Exec(ctx, `UPDATE managed_certificates SET expires_at=now()+interval '1 day',next_attempt_at=now() WHERE id=$1`, c.ID); err != nil {
		t.Fatal(err)
	}
	var discovered string
	if err = s.Pool.QueryRow(ctx, `SELECT id FROM pending_certificate_jobs() WHERE id=$1`, c.ID).Scan(&discovered); err != nil {
		t.Fatal("scheduler discovery", err)
	}
	v, _ := vault.New(fmt.Sprintf("%064x", 1))
	s.deliverCertificate(tenant, c.ID, org, certificateIssuerFixture{}, v)
	secret, err := s.CertificateSecret(tenant, c.ID)
	if err != nil {
		t.Fatal("delivery did not persist", err)
	}
	if bytes.Contains(secret.Encrypted, []byte("PRIVATE KEY")) {
		t.Fatal("private key stored in plaintext")
	}
	plain, err := v.Open(secret.Encrypted, CertificateAAD(org, c.ID))
	if err != nil || !bytes.Contains(plain, []byte("PRIVATE KEY")) {
		t.Fatal("encrypted bundle missing", err)
	}
	clear(plain)
	if _, err = v.Open(secret.Encrypted, CertificateAAD(NewID("org"), c.ID)); err == nil {
		t.Fatal("ciphertext accepted under foreign tenant AAD")
	}
	if err = s.MutateBilling(tenant, "", "", func(r *billing.Record) error { r.Plan = "premium"; return nil }); err != nil {
		t.Fatal(err)
	}
	extra, err := s.CreateCertificate(tenant, "extra."+root+".example.com", "validation.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.MutateBilling(tenant, "", "", func(r *billing.Record) error { r.Plan = "start"; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = s.QueueCertificate(tenant, extra.ID, certificates.Production); !errors.Is(err, ErrCertificateQuota) {
		t.Fatal("downgrade bypassed quota", err)
	}
	if _, err = admin.Pool.Exec(ctx, `UPDATE managed_certificates SET state='queued' WHERE id=$1`, extra.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.claimCertificate(tenant, extra.ID); !errors.Is(err, ErrCertificateQuota) {
		t.Fatal("excess renewal accepted", err)
	}
	if err = s.DeleteCertificate(tenant, extra.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Pool.Exec(ctx, `UPDATE managed_certificates SET expires_at=now()+interval '1 day',next_attempt_at=now() WHERE id=$1`, c.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.MutateBilling(tenant, "", "", func(r *billing.Record) error { r.PeriodEnd = time.Now().Unix(); return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = s.claimCertificate(tenant, c.ID); !errors.Is(err, ErrCertificatePaid) {
		t.Fatal("expired renewal accepted", err)
	}
	if _, err = s.CertificateSecret(tenant, c.ID); !errors.Is(err, ErrCertificatePaid) {
		t.Fatal("expired application accepted", err)
	}
	if err = s.DeleteCertificate(tenant, c.ID); err != nil {
		t.Fatal(err)
	}
	list, _, err = s.Certificates(tenant)
	if err != nil || len(list) != 9 {
		t.Fatal("delete", err)
	}
	if err = s.MutateBilling(tenant, "", "", func(r *billing.Record) error { r.PeriodEnd = time.Now().Add(time.Hour).Unix(); return nil }); err != nil {
		t.Fatal(err)
	}
	restored, err := s.CreateCertificate(tenant, c.Domain, "validation.example.com")
	if err != nil || restored.ID != c.ID || restored.State != "pending" {
		t.Fatal("same-tenant domain restoration", restored, err)
	}
}

type certificateIssuerFixture struct{}

func (certificateIssuerFixture) Issue(_ context.Context, domain, _ string) (certificates.Bundle, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certificates.Bundle{}, err
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{domain}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(80 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	if err != nil {
		return certificates.Bundle{}, err
	}
	private, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return certificates.Bundle{}, err
	}
	return certificates.Bundle{Domain: domain, Directory: certificates.Production, Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private}))}, nil
}
