package storage

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/vault"
)

// ACMEAccountKey is operator-owned platform state, separate from tenant data.
// A transaction lock ensures concurrent API instances persist one CA account.
// The optional legacy directory is read only when no database account exists.
func (s *Store) ACMEAccountKey(ctx context.Context, v *vault.Vault, directory, legacyDir string) (*ecdsa.PrivateKey, error) {
	if v == nil || (directory != certificates.Staging && directory != certificates.Production) {
		return nil, ErrInvalid
	}
	if _, ok := ctx.Value(apiKeyContext{}).(APIKey); ok {
		return nil, ErrForbidden
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "acme-account:"+directory); err != nil {
		return nil, err
	}
	aad := "acme-account:" + directory
	var encrypted []byte
	err = tx.QueryRow(ctx, "SELECT encrypted FROM acme_accounts WHERE directory=$1", directory).Scan(&encrypted)
	if err == nil {
		plain, err := v.Open(encrypted, aad)
		if err != nil {
			return nil, errors.New("ACME account decryption failed")
		}
		defer clear(plain)
		key, err := x509.ParseECPrivateKey(plain)
		if err != nil {
			return nil, errors.New("invalid ACME account state")
		}
		return key, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var key *ecdsa.PrivateKey
	if legacyDir != "" {
		// Missing migration input is an error: never silently replace an old account.
		if _, err = os.Lstat(filepath.Join(legacyDir, "account.pem")); err != nil {
			return nil, errors.New("legacy ACME account unavailable")
		}
		key, err = certificates.AccountKey(legacyDir)
	} else {
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	if err != nil {
		return nil, errors.New("ACME account initialization failed")
	}
	plain, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	if _, err = tx.Exec(ctx, "INSERT INTO acme_accounts(directory,encrypted) VALUES($1,$2)", directory, v.Seal(plain, aad)); err != nil {
		return nil, err
	}
	return key, tx.Commit(ctx)
}
