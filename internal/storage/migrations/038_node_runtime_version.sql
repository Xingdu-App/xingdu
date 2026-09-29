-- Empty means the installed runtime version has not been reported.
ALTER TABLE protocol_deployments ADD COLUMN runtime_version text NOT NULL DEFAULT '' CHECK(length(runtime_version)<=64);
