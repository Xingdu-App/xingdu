ALTER TABLE machine_jobs ADD COLUMN action text NOT NULL DEFAULT 'install' CHECK(action IN ('install','upgrade'));
ALTER TABLE machine_jobs ADD COLUMN target_version text NOT NULL DEFAULT '';
ALTER TABLE machine_jobs ADD COLUMN agent_hash text NOT NULL DEFAULT '';
ALTER TABLE machine_jobs ADD COLUMN artifact_sha256 text NOT NULL DEFAULT '';
ALTER TABLE machine_jobs ADD COLUMN arch text NOT NULL DEFAULT '';
