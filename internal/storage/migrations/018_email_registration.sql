ALTER TABLE users ADD COLUMN email text;
ALTER TABLE users ADD COLUMN email_verified_at timestamptz;
ALTER TABLE users ADD CONSTRAINT verified_email_pair CHECK((email IS NULL)=(email_verified_at IS NULL));
CREATE UNIQUE INDEX users_email_unique ON users(lower(email)) WHERE email IS NOT NULL;
-- Authentication bootstrap records are account-level, not tenant data. Like
-- users/sessions, they are accessed only by authentication storage methods.
CREATE TABLE pending_registrations (
 token_hash text PRIMARY KEY CHECK(length(token_hash)=64),
 email text NOT NULL, password_hash text NOT NULL, organization_name text NOT NULL,
 code_hash text NOT NULL CHECK(length(code_hash)=64),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 delivered boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL DEFAULT now()+interval '10 minutes',
 consumed_at timestamptz
);
CREATE INDEX pending_registrations_email_created ON pending_registrations(email,created_at);
CREATE INDEX pending_registrations_expiry ON pending_registrations(expires_at);
GRANT SELECT,INSERT,UPDATE,DELETE ON pending_registrations TO xingdu_app;
