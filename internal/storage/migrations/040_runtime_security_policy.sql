ALTER TABLE protocol_deployments DROP CONSTRAINT protocol_deployments_service_status_check;
ALTER TABLE protocol_deployments ADD CONSTRAINT protocol_deployments_service_status_check
 CHECK (service_status IN ('unknown','active','inactive','missing','policy_required'));
