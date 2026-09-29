ALTER TABLE protocol_revisions ADD COLUMN port integer;
UPDATE protocol_revisions r SET port=d.port FROM protocol_deployments d WHERE d.id=r.node_id AND d.organization_id=r.organization_id;
ALTER TABLE protocol_revisions ALTER COLUMN port SET NOT NULL;
ALTER TABLE protocol_revisions ADD CHECK(port BETWEEN 1 AND 65535);
