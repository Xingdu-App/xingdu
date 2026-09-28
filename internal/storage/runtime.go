package storage

import (
	"context"
	"errors"
	"strings"
)

// OpenRuntime refuses credentials that could bypass tenant policies or change the schema.
func OpenRuntime(ctx context.Context, url string, mode ...string) (*Store, error) {
	s, err := Open(ctx, url, mode...)
	if err != nil {
		return nil, err
	}
	var unsafe bool
	err = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname IN(current_user,session_user) AND (rolsuper OR rolbypassrls OR rolcreaterole)) OR EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)) OR has_schema_privilege(current_user,'public','CREATE')`).Scan(&unsafe)
	if err != nil || unsafe {
		s.Close()
		return nil, errors.New("runtime database role must be a non-owner without superuser, BYPASSRLS, CREATEROLE or schema CREATE privileges")
	}
	if len(mode) > 0 && mode[0] == "self_hosted" {
		var count int
		if err := s.Pool.QueryRow(ctx, "SELECT public.deployment_organization_count()").Scan(&count); err != nil {
			s.Close()
			return nil, err
		}
		if count > 1 {
			s.Close()
			return nil, errors.New("MODE=self_hosted requires at most one organization; existing organizations were not changed")
		}
	}
	return s, nil
}

// Only the deployment/migration process receives this credential, never the API's owner password.
func (s *Store) ConfigureRuntime(ctx context.Context, password string) error {
	return s.configureRole(ctx, "xingdu_app", password)
}
func (s *Store) ConfigureWorker(ctx context.Context, password string) error {
	return s.configureRole(ctx, "xingdu_worker", password)
}
func (s *Store) configureRole(ctx context.Context, role, password string) error {
	if len(password) < 16 || strings.ContainsRune(password, 0) {
		return errors.New("runtime database password must contain at least 16 bytes")
	}
	// Utility statements cannot bind password parameters. Quote as an SQL literal and never log SQL.
	_, err := s.Pool.Exec(ctx, "ALTER ROLE "+role+" LOGIN PASSWORD '"+strings.ReplaceAll(password, "'", "''")+"'")
	if err != nil {
		return errors.New("could not configure runtime database role")
	}
	return nil
}
