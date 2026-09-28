ALTER TABLE users ADD COLUMN password_login_enabled boolean NOT NULL DEFAULT true;
CREATE TABLE oauth_states (
 state_hash text PRIMARY KEY CHECK(length(state_hash)=64), browser_hash text NOT NULL CHECK(length(browser_hash)=64),
 provider text NOT NULL CHECK(provider IN ('google','github')), mode text NOT NULL CHECK(mode IN ('login','link')),
 user_id uuid REFERENCES users(id) ON DELETE CASCADE, session_hash text,
 encrypted bytea NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL DEFAULT now()+interval '10 minutes', used_at timestamptz,
 CHECK((mode='login' AND user_id IS NULL AND session_hash IS NULL) OR (mode='link' AND user_id IS NOT NULL AND session_hash IS NOT NULL))
);
CREATE INDEX oauth_states_expiry ON oauth_states(expires_at);
GRANT SELECT,INSERT,UPDATE,DELETE ON oauth_states TO xingdu_app;
CREATE TABLE oauth_identities (
 provider text NOT NULL CHECK(provider IN ('google','github')), subject text NOT NULL CHECK(length(subject) BETWEEN 1 AND 255),
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,email text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(provider,subject),UNIQUE(user_id,provider)
);
ALTER TABLE oauth_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_identities FORCE ROW LEVEL SECURITY;
CREATE POLICY own_oauth_identity ON oauth_identities FOR ALL TO xingdu_app USING(user_id=request_user_id()) WITH CHECK(user_id=request_user_id());
GRANT SELECT,INSERT,DELETE ON oauth_identities TO xingdu_app;
-- Resolving a verified provider subject precedes user-scoped access. The app
-- cannot assume this non-login read-only role or bypass tenant/account RLS.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_identity') THEN
 CREATE ROLE xingdu_identity NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
END IF; END $$;
GRANT USAGE ON SCHEMA public TO xingdu_identity;
GRANT SELECT ON oauth_identities TO xingdu_identity;
CREATE POLICY identity_lookup_read ON oauth_identities FOR SELECT TO xingdu_identity USING(true);
CREATE FUNCTION oauth_identity_user(provider_name text,provider_subject text) RETURNS uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT user_id FROM public.oauth_identities WHERE provider=provider_name AND subject=provider_subject
$$;
ALTER FUNCTION oauth_identity_user(text,text) OWNER TO xingdu_identity;
REVOKE ALL ON FUNCTION oauth_identity_user(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION oauth_identity_user(text,text) TO xingdu_app;
