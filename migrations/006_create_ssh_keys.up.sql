CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    name TEXT,
    private_key TEXT,
    created_at TIMESTAMPTZ
)