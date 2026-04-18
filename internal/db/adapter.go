package db

import (
	"context"
	"database/sql"

	"github.com/amr/naqb/pkg/runtime"
)

// SessionStore implements runtime.SessionStore backed by the nqb SQLite schema.
type SessionStore struct {
	DB *sql.DB
}

// compile-time check
var _ runtime.SessionStore = (*SessionStore)(nil)

// NewSessionStore creates a SessionStore backed by the given DB.
func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{DB: db}
}

func (s *SessionStore) CreateSession(ctx context.Context, sessionID, bookDir string, chapterNum int) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO sessions(id, book_dir, chapter_num) VALUES (?, ?, ?)`,
		sessionID, bookDir, chapterNum,
	)
	return err
}

func (s *SessionStore) AppendMessage(ctx context.Context, msgID, sessionID, role, content, model string, tokensIn, tokensOut int) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO messages(id, session_id, role, content, model, tokens_in, tokens_out)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msgID, sessionID, role, content, model, tokensIn, tokensOut,
	)
	return err
}

func (s *SessionStore) TouchSession(ctx context.Context, sessionID string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		sessionID,
	)
	return err
}

func (s *SessionStore) ListSessions(ctx context.Context, bookDir string, limit int) ([]runtime.SessionInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, book_dir, chapter_num, created_at, updated_at
		 FROM sessions WHERE book_dir = ?
		 ORDER BY created_at DESC LIMIT ?`,
		bookDir, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []runtime.SessionInfo
	for rows.Next() {
		var si runtime.SessionInfo
		if err := rows.Scan(&si.ID, &si.BookDir, &si.ChapterNum, &si.CreatedAt, &si.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, si)
	}
	return sessions, rows.Err()
}
