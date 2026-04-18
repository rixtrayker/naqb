package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	internaldb "github.com/amr/naqb/internal/db"
)

// LensType identifies the analytical perspective applied to a passage.
type LensType string

const (
	LensHistorical    LensType = "HISTORICAL"
	LensLinguistic    LensType = "LINGUISTIC"
	LensPhilosophical LensType = "PHILOSOPHICAL"
	LensTheological   LensType = "THEOLOGICAL"
	LensComparative   LensType = "COMPARATIVE"
	LensCritical      LensType = "CRITICAL"
)

// EpistemicState represents the accumulated understanding of a book project.
// It is the shared memory injected into agent context across sessions.
type EpistemicState struct {
	BookID            string         `json:"book_id"`
	Version           int            `json:"version"`
	ResearchQs        []string       `json:"research_questions,omitempty"`
	Thesis            string         `json:"thesis,omitempty"`
	Outline           string         `json:"outline,omitempty"`
	EstablishedClaims []string       `json:"established_claims,omitempty"` // claim IDs
	AuthorProfile     map[string]any `json:"author_profile,omitempty"`
	CorpusView        map[string]any `json:"corpus_view,omitempty"`
	AnalyticalStance  string         `json:"analytical_stance,omitempty"`
	ProcessingLog     []string       `json:"processing_log,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at,omitempty"`
}

// Load retrieves the EpistemicState for a book from the database.
// Returns a new blank state if none exists yet.
func Load(ctx context.Context, db *sql.DB, bookID string) (*EpistemicState, error) {
	_, dataJSON, version, err := internaldb.GetEpistemicState(db, bookID)
	if err != nil {
		return nil, fmt.Errorf("epistemic: load: %w", err)
	}
	if dataJSON == "" {
		return &EpistemicState{
			BookID:  bookID,
			Version: 1,
		}, nil
	}
	var s EpistemicState
	if err := json.Unmarshal([]byte(dataJSON), &s); err != nil {
		return nil, fmt.Errorf("epistemic: unmarshal: %w", err)
	}
	s.Version = version
	return &s, nil
}

// Save persists the EpistemicState to the database.
func (e *EpistemicState) Save(ctx context.Context, db *sql.DB) error {
	e.UpdatedAt = time.Now()
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("epistemic: marshal: %w", err)
	}
	id := uuid.NewString()
	return internaldb.UpsertEpistemicState(db, id, e.BookID, string(data), e.Version)
}

// Accumulate appends a processing log entry from a lens pass.
func (e *EpistemicState) Accumulate(lens LensType, output string) {
	entry := fmt.Sprintf("[%s] %s: %s", time.Now().Format(time.RFC3339), lens, output)
	e.ProcessingLog = append(e.ProcessingLog, entry)
}

// AddEstablishedClaim registers a claim ID as established (verified, high confidence).
func (e *EpistemicState) AddEstablishedClaim(claimID string) {
	for _, id := range e.EstablishedClaims {
		if id == claimID {
			return // already present
		}
	}
	e.EstablishedClaims = append(e.EstablishedClaims, claimID)
}

// Summary returns a concise text summary for injection into agent prompts.
func (e *EpistemicState) Summary() string {
	if e == nil {
		return ""
	}
	var parts []string
	if e.Thesis != "" {
		parts = append(parts, "Thesis: "+e.Thesis)
	}
	if e.AnalyticalStance != "" {
		parts = append(parts, "Stance: "+e.AnalyticalStance)
	}
	if len(e.ResearchQs) > 0 {
		parts = append(parts, fmt.Sprintf("Open research questions: %d", len(e.ResearchQs)))
	}
	if len(e.EstablishedClaims) > 0 {
		parts = append(parts, fmt.Sprintf("Established claims: %d", len(e.EstablishedClaims)))
	}
	if len(parts) == 0 {
		return "(no epistemic state yet)"
	}
	result := "## Epistemic State\n"
	for _, p := range parts {
		result += "- " + p + "\n"
	}
	return result
}
