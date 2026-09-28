-- Installation is a resource lifecycle separate from the latest deployment task.
ALTER TABLE protocol_deployments ADD COLUMN installed_at timestamptz;
UPDATE protocol_deployments SET installed_at=COALESCE(finished_at,created_at)
WHERE state='succeeded' AND action='deploy';
CREATE INDEX protocol_nodes ON protocol_deployments(organization_id,installed_at DESC)
WHERE installed_at IS NOT NULL AND state <> 'removed';
