package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/llm"
)

// Notification represents a persistent alert written to ~/.naqb/notifications/
// when a background job fails due to a provider error. Survives process restarts.
type Notification struct {
	ID         string    `json:"id"`
	JobID      string    `json:"job_id"`
	JobType    string    `json:"job_type"`
	BookDir    string    `json:"book_dir"`
	ChapterNum int       `json:"chapter_num"`
	ErrorKind  string    `json:"error_kind"` // "auth" | "credit" | "rate_limit" | "provider_unavailable" | "unknown"
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
}

// WriteNotification persists a notification as a JSON file in notifyDir().
func WriteNotification(n Notification) error {
	dir := notifyDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("notifications: create dir: %w", err)
	}
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("notifications: marshal: %w", err)
	}
	path := filepath.Join(dir, n.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// ReadNotifications reads all notification files from notifyDir().
// Files that cannot be parsed are silently skipped.
func ReadNotifications() ([]Notification, error) {
	dir := notifyDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notifications: read dir: %w", err)
	}

	var out []Notification
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var n Notification
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// ClearNotificationsForJob removes notifications associated with a specific job ID.
func ClearNotificationsForJob(jobID string) {
	dir := notifyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var n Notification
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		if n.JobID == jobID {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// ClearNotifications removes all notification files from notifyDir().
func ClearNotifications() error {
	dir := notifyDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("notifications: read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

// notifyDirOverride allows tests to redirect notifications to a temp directory.
var notifyDirOverride string

// notifyDir returns the path to ~/.naqb/notifications/.
func notifyDir() string {
	if notifyDirOverride != "" {
		return notifyDirOverride
	}
	return filepath.Join(config.NaqbDir(), "notifications")
}

// errorKind classifies an error into a short string suitable for a notification.
func errorKind(err error) string {
	switch {
	case llm.IsAuthError(err):
		return "auth"
	case llm.IsCreditError(err):
		return "credit"
	case llm.IsRateLimit(err):
		return "rate_limit"
	case llm.IsProviderError(err):
		return "provider_unavailable"
	default:
		return "unknown"
	}
}
