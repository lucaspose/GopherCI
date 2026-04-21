CREATE TABLE organizations (
    id TEXT PRIMARY KEY,
    name TEXT,
    owner_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ
)