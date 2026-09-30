package storage

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type APIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	CreatedBy      string     `json:"created_by"`
	Name           string     `json:"name"`
	Prefix         string     `json:"prefix"`
	Scopes         []string   `json:"scopes"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
	RevokedBy      *string    `json:"revoked_by"`
}

const keyColumns = "id,organization_id,created_by,name,prefix,scopes,created_at,expires_at,last_used_at,revoked_at,revoked_by"

func scanKey(row pgx.Row) (k APIKey, err error) {
	err = row.Scan(&k.ID, &k.OrganizationID, &k.CreatedBy, &k.Name, &k.Prefix, &k.Scopes, &k.CreatedAt, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.RevokedBy)
	return
}

type apiKeyContext struct{}

func WithAPIKeyTenant(ctx context.Context, k APIKey) context.Context {
	return context.WithValue(WithTenant(ctx, k.CreatedBy, k.OrganizationID), apiKeyContext{}, k.ID)
}
func ValidAPIKeyScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 9 {
		return false
	}
	seen := map[string]bool{}
	for _, v := range scopes {
		switch v {
		case "hosts:read", "hosts:write", "nodes:read", "nodes:write", "nodes:credentials", "nodes:probe", "subscriptions:read", "subscriptions:write", "subscriptions:export":
		default:
			return false
		}
		if seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
func (s *Store) APIKeys(ctx context.Context) ([]APIKey, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT "+keyColumns+" FROM api_keys WHERE organization_id=request_org_id() ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		k, e := scanKey(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Store) CreateAPIKey(ctx context.Context, name, hash, prefix string, scopes []string, expires time.Time) (APIKey, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 || !ValidAPIKeyScopes(scopes) || !expires.After(time.Now()) || expires.After(time.Now().Add(366*24*time.Hour)) {
		return APIKey{}, ErrInvalid
	}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return APIKey{}, err
	}
	defer tx.Rollback(ctx)
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM api_keys WHERE organization_id=request_org_id() AND revoked_at IS NULL AND expires_at>now()").Scan(&count); err != nil {
		return APIKey{}, err
	}
	if count >= 50 {
		return APIKey{}, ErrInvalid
	}
	k, err := scanKey(tx.QueryRow(ctx, "INSERT INTO api_keys(id,organization_id,created_by,name,token_hash,prefix,scopes,expires_at) VALUES($1,request_org_id(),request_user_id(),$2,$3,$4,$5,$6) RETURNING "+keyColumns, NewID("key"), name, hash, prefix, scopes, expires))
	if err != nil {
		return k, err
	}
	return k, tx.Commit(ctx)
}
func (s *Store) RevokeAPIKey(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, "UPDATE api_keys SET revoked_at=coalesce(revoked_at,now()),revoked_by=coalesce(revoked_by,request_user_id()) WHERE id=$1 AND organization_id=request_org_id()", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
func (s *Store) AuthenticateAPIKey(ctx context.Context, hash string) (APIKey, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return APIKey{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT set_config('app.api_key_hash',$1,true)", hash); err != nil {
		return APIKey{}, err
	}
	k, err := scanKey(tx.QueryRow(ctx, "SELECT "+keyColumns+" FROM api_keys WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()", hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return k, ErrForbidden
	}
	if err != nil {
		return k, err
	}
	// End the lookup transaction before acquiring a tenant transaction.
	if err = tx.Commit(ctx); err != nil {
		return k, err
	}
	scoped := WithAPIKeyTenant(ctx, k)
	tenant, _, err := s.tenantTx(scoped, false, true)
	if err != nil {
		return k, err
	}
	defer tenant.Rollback(ctx)
	_, err = tenant.Exec(scoped, "UPDATE api_keys SET last_used_at=now() WHERE id=$1", k.ID)
	if err != nil {
		return k, err
	}
	return k, tenant.Commit(ctx)
}
