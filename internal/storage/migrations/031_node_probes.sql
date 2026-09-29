ALTER TABLE protocol_deployments ADD COLUMN probe_ok boolean;
ALTER TABLE protocol_deployments ADD COLUMN probe_at timestamptz;
ALTER TABLE protocol_deployments ADD COLUMN probe_latency_ms integer;
ALTER TABLE protocol_deployments ADD COLUMN probe_exit_ip text;
ALTER TABLE api_keys DROP CONSTRAINT api_keys_scopes_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_scopes_check CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials','nodes:probe']::text[]);
