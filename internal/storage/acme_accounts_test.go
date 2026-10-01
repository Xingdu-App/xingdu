package storage

import (
	"bytes"
	"context"
	"crypto/x509"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/vault"
)

func TestACMEAccountPersistenceAndMigration(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	owner, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err = owner.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	defer owner.Pool.Exec(ctx, "DELETE FROM acme_accounts")
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("options", "-crole=xingdu_app")
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, _ := vault.New(strings.Repeat("a", 64))
	dir := filepath.Join(t.TempDir(), "legacy")
	legacy, err := certificates.AccountKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := x509.MarshalECPrivateKey(legacy)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, e := s.ACMEAccountKey(ctx, v, certificates.Production, dir)
			if e != nil {
				t.Error(e)
				return
			}
			der, _ := x509.MarshalECPrivateKey(k)
			if !bytes.Equal(der, expected) {
				t.Error("concurrent migration changed account")
			}
		}()
	}
	wg.Wait()
	var encrypted []byte
	if err = owner.Pool.QueryRow(ctx, "SELECT encrypted FROM acme_accounts WHERE directory=$1", certificates.Production).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(encrypted, expected) || bytes.Contains(encrypted, expected) {
		t.Fatal("plaintext persisted")
	}
	if _, err = s.ACMEAccountKey(ctx, v, certificates.Production, ""); err != nil {
		t.Fatal("database-only restart", err)
	}
	if _, err = s.ACMEAccountKey(ctx, v, certificates.Production, "missing-directory"); err != nil {
		t.Fatal("existing account depended on legacy volume", err)
	}
	wrong, _ := vault.New(strings.Repeat("b", 64))
	if _, err = s.ACMEAccountKey(ctx, wrong, certificates.Production, ""); err == nil {
		t.Fatal("wrong encryption key accepted")
	}
	if _, err = s.ACMEAccountKey(ctx, v, certificates.Staging, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing migration file generated new account")
	}
	if _, err = os.Stat(filepath.Join(dir, "account.pem")); err != nil {
		t.Fatal("legacy file removed", err)
	}
	fresh, err := s.ACMEAccountKey(ctx, v, certificates.Staging, "")
	if err != nil {
		t.Fatal(err)
	}
	freshDER, _ := x509.MarshalECPrivateKey(fresh)
	if bytes.Equal(expected, freshDER) {
		t.Fatal("new CA reused generated account")
	}
	if _, err = s.ACMEAccountKey(WithAPIKeyTenant(ctx, APIKey{}), v, certificates.Staging, ""); err != ErrForbidden {
		t.Fatal("API principal accessed platform identity", err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE acme_accounts SET encrypted=encrypted"); err == nil {
		t.Fatal("runtime can overwrite account")
	}
}
