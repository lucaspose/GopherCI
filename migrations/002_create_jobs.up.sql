CREATE TABLE jobs (
    id TEXT PRIMARY KEY NOT NULL,
    repo TEXT,
    cmd TEXT[],
    user_id TEXT REFERENCES users(id),
    status TEXT,
    created_at TIMESTAMPTZ
)