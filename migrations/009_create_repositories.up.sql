CREATE TABLE repositories (
    id TEXT PRIMARY KEY,
    name TEXT,
    repo TEXT,
    org_id TEXT REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ
)