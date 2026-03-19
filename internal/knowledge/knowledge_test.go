package knowledge

import (
	"context"
	"database/sql"
	"testing"

	internaldb "github.com/amr/naqb/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := internaldb.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestClaimCRUD(t *testing.T) {
	db := openTestDB(t)
	s := NewClaimStore(db)

	claim := Claim{
		ID:          "test-claim-1",
		BookID:      "book1",
		Chapter:     1,
		Paragraph:   2,
		SentenceIdx: 0,
		Text:        "This is a factual claim.",
		ClaimType:   ClaimFactual,
		Confidence:  0.9,
		Language:    "en",
	}

	if err := s.Add(claim); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Get("test-claim-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected claim, got nil")
	}
	if got.Text != claim.Text {
		t.Errorf("text mismatch: %q != %q", got.Text, claim.Text)
	}
	if got.ClaimType != ClaimFactual {
		t.Errorf("type mismatch: %v != %v", got.ClaimType, ClaimFactual)
	}

	// List
	claims, err := s.List(ClaimFilter{BookID: "book1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(claims) != 1 {
		t.Errorf("expected 1 claim, got %d", len(claims))
	}

	// Delete
	if err := s.Delete("test-claim-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got2, _ := s.Get("test-claim-1")
	if got2 != nil {
		t.Error("expected nil after delete")
	}
}

func TestClaimRequiresID(t *testing.T) {
	db := openTestDB(t)
	s := NewClaimStore(db)
	err := s.Add(Claim{BookID: "b", Chapter: 1, Text: "test", ClaimType: ClaimFactual})
	if err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestClaimAdd_Validation(t *testing.T) {
	db := openTestDB(t)
	s := NewClaimStore(db)

	cases := []struct {
		name    string
		claim   Claim
		wantErr bool
	}{
		{
			name:    "missing BookID",
			claim:   Claim{ID: "v1", Chapter: 1, Text: "test", ClaimType: ClaimFactual},
			wantErr: true,
		},
		{
			name:    "invalid ClaimType",
			claim:   Claim{ID: "v2", BookID: "b", Chapter: 1, Text: "test", ClaimType: "BOGUS"},
			wantErr: true,
		},
		{
			name:    "confidence > 1.0",
			claim:   Claim{ID: "v3", BookID: "b", Chapter: 1, Text: "test", ClaimType: ClaimFactual, Confidence: 1.5},
			wantErr: true,
		},
		{
			name:    "confidence < 0",
			claim:   Claim{ID: "v4", BookID: "b", Chapter: 1, Text: "test", ClaimType: ClaimFactual, Confidence: -0.1},
			wantErr: true,
		},
		{
			name: "valid claim",
			claim: Claim{
				ID: "v5", BookID: "b", Chapter: 1, Text: "valid claim",
				ClaimType: ClaimInterpretive, Confidence: 0.8, Language: "ar",
			},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Add(tc.claim)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGraph_AddAndQuery(t *testing.T) {
	db := openTestDB(t)
	g := NewGraph(db)
	ctx := context.Background()

	c1 := Claim{ID: "c1", BookID: "b", Chapter: 1, Text: "First claim", ClaimType: ClaimFactual, Language: "ar"}
	c2 := Claim{ID: "c2", BookID: "b", Chapter: 1, Text: "Second claim", ClaimType: ClaimInterpretive, Language: "ar"}

	if err := g.AddClaim(ctx, c1); err != nil {
		t.Fatalf("AddClaim c1: %v", err)
	}
	if err := g.AddClaim(ctx, c2); err != nil {
		t.Fatalf("AddClaim c2: %v", err)
	}

	rel := ClaimRelation{
		SourceID: "c1",
		TargetID: "c2",
		Relation: RelSupports,
		Weight:   0.8,
	}
	if err := g.AddRelation(ctx, rel); err != nil {
		t.Fatalf("AddRelation: %v", err)
	}

	claims, err := g.Query(ctx, ClaimFilter{BookID: "b"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(claims) != 2 {
		t.Errorf("expected 2 claims, got %d", len(claims))
	}

	rels, err := g.GetRelations(ctx, "c1")
	if err != nil {
		t.Fatalf("GetRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected 1 relation, got %d", len(rels))
	}
	if rels[0].Relation != RelSupports {
		t.Errorf("expected SUPPORTS, got %v", rels[0].Relation)
	}
}

func TestGraph_ShortestPath(t *testing.T) {
	db := openTestDB(t)
	g := NewGraph(db)
	ctx := context.Background()

	for _, c := range []Claim{
		{ID: "a", BookID: "b", Chapter: 1, Text: "A", ClaimType: ClaimFactual, Language: "ar"},
		{ID: "b", BookID: "b", Chapter: 1, Text: "B", ClaimType: ClaimFactual, Language: "ar"},
		{ID: "c", BookID: "b", Chapter: 1, Text: "C", ClaimType: ClaimFactual, Language: "ar"},
	} {
		_ = g.AddClaim(ctx, c)
	}
	_ = g.AddRelation(ctx, ClaimRelation{SourceID: "a", TargetID: "b", Relation: RelSupports})
	_ = g.AddRelation(ctx, ClaimRelation{SourceID: "b", TargetID: "c", Relation: RelElaborates})

	path, err := g.ShortestPath(ctx, "a", "c")
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if len(path) != 2 {
		t.Errorf("expected path of length 2, got %d", len(path))
	}
}

func TestEpistemicState_SaveAndLoad(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	state := &EpistemicState{
		BookID:           "book1",
		Version:          1,
		Thesis:           "This is the thesis.",
		AnalyticalStance: "critical",
		ResearchQs:       []string{"What is X?", "Why Y?"},
	}

	if err := state.Save(ctx, db); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(ctx, db, "book1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Thesis != state.Thesis {
		t.Errorf("thesis mismatch: %q != %q", loaded.Thesis, state.Thesis)
	}
	if len(loaded.ResearchQs) != 2 {
		t.Errorf("expected 2 research questions, got %d", len(loaded.ResearchQs))
	}
}

func TestEpistemicState_LoadBlank(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	s, err := Load(ctx, db, "nonexistent-book")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil blank state")
	}
	if s.BookID != "nonexistent-book" {
		t.Errorf("expected book_id to be set on blank state")
	}
}

func TestEpistemicState_Summary(t *testing.T) {
	s := &EpistemicState{
		BookID:           "b",
		Thesis:           "My thesis",
		AnalyticalStance: "comparative",
		ResearchQs:       []string{"Q1", "Q2"},
	}
	summary := s.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if summary == "(no epistemic state yet)" {
		t.Error("expected populated summary")
	}
}

func TestEpistemicState_Accumulate(t *testing.T) {
	s := &EpistemicState{BookID: "b"}
	s.Accumulate(LensHistorical, "found X")
	s.Accumulate(LensLinguistic, "found Y")
	if len(s.ProcessingLog) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(s.ProcessingLog))
	}
}
