-- Run this against your PostgreSQL database before starting the app.
CREATE TABLE IF NOT EXISTS todos (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    completed   BOOLEAN NOT NULL DEFAULT FALSE,
    user_id     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_todos_user_id ON todos(user_id);
CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at DESC);
