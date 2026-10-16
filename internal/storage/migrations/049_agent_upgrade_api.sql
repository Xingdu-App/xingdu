ALTER TABLE api_keys DROP CONSTRAINT api_keys_scopes_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_scopes_check CHECK(cardinality(scopes)>0 AND scopes <@ ARRAY['hosts:read','hosts:write','nodes:read','nodes:write','nodes:credentials','nodes:probe','subscriptions:read','subscriptions:write','subscriptions:export','certificates:read','certificates:write','exits:read','exits:write','agents:upgrade']::text[]);

ALTER TABLE machine_jobs ADD COLUMN origin_api_key_id text REFERENCES api_keys(id);
