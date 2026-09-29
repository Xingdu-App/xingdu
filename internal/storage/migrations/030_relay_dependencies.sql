ALTER TABLE protocol_deployments ADD COLUMN relay_exit_id text;
ALTER TABLE protocol_deployments ADD CONSTRAINT relay_exit_tenant FOREIGN KEY(organization_id,relay_exit_id) REFERENCES protocol_deployments(organization_id,id);
ALTER TABLE protocol_revisions ADD COLUMN relay_exit_id text;
