-- +goose Up

CREATE TABLE IF NOT EXISTS claims (
    id           TEXT PRIMARY KEY,
    book_id      TEXT NOT NULL,
    chapter      INTEGER NOT NULL,
    paragraph    INTEGER NOT NULL,
    sentence_idx INTEGER NOT NULL,
    text         TEXT NOT NULL,
    claim_type   TEXT NOT NULL,
    confidence   REAL DEFAULT 1.0,
    language     TEXT DEFAULT 'ar',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_claims_book_chapter ON claims(book_id, chapter);
CREATE INDEX IF NOT EXISTS idx_claims_type ON claims(claim_type);

CREATE TABLE IF NOT EXISTS claim_relations (
    id         TEXT PRIMARY KEY,
    source_id  TEXT NOT NULL REFERENCES claims(id),
    target_id  TEXT NOT NULL REFERENCES claims(id),
    relation   TEXT NOT NULL,
    weight     REAL DEFAULT 1.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_claim_relations_source ON claim_relations(source_id);
CREATE INDEX IF NOT EXISTS idx_claim_relations_target ON claim_relations(target_id);

CREATE TABLE IF NOT EXISTS concepts (
    id       TEXT PRIMARY KEY,
    label    TEXT NOT NULL,
    language TEXT DEFAULT 'ar',
    aliases  TEXT  -- JSON array of alternative labels
);

CREATE INDEX IF NOT EXISTS idx_concepts_label ON concepts(label);

CREATE TABLE IF NOT EXISTS concept_claims (
    concept_id TEXT NOT NULL REFERENCES concepts(id),
    claim_id   TEXT NOT NULL REFERENCES claims(id),
    PRIMARY KEY (concept_id, claim_id)
);

-- +goose Down

DROP TABLE IF EXISTS concept_claims;
DROP TABLE IF EXISTS concepts;
DROP TABLE IF EXISTS claim_relations;
DROP TABLE IF EXISTS claims;
