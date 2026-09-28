ALTER TABLE sessions ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
CREATE UNIQUE INDEX sessions_public_id_idx ON sessions(id);
CREATE TABLE user_profiles (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 64),
 updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE user_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY own_profile ON user_profiles FOR ALL
 USING(user_id=request_user_id()) WITH CHECK(user_id=request_user_id());
GRANT SELECT,INSERT,UPDATE,DELETE ON user_profiles TO xingdu_app;
-- Only this narrowly scoped function can update a password. Runtime role keeps
-- SELECT/INSERT access to the auth identity table, without broad UPDATE access.
CREATE FUNCTION change_account_password(account_id uuid, previous_hash text, replacement_hash text, session_hash text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF account_id IS DISTINCT FROM request_user_id() THEN RETURN false; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('account:' || account_id::text, 0));
 IF NOT EXISTS(SELECT 1 FROM sessions WHERE user_id=account_id AND token_hash=session_hash AND expires_at>now()) THEN RETURN false; END IF;
 UPDATE users SET password_hash=replacement_hash WHERE id=account_id AND password_hash=previous_hash;
 IF NOT FOUND THEN RETURN false; END IF;
 DELETE FROM sessions WHERE user_id=account_id;
 RETURN true;
END $$;
REVOKE ALL ON FUNCTION change_account_password(uuid,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION change_account_password(uuid,text,text,text) TO xingdu_app;
