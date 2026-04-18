-- +goose Up

CREATE TABLE IF NOT EXISTS epistemic_states (
    id         TEXT PRIMARY KEY,
    book_id    TEXT NOT NULL UNIQUE,
    version    INTEGER DEFAULT 1,
    data       TEXT NOT NULL,  -- JSON blob of EpistemicState
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down

DROP TABLE IF EXISTS epistemic_states;
