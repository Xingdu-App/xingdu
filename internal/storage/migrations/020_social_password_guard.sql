DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='users'::regclass AND conname='social_password_disabled') THEN
 ALTER TABLE users ADD CONSTRAINT social_password_disabled CHECK(password_login_enabled OR password_hash='');
END IF; END $$;

-- Disabled password methods cannot be silently re-enabled through the old
-- current-password change function, even if a future caller supplies a hash.
CREATE OR REPLACE FUNCTION change_account_password(account_id uuid, previous_hash text, replacement_hash text, session_hash text)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=public,pg_temp AS $$
BEGIN
 IF account_id IS DISTINCT FROM request_user_id() THEN RETURN false; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('account:' || account_id::text, 0));
 IF NOT EXISTS(SELECT 1 FROM sessions WHERE user_id=account_id AND token_hash=session_hash AND expires_at>now()) THEN RETURN false; END IF;
 UPDATE users SET password_hash=replacement_hash WHERE id=account_id AND password_hash=previous_hash AND password_login_enabled;
 IF NOT FOUND THEN RETURN false; END IF;
 DELETE FROM sessions WHERE user_id=account_id;
 RETURN true;
END $$;
