ALTER TABLE protocol_deployments ADD COLUMN minimum_agent_version text NOT NULL DEFAULT '0.7.0-dev';
ALTER TABLE subscriptions DROP CONSTRAINT subscriptions_format_check;
ALTER TABLE subscriptions ADD CONSTRAINT subscriptions_format_check
 CHECK (format IN ('stash','mihomo','surge','loon','hysteria2_uri','singbox','uri','base64'));
