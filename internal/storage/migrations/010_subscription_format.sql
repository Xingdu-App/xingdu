-- Preserve the format of existing subscriptions and explicit Mihomo URLs.
ALTER TABLE subscriptions ADD COLUMN format text NOT NULL DEFAULT 'mihomo' CHECK(format IN ('stash','mihomo'));
ALTER TABLE subscriptions ALTER COLUMN format SET DEFAULT 'stash';
