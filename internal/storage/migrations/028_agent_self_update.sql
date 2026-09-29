ALTER TABLE machine_jobs ADD COLUMN transport text NOT NULL DEFAULT 'ssh' CHECK(transport IN ('ssh','agent'));
ALTER TABLE machine_jobs ADD CONSTRAINT self_upgrade_only CHECK(transport='ssh' OR (action='upgrade' AND encrypted IS NULL));
ALTER TABLE machine_agents ADD COLUMN updater_seen_at timestamptz;
CREATE POLICY machine_job_agent_read ON machine_jobs FOR SELECT USING (
 transport='agent' AND organization_id=request_org_id() AND EXISTS (
 SELECT 1 FROM machine_agents a WHERE a.host_id=machine_jobs.host_id AND a.organization_id=machine_jobs.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL));
CREATE POLICY machine_job_agent_update ON machine_jobs FOR UPDATE USING (
 transport='agent' AND organization_id=request_org_id() AND EXISTS (
 SELECT 1 FROM machine_agents a WHERE a.host_id=machine_jobs.host_id AND a.organization_id=machine_jobs.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL)) WITH CHECK (
 transport='agent' AND organization_id=request_org_id() AND EXISTS (
 SELECT 1 FROM machine_agents a WHERE a.host_id=machine_jobs.host_id AND a.organization_id=machine_jobs.organization_id AND a.token_hash=nullif(current_setting('app.agent_hash',true),'') AND a.revoked_at IS NULL));
CREATE OR REPLACE FUNCTION claim_machine_job() RETURNS TABLE(id text,organization_id text,created_by text,lease text)
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 UPDATE public.machine_jobs SET state='failed',result='interrupted_or_expired',encrypted=NULL,finished_at=now()
 WHERE (state='running' AND lease_until<now()) OR (state='queued' AND created_at<now()-interval '30 minutes');
 RETURN QUERY WITH candidate AS (
 SELECT j.id FROM public.machine_jobs j WHERE j.state='queued' AND j.transport='ssh' ORDER BY j.created_at FOR UPDATE SKIP LOCKED LIMIT 1
 ) UPDATE public.machine_jobs j SET state='running',lease=public.new_resource_id('lease'),lease_until=now()+interval '3 minutes'
 FROM candidate c WHERE j.id=c.id RETURNING j.id,j.organization_id,j.created_by,j.lease;
END $$;
ALTER FUNCTION claim_machine_job() OWNER TO xingdu_policy;
REVOKE ALL ON FUNCTION claim_machine_job() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claim_machine_job() TO xingdu_worker;
