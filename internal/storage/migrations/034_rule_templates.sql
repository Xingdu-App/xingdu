CREATE TABLE rule_templates (
 id text PRIMARY KEY,
 organization_id text NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 80),
 rules jsonb NOT NULL CHECK(jsonb_typeof(rules)='array' AND jsonb_array_length(rules)<=100),
 final_action text NOT NULL CHECK(final_action IN ('proxy','direct')),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX rule_templates_org ON rule_templates(organization_id);
ALTER TABLE rule_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE rule_templates FORCE ROW LEVEL SECURITY;
CREATE POLICY rule_template_read ON rule_templates FOR SELECT USING(organization_id=request_org_id() AND tenant_role(organization_id) IS NOT NULL);
CREATE POLICY rule_template_manage ON rule_templates FOR ALL USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
GRANT SELECT,INSERT,UPDATE,DELETE ON rule_templates TO xingdu_app;
