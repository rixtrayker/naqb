package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	internaldb "github.com/amr/naqb/internal/db"
)

// RelationType defines the epistemic relationship between two claims.
type RelationType string

const (
	RelSupports    RelationType = "SUPPORTS"
	RelContradicts RelationType = "CONTRADICTS"
	RelQualifies   RelationType = "QUALIFIES"
	RelElaborates  RelationType = "ELABORATES"
	RelCites       RelationType = "CITES"
	RelDerivesFrom RelationType = "DERIVES_FROM"
	RelExemplifies RelationType = "EXEMPLIFIES"
	RelNegates     RelationType = "NEGATES"
)

// ClaimRelation is a directed edge between two claims.
type ClaimRelation struct {
	ID        string
	SourceID  string
	TargetID  string
	Relation  RelationType
	Weight    float64
	CreatedAt time.Time
}

// Graph manages the knowledge graph of claims and relations.
type Graph struct {
	db     *sql.DB
	claims *ClaimStore
}

// NewGraph creates a Graph backed by the given database.
func NewGraph(db *sql.DB) *Graph {
	return &Graph{
		db:     db,
		claims: NewClaimStore(db),
	}
}

// AddClaim inserts a claim into the graph.
func (g *Graph) AddClaim(_ context.Context, claim Claim) error {
	return g.claims.Add(claim)
}

// AddRelation inserts a directed relation between two claims.
// If rel.ID is empty, a new UUID is generated.
func (g *Graph) AddRelation(_ context.Context, rel ClaimRelation) error {
	if rel.ID == "" {
		rel.ID = uuid.NewString()
	}
	if rel.Weight == 0 {
		rel.Weight = 1.0
	}
	return internaldb.InsertRelation(g.db, internaldb.RelationRow{
		ID:       rel.ID,
		SourceID: rel.SourceID,
		TargetID: rel.TargetID,
		Relation: string(rel.Relation),
		Weight:   rel.Weight,
	})
}

// Query returns claims matching the given filter.
func (g *Graph) Query(_ context.Context, filter ClaimFilter) ([]Claim, error) {
	return g.claims.List(filter)
}

// GetRelations returns all outgoing relations for a claim.
func (g *Graph) GetRelations(_ context.Context, sourceID string) ([]ClaimRelation, error) {
	rows, err := internaldb.GetRelations(g.db, sourceID)
	if err != nil {
		return nil, err
	}
	rels := make([]ClaimRelation, len(rows))
	for i, r := range rows {
		rels[i] = ClaimRelation{
			ID:        r.ID,
			SourceID:  r.SourceID,
			TargetID:  r.TargetID,
			Relation:  RelationType(r.Relation),
			Weight:    r.Weight,
			CreatedAt: r.CreatedAt,
		}
	}
	return rels, nil
}

// ShortestPath finds the shortest path between two claims using BFS over the relation graph.
// Returns the sequence of relations traversed, or an error if no path exists.
func (g *Graph) ShortestPath(ctx context.Context, fromID, toID string) ([]ClaimRelation, error) {
	if fromID == toID {
		return nil, nil
	}

	type node struct {
		id   string
		path []ClaimRelation
	}

	visited := map[string]bool{fromID: true}
	queue := []node{{id: fromID, path: nil}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		rels, err := g.GetRelations(ctx, current.id)
		if err != nil {
			return nil, fmt.Errorf("shortest path: get relations from %s: %w", current.id, err)
		}

		for _, rel := range rels {
			newPath := make([]ClaimRelation, len(current.path)+1)
			copy(newPath, current.path)
			newPath[len(current.path)] = rel

			if rel.TargetID == toID {
				return newPath, nil
			}

			if !visited[rel.TargetID] {
				visited[rel.TargetID] = true
				queue = append(queue, node{id: rel.TargetID, path: newPath})
			}
		}
	}

	return nil, fmt.Errorf("no path found from %s to %s", fromID, toID)
}
