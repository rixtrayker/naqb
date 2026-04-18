// Package knowledge provides the claim graph, epistemic state, and ingestion
// pipeline for نقب's scholarly text intelligence engine.
package knowledge

import (
	"database/sql"
	"fmt"
	"time"

	internaldb "github.com/amr/naqb/internal/db"
)

// ClaimType classifies the epistemological nature of a claim.
type ClaimType string

const (
	ClaimFactual       ClaimType = "FACTUAL"
	ClaimInterpretive  ClaimType = "INTERPRETIVE"
	ClaimMethodological ClaimType = "METHODOLOGICAL"
	ClaimEvaluative    ClaimType = "EVALUATIVE"
	ClaimContextual    ClaimType = "CONTEXTUAL"
	ClaimRelational    ClaimType = "RELATIONAL"
	ClaimNormative     ClaimType = "NORMATIVE"
	ClaimHypothetical  ClaimType = "HYPOTHETICAL"
)

// Claim is an atomic proposition extracted from a chapter.
type Claim struct {
	ID          string
	BookID      string
	Chapter     int
	Paragraph   int
	SentenceIdx int
	Text        string
	ClaimType   ClaimType
	Confidence  float64
	Language    string
	CreatedAt   time.Time
}

// ClaimFilter holds query parameters for claim lookup.
type ClaimFilter struct {
	BookID    string
	Chapter   int // 0 = all chapters
	ClaimType ClaimType
}

// ClaimStore provides CRUD for claims backed by SQLite.
type ClaimStore struct {
	db *sql.DB // plain *sql.DB from internal/db.Open
}

// NewClaimStore creates a ClaimStore.
func NewClaimStore(db *sql.DB) *ClaimStore {
	return &ClaimStore{db: db}
}

// validClaimTypes is the set of recognized claim types.
var validClaimTypes = map[ClaimType]bool{
	ClaimFactual: true, ClaimInterpretive: true, ClaimMethodological: true,
	ClaimEvaluative: true, ClaimContextual: true, ClaimRelational: true,
	ClaimNormative: true, ClaimHypothetical: true,
}

// Add inserts a claim into the database.
func (s *ClaimStore) Add(c Claim) error {
	if c.ID == "" {
		return fmt.Errorf("claim: ID is required")
	}
	if c.BookID == "" {
		return fmt.Errorf("claim: BookID is required")
	}
	if c.ClaimType != "" && !validClaimTypes[c.ClaimType] {
		return fmt.Errorf("claim: invalid type %q", c.ClaimType)
	}
	if c.Language == "" {
		c.Language = "ar"
	}
	if c.Confidence == 0 {
		c.Confidence = 1.0
	}
	if c.Confidence < 0 || c.Confidence > 1.0 {
		return fmt.Errorf("claim: confidence must be between 0.0 and 1.0, got %f", c.Confidence)
	}
	return internaldb.InsertClaim(s.db, internaldb.ClaimRow{
		ID:          c.ID,
		BookID:      c.BookID,
		Chapter:     c.Chapter,
		Paragraph:   c.Paragraph,
		SentenceIdx: c.SentenceIdx,
		Text:        c.Text,
		ClaimType:   string(c.ClaimType),
		Confidence:  c.Confidence,
		Language:    c.Language,
	})
}

// Get retrieves a claim by ID. Returns nil, nil if not found.
func (s *ClaimStore) Get(id string) (*Claim, error) {
	row, err := internaldb.GetClaim(s.db, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return rowToClaim(row), nil
}

// List returns claims matching the filter.
func (s *ClaimStore) List(f ClaimFilter) ([]Claim, error) {
	rows, err := internaldb.ListClaims(s.db, f.BookID, f.Chapter)
	if err != nil {
		return nil, err
	}
	claims := make([]Claim, 0, len(rows))
	for i := range rows {
		c := rowToClaim(&rows[i])
		if f.ClaimType != "" && c.ClaimType != f.ClaimType {
			continue
		}
		claims = append(claims, *c)
	}
	return claims, nil
}

// Delete removes a claim by ID.
func (s *ClaimStore) Delete(id string) error {
	return internaldb.DeleteClaim(s.db, id)
}

func rowToClaim(r *internaldb.ClaimRow) *Claim {
	return &Claim{
		ID:          r.ID,
		BookID:      r.BookID,
		Chapter:     r.Chapter,
		Paragraph:   r.Paragraph,
		SentenceIdx: r.SentenceIdx,
		Text:        r.Text,
		ClaimType:   ClaimType(r.ClaimType),
		Confidence:  r.Confidence,
		Language:    r.Language,
		CreatedAt:   r.CreatedAt,
	}
}
