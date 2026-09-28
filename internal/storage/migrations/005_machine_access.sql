ALTER TABLE hosts ADD CONSTRAINT hosts_org_id_unique UNIQUE(organization_id,id);
CREATE TABLE machine_enrollments (
 token_hash text PRIMARY KEY, organization_id uuid NOT NULL, host_id uuid NOT NULL,
 mode text NOT NULL CHECK(mode IN ('monitor','manage')), expires_at timestamptz NOT NULL,
 used_at timestamptz, created_by uuid NOT NULL REFERENCES users(id),
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE CASCADE
);
CREATE INDEX enrollments_host_idx ON machine_enrollments(organization_id,host_id);
CREATE TABLE machine_agents (
 host_id uuid PRIMARY KEY, organization_id uuid NOT NULL, token_hash text NOT NULL UNIQUE,
 mode text NOT NULL CHECK(mode IN ('monitor','manage')), enrolled_at timestamptz NOT NULL DEFAULT now(),
 revoked_at timestamptz, metrics jsonb NOT NULL DEFAULT '{}',
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE CASCADE
);
CREATE TABLE machine_credentials (
 host_id uuid PRIMARY KEY, organization_id uuid NOT NULL, method text NOT NULL CHECK(method IN ('password','pem')),
 fingerprint text NOT NULL, encrypted bytea NOT NULL, saved_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE CASCADE
);
CREATE TABLE machine_jobs (
 id uuid PRIMARY KEY,organization_id uuid NOT NULL,host_id uuid NOT NULL,created_by uuid NOT NULL REFERENCES users(id),
 mode text NOT NULL CHECK(mode IN ('monitor','manage')),state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','installed','failed','cancelled')),
 result text NOT NULL DEFAULT '',encrypted bytea,created_at timestamptz NOT NULL DEFAULT now(),finished_at timestamptz,lease uuid,lease_until timestamptz,
 FOREIGN KEY(organization_id,host_id) REFERENCES hosts(organization_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX one_machine_install ON machine_jobs(host_id) WHERE state IN ('queued','running');
CREATE INDEX machine_jobs_org_idx ON machine_jobs(organization_id,host_id,created_at);
CREATE TABLE machine_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 host_id uuid NOT NULL,actor_id uuid NOT NULL,event text NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['machine_enrollments','machine_agents','machine_credentials','machine_jobs','machine_audit'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
 EXECUTE format('CREATE POLICY tenant_manage ON %I USING(organization_id=request_org_id() AND tenant_role(organization_id) IN (''owner'',''admin'')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN (''owner'',''admin''))',t);
 END LOOP;
END $$;
CREATE POLICY agent_read ON machine_agents FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY enrollment_token ON machine_enrollments FOR SELECT USING(token_hash=nullif(current_setting('app.enrollment_hash',true),''));
CREATE POLICY enrollment_consume ON machine_enrollments FOR UPDATE USING(token_hash=nullif(current_setting('app.enrollment_hash',true),'')) WITH CHECK(token_hash=nullif(current_setting('app.enrollment_hash',true),''));
CREATE POLICY agent_token_read ON machine_agents FOR SELECT USING(token_hash=nullif(current_setting('app.agent_hash',true),''));
CREATE POLICY agent_enroll ON machine_agents FOR ALL USING(EXISTS(SELECT 1 FROM machine_enrollments e WHERE e.host_id=machine_agents.host_id AND e.token_hash=nullif(current_setting('app.enrollment_hash',true),'') AND e.expires_at>now() AND e.used_at IS NULL)) WITH CHECK(EXISTS(SELECT 1 FROM machine_enrollments e WHERE e.host_id=machine_agents.host_id AND e.organization_id=machine_agents.organization_id AND e.mode=machine_agents.mode AND e.token_hash=nullif(current_setting('app.enrollment_hash',true),'') AND e.expires_at>now() AND e.used_at IS NULL));
CREATE POLICY agent_heartbeat ON machine_agents FOR UPDATE USING(token_hash=nullif(current_setting('app.agent_hash',true),'') AND revoked_at IS NULL) WITH CHECK(token_hash=nullif(current_setting('app.agent_hash',true),'') AND revoked_at IS NULL);
CREATE POLICY host_agent ON hosts FOR ALL USING(EXISTS(SELECT 1 FROM machine_agents a WHERE a.host_id=hosts.id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL)) WITH CHECK(EXISTS(SELECT 1 FROM machine_agents a WHERE a.host_id=hosts.id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL));
GRANT SELECT,INSERT,UPDATE,DELETE ON machine_enrollments,machine_agents,machine_credentials,machine_jobs TO xingdu_app;
GRANT SELECT,INSERT ON machine_audit TO xingdu_app;
GRANT USAGE ON SEQUENCE machine_audit_id_seq TO xingdu_app;
-- The worker alone can claim global queue metadata. Business operations still use RLS.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='xingdu_worker') THEN CREATE ROLE xingdu_worker NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE; END IF; END $$;
GRANT xingdu_app TO xingdu_worker;
GRANT SELECT,UPDATE ON machine_jobs TO xingdu_policy;
CREATE FUNCTION claim_machine_job() RETURNS TABLE(id uuid,organization_id uuid,created_by uuid,lease uuid)
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 UPDATE public.machine_jobs SET state='failed',result='interrupted_or_expired',encrypted=NULL,finished_at=now()
 WHERE (state='running' AND lease_until<now()) OR (state='queued' AND created_at<now()-interval '30 minutes');
 RETURN QUERY WITH candidate AS (
 SELECT j.id FROM public.machine_jobs j WHERE j.state='queued' ORDER BY j.created_at FOR UPDATE SKIP LOCKED LIMIT 1
 ) UPDATE public.machine_jobs j SET state='running',lease=gen_random_uuid(),lease_until=now()+interval '3 minutes'
 FROM candidate c WHERE j.id=c.id RETURNING j.id,j.organization_id,j.created_by,j.lease;
END $$;
ALTER FUNCTION claim_machine_job() OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION claim_machine_job() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claim_machine_job() TO xingdu_worker;
CREATE FUNCTION fail_machine_job(job uuid,claim uuid) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 UPDATE public.machine_jobs SET state='failed',result='authorization_revoked',encrypted=NULL,finished_at=now()
 WHERE id=job AND lease=claim AND state='running';
$$;
ALTER FUNCTION fail_machine_job(uuid,uuid) OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION fail_machine_job(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fail_machine_job(uuid,uuid) TO xingdu_worker;
