package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/amr/naqb/internal/store"
)

const defaultChromaHost = "http://localhost:8000"

// ChromaStore is a VectorStore backed by a Chroma HTTP server.
// Uses the Chroma v1 REST API directly (no SDK dependency).
type ChromaStore struct {
	host       string
	collection string
	dimensions int
	client     *http.Client
	collID     string // Chroma internal collection UUID
}

// NewChroma creates a ChromaStore.
func NewChroma(cfg VectorConfig) (*ChromaStore, error) {
	host := cfg.Host
	if host == "" {
		host = defaultChromaHost
	}
	s := &ChromaStore{
		host:       host,
		collection: cfg.CollectionName,
		dimensions: cfg.Dimensions,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
	return s, nil
}

// chromaRequest sends a JSON request to the Chroma API.
func (s *ChromaStore) chromaRequest(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("chroma: marshal: %w", err)
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.host+path, r)
	if err != nil {
		return nil, 0, fmt.Errorf("chroma: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("chroma: HTTP: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, nil
}

// ensureCollection creates the collection if it doesn't exist.
func (s *ChromaStore) ensureCollection(ctx context.Context) error {
	if s.collID != "" {
		return nil
	}

	// Try to get existing collection
	body, status, err := s.chromaRequest(ctx, http.MethodGet, "/api/v1/collections/"+s.collection, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &result); err == nil && result.ID != "" {
			s.collID = result.ID
			return nil
		}
	}

	// Create collection
	createBody := map[string]any{
		"name": s.collection,
		"metadata": map[string]any{
			"hnsw:space": "cosine",
		},
	}
	body, status, err = s.chromaRequest(ctx, http.MethodPost, "/api/v1/collections", createBody)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("chroma: create collection failed (%d): %s", status, string(body))
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("chroma: parse create response: %w", err)
	}
	s.collID = result.ID
	return nil
}

// Upsert adds or updates documents in the collection.
func (s *ChromaStore) Upsert(ctx context.Context, docs []store.VectorDoc) error {
	if len(docs) == 0 {
		return nil
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}

	// Validate vector dimensions
	for i, d := range docs {
		if len(d.Vector) > 0 && len(d.Vector) != s.dimensions {
			return fmt.Errorf("chroma: doc %d (%s) vector dimension %d does not match store dimension %d",
				i, d.ID, len(d.Vector), s.dimensions)
		}
	}

	ids := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))
	documents := make([]string, len(docs))
	metadatas := make([]map[string]any, len(docs))

	for i, d := range docs {
		ids[i] = d.ID
		embeddings[i] = d.Vector
		documents[i] = d.Content
		meta := make(map[string]any)
		for k, v := range d.Metadata {
			meta[k] = v
		}
		if d.BookID != "" {
			meta["book_id"] = d.BookID
		}
		if d.Chapter > 0 {
			meta["chapter"] = d.Chapter
		}
		if d.Language != "" {
			meta["language"] = d.Language
		}
		metadatas[i] = meta
	}

	body := map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"documents":  documents,
		"metadatas":  metadatas,
	}
	respBody, status, err := s.chromaRequest(ctx, http.MethodPost,
		"/api/v1/collections/"+s.collID+"/upsert", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("chroma: upsert failed (%d): %s", status, string(respBody))
	}
	return nil
}

// Delete removes documents from the collection.
func (s *ChromaStore) Delete(ctx context.Context, ids []string) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	body := map[string]any{"ids": ids}
	_, status, err := s.chromaRequest(ctx, http.MethodPost,
		"/api/v1/collections/"+s.collID+"/delete", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("chroma: delete failed (%d)", status)
	}
	return nil
}

// Search performs a vector similarity search.
func (s *ChromaStore) Search(ctx context.Context, query []float32, topK int, filter store.Filter) ([]store.SearchResult, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	if topK <= 0 {
		topK = 10
	}

	reqBody := map[string]any{
		"query_embeddings": [][]float32{query},
		"n_results":        topK,
		"include":          []string{"documents", "distances", "metadatas"},
	}

	// Apply filter
	if len(filter.Clauses) > 0 {
		where := map[string]any{}
		for _, clause := range filter.Clauses {
			where[clause.Field] = clause.Value
		}
		reqBody["where"] = where
	}

	respBody, status, err := s.chromaRequest(ctx, http.MethodPost,
		"/api/v1/collections/"+s.collID+"/query", reqBody)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("chroma: search failed (%d): %s", status, string(respBody))
	}

	var result struct {
		IDs       [][]string               `json:"ids"`
		Documents [][]string               `json:"documents"`
		Distances [][]float64              `json:"distances"`
		Metadatas [][]map[string]any       `json:"metadatas"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("chroma: parse search response: %w", err)
	}

	if len(result.IDs) == 0 || len(result.IDs[0]) == 0 {
		return nil, nil
	}
	if len(result.Documents) == 0 || len(result.Documents[0]) != len(result.IDs[0]) {
		return nil, fmt.Errorf("chroma: malformed response: IDs and Documents length mismatch")
	}
	if len(result.Distances) == 0 || len(result.Distances[0]) != len(result.IDs[0]) {
		return nil, fmt.Errorf("chroma: malformed response: IDs and Distances length mismatch")
	}

	ids := result.IDs[0]
	docs := result.Documents[0]
	distances := result.Distances[0]

	results := make([]store.SearchResult, len(ids))
	for i := range ids {
		// Chroma uses L2 distance for cosine (normalized), convert to similarity
		score := float32(1.0 - distances[i])
		if score < 0 {
			score = 0
		}
		meta := map[string]string{}
		if i < len(result.Metadatas[0]) {
			for k, v := range result.Metadatas[0][i] {
				meta[k] = fmt.Sprintf("%v", v)
			}
		}
		results[i] = store.SearchResult{
			ID:       ids[i],
			Content:  docs[i],
			Score:    score,
			Metadata: meta,
		}
	}
	return results, nil
}

// SearchByID retrieves a document by its ID.
func (s *ChromaStore) SearchByID(ctx context.Context, id string) (*store.VectorDoc, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	body := map[string]any{
		"ids":     []string{id},
		"include": []string{"documents", "embeddings", "metadatas"},
	}
	respBody, status, err := s.chromaRequest(ctx, http.MethodPost,
		"/api/v1/collections/"+s.collID+"/get", body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("chroma: get failed (%d): %s", status, string(respBody))
	}

	var result struct {
		IDs        []string      `json:"ids"`
		Documents  []string      `json:"documents"`
		Embeddings [][]float32   `json:"embeddings"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("chroma: parse get response: %w", err)
	}
	if len(result.IDs) == 0 {
		return nil, nil
	}
	doc := &store.VectorDoc{
		ID:      result.IDs[0],
		Content: result.Documents[0],
	}
	if len(result.Embeddings) > 0 {
		doc.Vector = result.Embeddings[0]
	}
	return doc, nil
}

// CreateCollection creates a new collection (no-op if it already exists).
func (s *ChromaStore) CreateCollection(ctx context.Context, cfg store.CollectionConfig) error {
	old := s.collection
	s.collection = cfg.Name
	s.collID = ""
	err := s.ensureCollection(ctx)
	if err != nil {
		s.collection = old
		s.collID = ""
	}
	return err
}

// DropCollection deletes the collection.
func (s *ChromaStore) DropCollection(ctx context.Context, name string) error {
	_, status, err := s.chromaRequest(ctx, http.MethodDelete, "/api/v1/collections/"+name, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return fmt.Errorf("chroma: drop collection failed (%d)", status)
	}
	if s.collection == name {
		s.collID = ""
	}
	return nil
}

// Close is a no-op for the HTTP-based Chroma client.
func (s *ChromaStore) Close() error { return nil }
