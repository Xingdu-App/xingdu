ALTER TABLE protocol_deployments DROP CONSTRAINT protocol_deployments_action_check;
ALTER TABLE protocol_deployments ADD CONSTRAINT protocol_deployments_action_check CHECK (action IN ('deploy','remove','restart'));
ALTER TABLE protocol_deployments ADD COLUMN certificate_expires_at timestamptz;
ALTER TABLE protocol_deployments ADD COLUMN service_status text NOT NULL DEFAULT 'unknown' CHECK (service_status IN ('unknown','active','inactive','missing'));
ALTER TABLE protocol_deployments ADD COLUMN service_checked_at timestamptz;
