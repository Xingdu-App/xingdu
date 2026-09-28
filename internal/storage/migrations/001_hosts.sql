CREATE TABLE hosts (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (length(name) > 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'online', 'offline')),
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
