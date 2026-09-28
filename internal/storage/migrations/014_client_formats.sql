ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_format_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_format_check
 CHECK (format IN ('stash','mihomo','surge','loon','hysteria2_uri'));
