CREATE TABLE admins (
    id uuid PRIMARY KEY,
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    singleton boolean NOT NULL DEFAULT true UNIQUE CHECK (singleton),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);
ALTER TABLE hosts ADD COLUMN address text NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN ssh_port integer NOT NULL DEFAULT 22 CHECK (ssh_port BETWEEN 1 AND 65535);
ALTER TABLE hosts ADD COLUMN ssh_user text NOT NULL DEFAULT 'root';
ALTER TABLE hosts ADD COLUMN tags text[] NOT NULL DEFAULT '{}';
ALTER TABLE hosts ADD COLUMN notes text NOT NULL DEFAULT '';
CREATE UNIQUE INDEX hosts_address_port_idx ON hosts(lower(address), ssh_port) WHERE address <> '';
