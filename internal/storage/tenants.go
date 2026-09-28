package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrForbidden = errors.New("forbidden")
var ErrInvalid = errors.New("invalid input")

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}
type Member struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
type Invitation struct {
	ID         string     `json:"id"`
	Role       string     `json:"role"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}
type scope struct{ User, Org string }
type scopeKey struct{}

func WithTenant(ctx context.Context, user, org string) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope{user, org})
}
func setScope(ctx context.Context, tx pgx.Tx, user, org string) error {
	_, err := tx.Exec(ctx, "SELECT set_config('app.user_id',$1,true),set_config('app.organization_id',$2,true)", user, org)
	return err
}
func (s *Store) tenantTx(ctx context.Context, write, manage bool) (pgx.Tx, string, error) {
	sc, ok := ctx.Value(scopeKey{}).(scope)
	if !ok || sc.User == "" || sc.Org == "" {
		return nil, "", ErrForbidden
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	fail := func(e error) (pgx.Tx, string, error) { tx.Rollback(ctx); return nil, "", e }
	if err = setScope(ctx, tx, sc.User, sc.Org); err != nil {
		return fail(err)
	}
	if s.CloudBilling {
		if _, err = tx.Exec(ctx, "SELECT set_config('app.billing_mode','cloud',true)"); err != nil {
			return fail(err)
		}
	}
	// Serialize membership mutations, invitations and inventory access per organization.
	// This makes a committed removal effective before the next tenant operation.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", sc.Org); err != nil {
		return fail(err)
	}
	var role string
	err = tx.QueryRow(ctx, "SELECT role FROM memberships WHERE organization_id=$1 AND user_id=$2", sc.Org, sc.User).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrForbidden)
	}
	if err != nil {
		return fail(err)
	}
	if (write && role == "viewer") || (manage && role != "owner" && role != "admin") {
		return fail(ErrForbidden)
	}
	return tx, role, nil
}
func validOrg(name string) bool {
	return utf8.RuneCountInString(name) > 0 && utf8.RuneCountInString(name) <= 64
}
func (s *Store) Register(ctx context.Context, username, hash, name string) (string, error) {
	name = strings.TrimSpace(name)
	if !validOrg(name) {
		return "", ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	id, org := NewID(), NewID()
	if err = setScope(ctx, tx, id, org); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO users(id,username,password_hash) VALUES($1,$2,$3)", id, username, hash); err != nil {
		return "", mapError(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO organizations(id,name,created_by) VALUES($1,$2,$3)", org, name, id); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')", org, id); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}
func (s *Store) Organizations(ctx context.Context, user string) ([]Organization, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, user, ""); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT o.id::text,o.name,m.role FROM organizations o JOIN memberships m ON m.organization_id=o.id WHERE m.user_id=$1 ORDER BY o.created_at,o.id", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		var o Organization
		if err = rows.Scan(&o.ID, &o.Name, &o.Role); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (s *Store) CreateOrganization(ctx context.Context, user, name string) (Organization, error) {
	name = strings.TrimSpace(name)
	o := Organization{ID: NewID(), Name: name, Role: "owner"}
	if !validOrg(name) {
		return o, ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return o, err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, user, o.ID); err != nil {
		return o, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO organizations(id,name,created_by) VALUES($1,$2,$3)", o.ID, name, user); err != nil {
		return o, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')", o.ID, user); err != nil {
		return o, err
	}
	return o, tx.Commit(ctx)
}
func (s *Store) Members(ctx context.Context) ([]Member, error) {
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT u.id::text,u.username,m.role FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.organization_id=request_org_id() ORDER BY m.created_at,u.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err = rows.Scan(&m.ID, &m.Username, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func inviteRole(role string) bool { return role == "admin" || role == "member" || role == "viewer" }
func (s *Store) ChangeMember(ctx context.Context, id, role string, remove bool) error {
	if !remove && !inviteRole(role) {
		return ErrInvalid
	}
	tx, actor, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var old string
	err = tx.QueryRow(ctx, "SELECT role FROM memberships WHERE organization_id=request_org_id() AND user_id=$1", id).Scan(&old)
	if err != nil {
		return mapError(err)
	}
	if old == "owner" || (actor == "admin" && (old == "admin" || role == "admin")) {
		return ErrForbidden
	}
	if remove {
		_, err = tx.Exec(ctx, "DELETE FROM memberships WHERE organization_id=request_org_id() AND user_id=$1", id)
	} else {
		_, err = tx.Exec(ctx, "UPDATE memberships SET role=$2 WHERE organization_id=request_org_id() AND user_id=$1", id, role)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) CreateInvitation(ctx context.Context, role, hash string) (Invitation, error) {
	var i Invitation
	if !inviteRole(role) {
		return i, ErrInvalid
	}
	tx, actor, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return i, err
	}
	defer tx.Rollback(ctx)
	if actor == "admin" && role == "admin" {
		return i, ErrForbidden
	}
	i.ID = NewID()
	i.Role = role
	err = tx.QueryRow(ctx, "INSERT INTO invitations(id,organization_id,token_hash,role,created_by,expires_at) VALUES($1,request_org_id(),$2,$3,request_user_id(),now()+interval '7 days') RETURNING expires_at", i.ID, hash, role).Scan(&i.ExpiresAt)
	if err != nil {
		return i, err
	}
	return i, tx.Commit(ctx)
}
func (s *Store) Invitations(ctx context.Context) ([]Invitation, error) {
	tx, _, err := s.tenantTx(ctx, false, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT id::text,role,expires_at,accepted_at,revoked_at FROM invitations WHERE organization_id=request_org_id() ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invitation{}
	for rows.Next() {
		var i Invitation
		if err = rows.Scan(&i.ID, &i.Role, &i.ExpiresAt, &i.AcceptedAt, &i.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *Store) RevokeInvitation(ctx context.Context, id string) error {
	tx, actor, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, "UPDATE invitations SET revoked_at=now() WHERE id=$1 AND organization_id=request_org_id() AND accepted_at IS NULL AND ($2='owner' OR role<>'admin')", id, actor)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
func (s *Store) AcceptInvitation(ctx context.Context, user, hash string) (string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, user, ""); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('app.invitation_hash',$1,true)", hash); err != nil {
		return "", err
	}
	var org, role, id string
	// Resolve token before locking the organization, then recheck under the same lock as revocation.
	err = tx.QueryRow(ctx, "SELECT organization_id::text FROM invitations WHERE token_hash=$1", hash).Scan(&org)
	if err != nil {
		return "", mapError(err)
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org); err != nil {
		return "", err
	}
	if err = setScope(ctx, tx, user, org); err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, "SELECT id::text,role FROM invitations WHERE token_hash=$1 AND expires_at>now() AND accepted_at IS NULL AND revoked_at IS NULL FOR UPDATE", hash).Scan(&id, &role)
	if err != nil {
		return "", mapError(err)
	}
	// Existing members cannot use an invitation to elevate their role.
	if _, err = tx.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,$3)", org, user, role); err != nil {
		return "", mapError(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE invitations SET accepted_at=now() WHERE id=$1", id); err != nil {
		return "", err
	}
	return org, tx.Commit(ctx)
}
