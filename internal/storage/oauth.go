package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

var ErrOAuthState = errors.New("invalid oauth state")
var ErrOAuthEmailExists = errors.New("oauth account email already exists")
var ErrOAuthIdentityInUse = errors.New("oauth identity already linked")
var ErrOAuthRegistrationDisabled = errors.New("oauth registration disabled")
var ErrOAuthLinkSession = errors.New("oauth link session expired")
var ErrLastLoginMethod = errors.New("last login method")

type OAuthState struct {
	Hash, BrowserHash, Provider, Mode, UserID, SessionHash string
	Encrypted                                              []byte
}
type OAuthIdentity struct {
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
type LoginMethods struct {
	HasPassword bool            `json:"has_password"`
	Identities  []OAuthIdentity `json:"identities"`
}

func (s *Store) SaveOAuthState(ctx context.Context, v OAuthState) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at<=now()`); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO oauth_states(state_hash,browser_hash,provider,mode,user_id,session_hash,encrypted) VALUES($1,$2,$3,$4,nullif($5,'')::uuid,nullif($6,''),$7)`, v.Hash, v.BrowserHash, v.Provider, v.Mode, v.UserID, v.SessionHash, v.Encrypted)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) ConsumeOAuthState(ctx context.Context, hash, browserHash, provider string) (OAuthState, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return OAuthState{}, err
	}
	defer tx.Rollback(ctx)
	v := OAuthState{Hash: hash, BrowserHash: browserHash, Provider: provider}
	err = tx.QueryRow(ctx, `SELECT mode,coalesce(user_id::text,''),coalesce(session_hash,''),encrypted FROM oauth_states WHERE state_hash=$1 AND browser_hash=$2 AND provider=$3 AND used_at IS NULL AND expires_at>now() FOR UPDATE`, hash, browserHash, provider).Scan(&v.Mode, &v.UserID, &v.SessionHash, &v.Encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrOAuthState
	}
	if err != nil {
		return v, err
	}
	if _, err = tx.Exec(ctx, `UPDATE oauth_states SET used_at=now(),encrypted=''::bytea WHERE state_hash=$1`, hash); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// CompleteOAuth only accepts identities already verified by a provider adapter.
// Subject and account locks serialize linking, login and last-method removal.
func (s *Store) CompleteOAuth(ctx context.Context, state OAuthState, subject, email, sessionHash string, expires time.Time, allowRegistration bool) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || subject == "" || (state.Provider != "google" && state.Provider != "github") || (state.Mode != "login" && state.Mode != "link") {
		return User{}, ErrInvalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('identity:' || $1 || ':' || $2,0))`, state.Provider, subject); err != nil {
		return User{}, err
	}
	var existing string
	if err = tx.QueryRow(ctx, `SELECT coalesce(oauth_identity_user($1,$2)::text,'')`, state.Provider, subject).Scan(&existing); err != nil {
		return User{}, err
	}
	id := existing
	if state.Mode == "link" {
		id = state.UserID
		if existing != "" && existing != id {
			return User{}, ErrOAuthIdentityInUse
		}
	}
	if id == "" {
		// Serialize first logins across providers sharing the verified email.
		// A username that looks like an email is not proof of email ownership.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('oauth-email:' || $1,0))`, email); err != nil {
			return User{}, err
		}
		err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(email)=$1 AND email_verified_at IS NOT NULL`, email).Scan(&id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return User{}, err
		}
		if id == "" {
			if !allowRegistration {
				return User{}, ErrOAuthRegistrationDisabled
			}
			var collision bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE lower(username)=$1 OR lower(email)=$1)`, email).Scan(&collision); err != nil {
				return User{}, err
			}
			if collision {
				return User{}, ErrOAuthEmailExists
			}
			id = NewID()
			org := NewID()
			if err = setScope(ctx, tx, id, org); err != nil {
				return User{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO users(id,username,password_hash,email,email_verified_at,password_login_enabled) VALUES($1,$2,'',$2,now(),false)`, id, email); err != nil {
				if errors.Is(mapError(err), ErrConflict) {
					return User{}, ErrOAuthEmailExists
				}
				return User{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO organizations(id,name,created_by) VALUES($1,'我的组织',$2)`, org, id); err != nil {
				return User{}, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'owner')`, org, id); err != nil {
				return User{}, err
			}
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account:' || $1,0))`, id); err != nil {
		return User{}, err
	}
	if err = setScope(ctx, tx, id, ""); err != nil {
		return User{}, err
	}
	if existing != "" {
		var stillLinked bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM oauth_identities WHERE provider=$1 AND subject=$2 AND user_id=$3)`, state.Provider, subject, id).Scan(&stillLinked); err != nil {
			return User{}, err
		}
		if !stillLinked {
			return User{}, ErrOAuthIdentityInUse
		}
	}
	if state.Mode == "link" {
		var live string
		// Row lock gives concurrent logout/revocation a defined before/after order.
		err = tx.QueryRow(ctx, `SELECT token_hash FROM sessions WHERE token_hash=$1 AND user_id=$2 AND expires_at>now() FOR UPDATE`, state.SessionHash, id).Scan(&live)
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrOAuthLinkSession
		}
		if err != nil {
			return User{}, err
		}
	}
	if existing == "" {
		_, err = tx.Exec(ctx, `INSERT INTO oauth_identities(provider,subject,user_id,email) VALUES($1,$2,$3,$4)`, state.Provider, subject, id, email)
		if errors.Is(mapError(err), ErrConflict) {
			return User{}, ErrOAuthIdentityInUse
		}
		if err != nil {
			return User{}, err
		}
	}
	var user User
	if err = tx.QueryRow(ctx, `SELECT id::text,username FROM users WHERE id=$1`, id).Scan(&user.ID, &user.Username); err != nil {
		return User{}, mapError(err)
	}
	if state.Mode == "login" {
		if _, err = tx.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, sessionHash, id, expires); err != nil {
			return User{}, err
		}
	}
	return user, tx.Commit(ctx)
}
func (s *Store) LoginMethods(ctx context.Context, id string) (LoginMethods, error) {
	out := LoginMethods{Identities: []OAuthIdentity{}}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, id, ""); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT password_login_enabled FROM users WHERE id=$1`, id).Scan(&out.HasPassword); err != nil {
		return out, mapError(err)
	}
	rows, err := tx.Query(ctx, `SELECT provider,email,created_at FROM oauth_identities WHERE user_id=$1 ORDER BY provider`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v OAuthIdentity
		if err = rows.Scan(&v.Provider, &v.Email, &v.CreatedAt); err != nil {
			return out, err
		}
		out.Identities = append(out.Identities, v)
	}
	return out, rows.Err()
}
func (s *Store) UnlinkOAuthIdentity(ctx context.Context, userID, provider, currentSession string, enabledProviders []string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, userID, ""); err != nil {
		return err
	}
	var subject string
	if err = tx.QueryRow(ctx, `SELECT subject FROM oauth_identities WHERE user_id=$1 AND provider=$2`, userID, provider).Scan(&subject); err != nil {
		return mapError(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('identity:' || $1 || ':' || $2,0))`, provider, subject); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account:' || $1,0))`, userID); err != nil {
		return err
	}
	var live string
	if err = tx.QueryRow(ctx, `SELECT token_hash FROM sessions WHERE token_hash=$1 AND user_id=$2 AND expires_at>now() FOR UPDATE`, currentSession, userID).Scan(&live); err != nil {
		return mapError(err)
	}
	var stillLinked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM oauth_identities WHERE user_id=$1 AND provider=$2 AND subject=$3)`, userID, provider, subject).Scan(&stillLinked); err != nil {
		return err
	}
	if !stillLinked {
		return ErrConflict
	}
	var password bool
	var identities int
	if err = tx.QueryRow(ctx, `SELECT password_login_enabled,(SELECT count(*) FROM oauth_identities WHERE user_id=$1 AND provider<>$2 AND provider=ANY($3::text[])) FROM users WHERE id=$1`, userID, provider, enabledProviders).Scan(&password, &identities); err != nil {
		return err
	}
	if !password && identities == 0 {
		return ErrLastLoginMethod
	}
	result, err := tx.Exec(ctx, `DELETE FROM oauth_identities WHERE user_id=$1 AND provider=$2 AND subject=$3`, userID, provider, subject)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
