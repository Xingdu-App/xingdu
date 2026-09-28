package storage

import (
	"context"
	"errors"
	"strings"
)

// OpenRuntime refuses credentials that could bypass tenant policies or change the schema.
func OpenRuntime(ctx context.Context, url string) (*Store, error) {
	s, err := Open(ctx, url)
	if err != nil {
		return nil, err
	}
	var unsafe bool
	err = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname IN(current_user,session_user) AND (rolsuper OR rolbypassrls OR rolcreaterole)) OR EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)) OR has_schema_privilege(current_user,'public','CREATE')`).Scan(&unsafe)
	if err != nil || unsafe {
		s.Close()
		return nil, errors.New("runtime database role must be a non-owner without superuser, BYPASSRLS, CREATEROLE or schema CREATE privileges")
	}
	return s, nil
}

// Only the deployment/migration process receives this credential, never the API's owner password.
func (s *Store) ConfigureRuntime(ctx context.Context, password string) error {
	if len(password) < 16 || strings.ContainsRune(password, 0) {
		return errors.New("runtime database password must contain at least 16 bytes")
	}
	// Utility statements cannot bind password parameters. Quote as an SQL literal and never log SQL.
	_, err := s.Pool.Exec(ctx, "ALTER ROLE xingdu_app LOGIN PASSWORD '"+strings.ReplaceAll(password, "'", "''")+"'")
	if err != nil {
		return errors.New("could not configure runtime database role")
	}
	return nil
}
