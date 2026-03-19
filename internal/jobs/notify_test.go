package jobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amr/naqb/internal/llm"
)

func setTestNotifyDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "notifications")
	notifyDirOverride = dir
	t.Cleanup(func() { notifyDirOverride = "" })
}

func TestWriteAndReadNotifications(t *testing.T) {
	setTestNotifyDir(t)

	n := Notification{
		JobID:      "job-1",
		JobType:    "pipeline",
		BookDir:    "/books/test",
		ChapterNum: 3,
		ErrorKind:  "auth",
		Message:    "bad api key",
	}
	if err := WriteNotification(n); err != nil {
		t.Fatalf("WriteNotification: %v", err)
	}

	list, err := ReadNotifications()
	if err != nil {
		t.Fatalf("ReadNotifications: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(list))
	}
	if list[0].JobID != "job-1" {
		t.Errorf("JobID = %q, want job-1", list[0].JobID)
	}
	if list[0].ErrorKind != "auth" {
		t.Errorf("ErrorKind = %q, want auth", list[0].ErrorKind)
	}
	if list[0].ID == "" {
		t.Error("ID should be auto-populated")
	}
}

func TestClearNotifications(t *testing.T) {
	setTestNotifyDir(t)

	for i := range 3 {
		n := Notification{JobID: string(rune('A' + i)), ErrorKind: "unknown"}
		if err := WriteNotification(n); err != nil {
			t.Fatalf("WriteNotification %d: %v", i, err)
		}
	}

	if err := ClearNotifications(); err != nil {
		t.Fatalf("ClearNotifications: %v", err)
	}

	list, err := ReadNotifications()
	if err != nil {
		t.Fatalf("ReadNotifications after clear: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 notifications after clear, got %d", len(list))
	}
}

func TestReadNotifications_EmptyDir(t *testing.T) {
	setTestNotifyDir(t)
	// Create the dir but leave it empty.
	_ = os.MkdirAll(notifyDirOverride, 0o750)

	list, err := ReadNotifications()
	if err != nil {
		t.Fatalf("ReadNotifications on empty dir: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0, got %d", len(list))
	}
}

func TestReadNotifications_NonExistentDir(t *testing.T) {
	setTestNotifyDir(t)
	// Do NOT create the dir — it should not exist yet.

	list, err := ReadNotifications()
	if err != nil {
		t.Fatalf("ReadNotifications on missing dir should not error: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil slice, got %v", list)
	}
}

func TestClearNotificationsForJob(t *testing.T) {
	setTestNotifyDir(t)

	// Write 3 notifications: 2 for job-A, 1 for job-B
	for _, n := range []Notification{
		{JobID: "job-A", ErrorKind: "auth", Message: "bad key 1"},
		{JobID: "job-A", ErrorKind: "credit", Message: "no credit"},
		{JobID: "job-B", ErrorKind: "rate_limit", Message: "slow down"},
	} {
		if err := WriteNotification(n); err != nil {
			t.Fatalf("WriteNotification: %v", err)
		}
	}

	// Clear only job-A notifications
	ClearNotificationsForJob("job-A")

	list, err := ReadNotifications()
	if err != nil {
		t.Fatalf("ReadNotifications: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 notification after clearing job-A, got %d", len(list))
	}
	if list[0].JobID != "job-B" {
		t.Errorf("remaining notification should be job-B, got %q", list[0].JobID)
	}
}

func TestErrorKind_Auth(t *testing.T) {
	err := &llm.ErrAuthFailed{Provider: "p", Detail: "bad key"}
	if got := errorKind(err); got != "auth" {
		t.Errorf("errorKind(auth) = %q, want auth", got)
	}
}

func TestErrorKind_Credit(t *testing.T) {
	err := &llm.ErrCreditExhausted{Provider: "p", Detail: "no credit"}
	if got := errorKind(err); got != "credit" {
		t.Errorf("errorKind(credit) = %q, want credit", got)
	}
}

func TestErrorKind_RateLimit(t *testing.T) {
	err := &llm.ErrRateLimit{Provider: "p", Detail: "too fast"}
	if got := errorKind(err); got != "rate_limit" {
		t.Errorf("errorKind(rate_limit) = %q, want rate_limit", got)
	}
}

func TestErrorKind_ProviderUnavailable(t *testing.T) {
	err := &llm.ErrProviderUnavailable{Provider: "p", StatusCode: 503}
	if got := errorKind(err); got != "provider_unavailable" {
		t.Errorf("errorKind(unavailable) = %q, want provider_unavailable", got)
	}
}

func TestErrorKind_Unknown(t *testing.T) {
	err := os.ErrNotExist
	if got := errorKind(err); got != "unknown" {
		t.Errorf("errorKind(unknown) = %q, want unknown", got)
	}
}
