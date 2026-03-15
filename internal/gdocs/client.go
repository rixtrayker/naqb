// Package gdocs provides Google Docs integration via the Composio REST API.
// Auth is handled by Composio managed OAuth — no GCP project needed.
// The Composio API key is read from macOS Keychain via config.ComposioAPIKey().
package gdocs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	composioExecuteURL = "https://backend.composio.dev/api/v2/actions/%s/execute"
	defaultUserID      = "pg-test-f3eaa561-6583-4190-9d84-06e15fd4b522"
)

// Client wraps the Composio REST API for Google Docs actions.
type Client struct {
	apiKey     string
	userID     string
	httpClient *http.Client
}

// New creates a new Client.
// apiKey is the Composio API key (from config.ComposioAPIKey()).
// userID is the Composio external user ID (stored in book.yaml sync.composio_user_id).
func New(apiKey, userID string) *Client {
	if userID == "" {
		userID = defaultUserID
	}
	return &Client{
		apiKey:     apiKey,
		userID:     userID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// executeResult is the common Composio action response envelope.
type executeResult struct {
	Data  map[string]any `json:"data"`
	Error *string        `json:"error"`
	LogID string         `json:"log_id"`
}

func (c *Client) execute(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	body := map[string]any{
		"connectedAccountId": "", // Composio resolves from userID
		"entityId":           c.userID,
		"input":              params,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(composioExecuteURL, action)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("composio request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("composio HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var result executeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("composio parse: %w", err)
	}
	if result.Error != nil && *result.Error != "" {
		return nil, fmt.Errorf("composio action error: %s", *result.Error)
	}
	return result.Data, nil
}

// DocInfo holds the essentials returned after creating or fetching a document.
type DocInfo struct {
	ID    string
	URL   string
	Title string
}

// CreateDocument creates a new Google Doc with the given title.
func (c *Client) CreateDocument(ctx context.Context, title string) (*DocInfo, error) {
	data, err := c.execute(ctx, "GOOGLEDOCS_CREATE_DOCUMENT", map[string]any{
		"title": title,
	})
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	id, _ := data["documentId"].(string)
	url, _ := data["documentUrl"].(string)
	if url == "" {
		url, _ = data["document_url"].(string) // alternate key in some responses
	}
	if id == "" {
		return nil, fmt.Errorf("create document: no documentId in response")
	}
	return &DocInfo{ID: id, URL: url, Title: title}, nil
}

// GetDocument retrieves basic document info by ID.
func (c *Client) GetDocument(ctx context.Context, docID string) (*DocInfo, error) {
	data, err := c.execute(ctx, "GOOGLEDOCS_GET_DOCUMENT_BY_ID", map[string]any{
		"document_id": docID,
	})
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	id, _ := data["documentId"].(string)
	url, _ := data["documentUrl"].(string)
	if url == "" {
		url, _ = data["document_url"].(string)
	}
	title, _ := data["title"].(string)
	if id == "" {
		id = docID
	}
	return &DocInfo{ID: id, URL: url, Title: title}, nil
}

// ReplaceContent replaces the entire content of an existing Google Doc with
// Markdown content using GOOGLEDOCS_UPDATE_DOCUMENT_MARKDOWN.
// Falls back to GOOGLEDOCS_INSERT_TEXT_ACTION if the markdown update fails.
func (c *Client) ReplaceContent(ctx context.Context, docID, content string) error {
	_, err := c.execute(ctx, "GOOGLEDOCS_UPDATE_DOCUMENT_MARKDOWN", map[string]any{
		"document_id": docID,
		"markdown":    content,
	})
	if err != nil {
		// Fallback: insert as plain text at index 1
		return c.insertTextFallback(ctx, docID, content)
	}
	return nil
}

// insertTextFallback inserts plain text at the beginning of a document.
func (c *Client) insertTextFallback(ctx context.Context, docID, content string) error {
	_, err := c.execute(ctx, "GOOGLEDOCS_INSERT_TEXT_ACTION", map[string]any{
		"document_id": docID,
		"text":        content,
		"index":       1,
	})
	return err
}

// FindOrCreateBookDoc looks for an existing master doc by ID, or creates a
// new one titled "<bookTitle> — نقب Master". Returns the DocInfo.
func (c *Client) FindOrCreateBookDoc(ctx context.Context, existingID, bookTitle string) (*DocInfo, error) {
	if existingID != "" {
		doc, err := c.GetDocument(ctx, existingID)
		if err == nil {
			return doc, nil
		}
		// Doc not accessible — create a new one
	}
	title := bookTitle + " — نقب Master"
	return c.CreateDocument(ctx, title)
}

// GetDocumentText fetches the plain text content of a Google Doc.
func (c *Client) GetDocumentText(ctx context.Context, docID string) (string, error) {
	data, err := c.execute(ctx, "GOOGLEDOCS_GET_DOCUMENT_BY_ID", map[string]any{
		"document_id": docID,
	})
	if err != nil {
		return "", fmt.Errorf("get document text: %w", err)
	}

	// Extract text from the tabs/body structure
	return extractDocText(data), nil
}

// extractDocText walks the Google Docs API response and collects all text runs.
func extractDocText(data map[string]any) string {
	var sb strings.Builder
	extractFromAny(&sb, data)
	return strings.TrimSpace(sb.String())
}

func extractFromAny(sb *strings.Builder, v any) {
	switch val := v.(type) {
	case map[string]any:
		// textRun has a "content" field with the actual text
		if content, ok := val["content"].(string); ok {
			sb.WriteString(content)
			return
		}
		// Recurse into all values
		for _, child := range val {
			extractFromAny(sb, child)
		}
	case []any:
		for _, item := range val {
			extractFromAny(sb, item)
		}
	}
}

// BuildDocContent converts written chapter files into a single Google Docs
// compatible plain-text document. Each chapter is separated by a heading.
func BuildDocContent(chapters []ChapterContent) string {
	var sb strings.Builder
	for i, ch := range chapters {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("# Chapter %d: %s\n\n", ch.Number, ch.Title))
		sb.WriteString(ch.Body)
	}
	return sb.String()
}

// ChapterContent holds a chapter's metadata and content for syncing.
type ChapterContent struct {
	Number int
	Title  string
	Body   string
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
