CREATE TABLE user_avatars (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 image bytea NOT NULL CHECK(octet_length(image) BETWEEN 1 AND 1048576),
 updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE user_avatars ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_avatars FORCE ROW LEVEL SECURITY;
CREATE POLICY own_avatar ON user_avatars FOR ALL
 USING(user_id=request_user_id()) WITH CHECK(user_id=request_user_id());
GRANT SELECT,INSERT,UPDATE,DELETE ON user_avatars TO xingdu_app;
