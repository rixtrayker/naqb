-- +goose Up
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    book_dir    TEXT NOT NULL,
    chapter_num INTEGER,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL CHECK(role IN ('user','assistant','tool')),
    content     TEXT NOT NULL,
    tokens_in   INTEGER DEFAULT 0,
    tokens_out  INTEGER DEFAULT 0,
    model       TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_session ON messages(session_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_session;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS sessions;
