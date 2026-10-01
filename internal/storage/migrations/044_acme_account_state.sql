-- Operator-owned CA identity, not tenant certificate or machine credentials.
-- Only the API runtime can read/insert encrypted account state; no browser route.
CREATE TABLE acme_accounts (
 directory text PRIMARY KEY CHECK(directory IN (
  'https://acme-staging-v02.api.letsencrypt.org/directory',
  'https://acme-v02.api.letsencrypt.org/directory')),
 encrypted bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE acme_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE acme_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY acme_runtime_read ON acme_accounts FOR SELECT TO xingdu_app USING(true);
CREATE POLICY acme_runtime_create ON acme_accounts FOR INSERT TO xingdu_app WITH CHECK(true);
GRANT SELECT,INSERT ON acme_accounts TO xingdu_app;
