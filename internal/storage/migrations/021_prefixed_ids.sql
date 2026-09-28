-- Replace UUID storage with typed text IDs in place. All relationships, policy
-- definitions, trigger definitions and function privileges are preserved in the
-- surrounding migration transaction; no parallel UUID/public-ID columns remain.
-- OAuth state is deliberately invalidated: its encrypted AAD contains old IDs.
DELETE FROM oauth_states;
CREATE TEMP TABLE id_conversion_columns(table_name text,column_name text,prefix text,PRIMARY KEY(table_name,column_name)) ON COMMIT DROP;
INSERT INTO id_conversion_columns
SELECT c.table_name,c.column_name,
 CASE
 WHEN c.column_name='organization_id' THEN 'org'
 WHEN c.column_name IN ('user_id','created_by','actor_id','target_id') THEN 'usr'
 WHEN c.column_name='host_id' THEN 'srv'
 WHEN c.column_name='node_id' THEN 'node'
 WHEN c.column_name='subscription_id' THEN 'sub'
 WHEN c.column_name='operation_id' THEN 'op'
 WHEN c.column_name='lease' THEN 'lease'
 WHEN c.column_name='id' THEN CASE c.table_name
  WHEN 'users' THEN 'usr' WHEN 'organizations' THEN 'org' WHEN 'hosts' THEN 'srv'
  WHEN 'invitations' THEN 'inv' WHEN 'machine_jobs' THEN 'job'
  WHEN 'protocol_deployments' THEN 'node' WHEN 'sessions' THEN 'ses' WHEN 'subscriptions' THEN 'sub'
 END END
FROM information_schema.columns c WHERE c.table_schema='public' AND c.udt_name='uuid';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM id_conversion_columns WHERE prefix IS NULL) THEN
 RAISE EXCEPTION 'unmapped UUID column; prefixed ID migration aborted'; END IF; END $$;
CREATE TEMP TABLE id_conversion_functions ON COMMIT DROP AS
 SELECT p.oid,p.proname,pg_get_function_identity_arguments(p.oid) args,pg_get_functiondef(p.oid) definition,
 pg_get_userbyid(p.proowner) owner,p.proacl
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname='public' AND p.prokind='f' AND p.proname IN (
 'request_user_id','request_org_id','tenant_role','claim_machine_job','fail_machine_job',
 'deployment_agent_context','subscription_context','change_account_password','transfer_organization_owner',
 'enforce_organization_quota','enforce_membership_quota','enforce_cloud_entitlement','oauth_identity_user');
CREATE TEMP TABLE id_conversion_function_grants ON COMMIT DROP AS
 SELECT f.oid,CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE quote_ident(pg_get_userbyid(a.grantee)) END grantee,
 a.privilege_type,a.is_grantable
 FROM id_conversion_functions f CROSS JOIN LATERAL aclexplode(coalesce(f.proacl,acldefault('f',(SELECT proowner FROM pg_proc WHERE oid=f.oid)))) a;
CREATE TEMP TABLE id_conversion_policies ON COMMIT DROP AS
 SELECT c.relname table_name,p.polname,
 format('CREATE POLICY %I ON public.%I AS %s FOR %s TO %s%s%s',p.polname,c.relname,
 CASE WHEN p.polpermissive THEN 'PERMISSIVE' ELSE 'RESTRICTIVE' END,
 CASE p.polcmd WHEN '*' THEN 'ALL' WHEN 'r' THEN 'SELECT' WHEN 'a' THEN 'INSERT' WHEN 'w' THEN 'UPDATE' WHEN 'd' THEN 'DELETE' END,
 (SELECT string_agg(CASE WHEN role_oid=0 THEN 'PUBLIC' ELSE quote_ident(pg_get_userbyid(role_oid)) END,',') FROM unnest(p.polroles) role_oid),
 CASE WHEN p.polqual IS NULL THEN '' ELSE ' USING ('||pg_get_expr(p.polqual,p.polrelid)||')' END,
 CASE WHEN p.polwithcheck IS NULL THEN '' ELSE ' WITH CHECK ('||pg_get_expr(p.polwithcheck,p.polrelid)||')' END) definition
 FROM pg_policy p JOIN pg_class c ON c.oid=p.polrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public';
CREATE TEMP TABLE id_conversion_triggers ON COMMIT DROP AS
 SELECT c.relname table_name,t.tgname,pg_get_triggerdef(t.oid) definition,t.tgenabled
 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND NOT t.tgisinternal;
CREATE TEMP TABLE id_conversion_foreign_keys ON COMMIT DROP AS
 SELECT c.relname table_name,k.conname,pg_get_constraintdef(k.oid) definition
 FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND k.contype='f';
CREATE TEMP TABLE id_conversion_defaults ON COMMIT DROP AS
 SELECT c.table_name,c.column_name,pg_get_expr(d.adbin,d.adrelid) definition
 FROM id_conversion_columns c JOIN pg_class t ON t.relname=c.table_name AND t.relnamespace='public'::regnamespace
 JOIN pg_attribute a ON a.attrelid=t.oid AND a.attname=c.column_name
 JOIN pg_attrdef d ON d.adrelid=t.oid AND d.adnum=a.attnum;
DO $$ DECLARE r record; BEGIN
 FOR r IN SELECT * FROM id_conversion_triggers LOOP EXECUTE format('DROP TRIGGER %I ON public.%I',r.tgname,r.table_name); END LOOP;
 FOR r IN SELECT * FROM id_conversion_policies LOOP EXECUTE format('DROP POLICY %I ON public.%I',r.polname,r.table_name); END LOOP;
 FOR r IN SELECT * FROM id_conversion_foreign_keys LOOP EXECUTE format('ALTER TABLE public.%I DROP CONSTRAINT %I',r.table_name,r.conname); END LOOP;
 FOR r IN SELECT * FROM id_conversion_defaults LOOP EXECUTE format('ALTER TABLE public.%I ALTER COLUMN %I DROP DEFAULT',r.table_name,r.column_name); END LOOP;
 -- Definitions are SQL-string/plpgsql bodies, so drop in reverse creation order
 -- without CASCADE. An unknown dependency aborts safely instead of discarding it.
 FOR r IN SELECT * FROM id_conversion_functions ORDER BY oid DESC LOOP EXECUTE format('DROP FUNCTION public.%I(%s)',r.proname,r.args); END LOOP;
 FOR r IN SELECT * FROM id_conversion_columns ORDER BY table_name,column_name LOOP
  EXECUTE format('ALTER TABLE public.%I ALTER COLUMN %I TYPE text USING CASE WHEN %I IS NULL THEN NULL ELSE %L || replace(%I::text,''-'','''') END',r.table_name,r.column_name,r.column_name,r.prefix||'_',r.column_name);
  EXECUTE format('ALTER TABLE public.%I ADD CONSTRAINT %I CHECK (%I IS NULL OR %I ~ %L)',r.table_name,r.table_name||'_'||r.column_name||'_typed_id',r.column_name,r.column_name,'^'||r.prefix||'_[0-9a-f]{32}$');
 END LOOP;
END $$;
-- PostgreSQL supplies cryptographically random UUID material. Taking the first
-- 12+12+8 hex digits of three independent values avoids their fixed version bits
-- and yields a full 128 random bits, while storing only the prefixed text ID.
CREATE FUNCTION new_resource_id(prefix text) RETURNS text LANGUAGE sql VOLATILE SET search_path=pg_catalog AS $$
 SELECT prefix||'_'||left(replace(gen_random_uuid()::text,'-',''),12)||left(replace(gen_random_uuid()::text,'-',''),12)||left(replace(gen_random_uuid()::text,'-',''),8)
$$;
REVOKE ALL ON FUNCTION new_resource_id(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION new_resource_id(text) TO xingdu_app,xingdu_policy;
DO $$ DECLARE r record; definition text; signature text; g record; BEGIN
 FOR r IN SELECT * FROM id_conversion_functions ORDER BY oid LOOP
  definition:=regexp_replace(r.definition,'\muuid\M','text','g');
  -- The only UUID generated by an application function is a worker lease.
  definition:=replace(definition,'gen_random_uuid()','public.new_resource_id(''lease'')');
  EXECUTE definition;
  signature:=format('public.%I(%s)',r.proname,regexp_replace(r.args,'\muuid\M','text','g'));
  EXECUTE 'ALTER FUNCTION '||signature||' OWNER TO '||quote_ident(r.owner);
  EXECUTE 'REVOKE ALL ON FUNCTION '||signature||' FROM PUBLIC';
  FOR g IN SELECT * FROM id_conversion_function_grants WHERE oid=r.oid LOOP
   EXECUTE 'GRANT '||g.privilege_type||' ON FUNCTION '||signature||' TO '||g.grantee||CASE WHEN g.is_grantable THEN ' WITH GRANT OPTION' ELSE '' END;
  END LOOP;
 END LOOP;
 FOR r IN SELECT * FROM id_conversion_defaults LOOP
  definition:=regexp_replace(r.definition,'\muuid\M','text','g');
  IF r.table_name='sessions' AND r.column_name='id' THEN definition:='public.new_resource_id(''ses'')'; END IF;
  EXECUTE format('ALTER TABLE public.%I ALTER COLUMN %I SET DEFAULT %s',r.table_name,r.column_name,definition);
 END LOOP;
 FOR r IN SELECT * FROM id_conversion_foreign_keys LOOP EXECUTE format('ALTER TABLE public.%I ADD CONSTRAINT %I %s',r.table_name,r.conname,r.definition); END LOOP;
 FOR r IN SELECT * FROM id_conversion_policies LOOP EXECUTE regexp_replace(r.definition,'\muuid\M','text','g'); END LOOP;
 FOR r IN SELECT * FROM id_conversion_triggers LOOP
  EXECUTE r.definition;
  IF r.tgenabled='D' THEN EXECUTE format('ALTER TABLE public.%I DISABLE TRIGGER %I',r.table_name,r.tgname);
  ELSIF r.tgenabled='R' THEN EXECUTE format('ALTER TABLE public.%I ENABLE REPLICA TRIGGER %I',r.table_name,r.tgname);
  ELSIF r.tgenabled='A' THEN EXECUTE format('ALTER TABLE public.%I ENABLE ALWAYS TRIGGER %I',r.table_name,r.tgname); END IF;
 END LOOP;
END $$;
-- Billing idempotency attempts were text UUIDs, never external Stripe IDs.
UPDATE organization_billing SET attempt='bat_'||replace(attempt,'-','') WHERE attempt ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
ALTER TABLE organization_billing ADD CONSTRAINT organization_billing_attempt_typed_id CHECK(attempt ~ '^bat_[0-9a-f]{32}$');
