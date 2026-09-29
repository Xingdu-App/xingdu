ALTER TABLE subscriptions ADD COLUMN routing jsonb NOT NULL DEFAULT 'null'::jsonb CHECK (jsonb_typeof(routing) IN ('null', 'object'));
