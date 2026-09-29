-- Keep the authentication hash and an independently encrypted recoverable token.
-- Existing hashes cannot recover old links; only explicit rotation creates one.
ALTER TABLE subscriptions ADD COLUMN encrypted_token bytea;
