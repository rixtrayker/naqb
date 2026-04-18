package vault

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amr/naqb/pkg/config"
)

// newTestRegistry creates a Registry backed by a temp dir and overrides the
// global registry path for the duration of the test.
func newTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestLoadRegistry_FirstRun(t *testing.T) {
	// Point the registry at a non-existent file (empty temp dir)
	dir := newTestDir(t)
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry on first run: %v", err)
	}
	if len(reg.Vaults) == 0 {
		t.Fatal("expected at least one vault (default)")
	}
	if reg.Vaults[0].Name != DefaultVaultName {
		t.Errorf("first vault should be %q, got %q", DefaultVaultName, reg.Vaults[0].Name)
	}
}

func TestLoadRegistry_CorruptYAML(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	// Write corrupt YAML to the registry file
	regPath := filepath.Join(dir, ".naqb", "vault.yaml")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, []byte("{{{{corrupt: [yaml: broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry should not error on corrupt YAML, got: %v", err)
	}
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(reg.Vaults) == 0 {
		t.Fatal("expected at least one vault (default)")
	}
	if reg.Vaults[0].Name != DefaultVaultName {
		t.Errorf("expected default vault, got %q", reg.Vaults[0].Name)
	}
}

func TestSaveAndLoadRegistry(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	reg := &Registry{
		Vaults: []VaultEntry{
			{Name: DefaultVaultName, Path: filepath.Join(dir, "projects")},
			{Name: "work", Path: filepath.Join(dir, "work-books")},
		},
	}
	if err := SaveRegistry(reg); err != nil {
		t.Fatalf("SaveRegistry: %v", err)
	}

	loaded, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(loaded.Vaults) != 2 {
		t.Errorf("expected 2 vaults, got %d", len(loaded.Vaults))
	}
	if loaded.Vaults[1].Name != "work" {
		t.Errorf("expected vault[1].Name = %q, got %q", "work", loaded.Vaults[1].Name)
	}
}

func TestAddAndRemoveVault(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	// Add a vault
	extraDir := filepath.Join(dir, "extra-books")
	if err := os.MkdirAll(extraDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := AddVault("extra", extraDir); err != nil {
		t.Fatalf("AddVault: %v", err)
	}

	// Verify it was saved
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range reg.Vaults {
		if v.Name == "extra" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'extra' vault to be in registry")
	}

	// Remove it
	if err := RemoveVault("extra"); err != nil {
		t.Fatalf("RemoveVault: %v", err)
	}

	reg, _ = LoadRegistry()
	for _, v := range reg.Vaults {
		if v.Name == "extra" {
			t.Error("expected 'extra' vault to be removed")
		}
	}
}

func TestRemoveVault_CannotRemoveDefault(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	if err := RemoveVault(DefaultVaultName); err == nil {
		t.Error("expected error when removing default vault")
	}
}

func TestAddVault_DuplicateName(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	extraDir := filepath.Join(dir, "extra")
	_ = os.MkdirAll(extraDir, 0o750)
	if err := AddVault("extra", extraDir); err != nil {
		t.Fatal(err)
	}
	if err := AddVault("extra", extraDir); err == nil {
		t.Error("expected error adding duplicate vault name")
	}
}

func TestRemoveVault_NotFound(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	if err := RemoveVault("nonexistent"); err == nil {
		t.Error("expected error removing nonexistent vault")
	}
}

func TestListProjects(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	// Create a fake vault with two projects
	vaultDir := filepath.Join(dir, "projects")
	createFakeProject(t, vaultDir, "book-one", &config.BookConfig{
		Title:    "Book One",
		Language: "ar",
		Chapters: []config.Chapter{
			{Number: 1, Title: "Intro", File: "ch-01.md"},
		},
	})
	createFakeProject(t, vaultDir, "book-two", &config.BookConfig{
		Title:    "Book Two",
		Language: "en",
		Chapters: []config.Chapter{
			{Number: 1, Title: "Start", File: "ch-01.md"},
			{Number: 2, Title: "Middle", File: "ch-02.md"},
		},
	})

	// Register the vault as the default
	reg := &Registry{
		Vaults: []VaultEntry{
			{Name: DefaultVaultName, Path: vaultDir},
		},
	}
	if err := SaveRegistry(reg); err != nil {
		t.Fatal(err)
	}

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	// Both projects should have titles loaded from book.yaml
	titles := map[string]bool{}
	for _, p := range projects {
		titles[p.Title] = true
	}
	if !titles["Book One"] {
		t.Error("expected 'Book One' in project list")
	}
	if !titles["Book Two"] {
		t.Error("expected 'Book Two' in project list")
	}
}

func TestListProjects_EmptyVault(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	// Vault exists but has no projects
	vaultDir := filepath.Join(dir, "empty-vault")
	_ = os.MkdirAll(vaultDir, 0o750)
	reg := &Registry{
		Vaults: []VaultEntry{
			{Name: DefaultVaultName, Path: vaultDir},
		},
	}
	_ = SaveRegistry(reg)

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects on empty vault: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestListProjects_MissingVaultDir(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	// Vault dir doesn't exist on disk
	reg := &Registry{
		Vaults: []VaultEntry{
			{Name: DefaultVaultName, Path: filepath.Join(dir, "nonexistent")},
		},
	}
	_ = SaveRegistry(reg)

	// Should not error — just skip the missing vault
	_, err := ListProjects()
	if err != nil {
		t.Errorf("expected no error for missing vault dir, got: %v", err)
	}
}

func TestRecordRecent(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	if err := RecordRecent("/path/to/book-a", "book-a", "ar"); err != nil {
		t.Fatalf("RecordRecent: %v", err)
	}
	if err := RecordRecent("/path/to/book-b", "book-b", "en"); err != nil {
		t.Fatalf("RecordRecent: %v", err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Recents) != 2 {
		t.Fatalf("expected 2 recents, got %d", len(reg.Recents))
	}
	// Most recent should be first
	if reg.Recents[0].Name != "book-b" {
		t.Errorf("expected 'book-b' first, got %q", reg.Recents[0].Name)
	}
}

func TestRecordRecent_Deduplicates(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	_ = RecordRecent("/path/book-a", "book-a", "ar")
	_ = RecordRecent("/path/book-a", "book-a", "ar") // same path

	reg, _ := LoadRegistry()
	if len(reg.Recents) != 1 {
		t.Errorf("expected deduplication to keep 1 entry, got %d", len(reg.Recents))
	}
}

func TestFindProject(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	vaultDir := filepath.Join(dir, "projects")
	createFakeProject(t, vaultDir, "my-book", &config.BookConfig{
		Title:    "My Book",
		Language: "ar",
	})
	reg := &Registry{
		Vaults: []VaultEntry{{Name: DefaultVaultName, Path: vaultDir}},
	}
	_ = SaveRegistry(reg)

	p, err := FindProject("my-book")
	if err != nil {
		t.Fatalf("FindProject: %v", err)
	}
	if p.Name != "my-book" {
		t.Errorf("expected name 'my-book', got %q", p.Name)
	}
}

func TestFindProject_NotFound(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	_, err := FindProject("nonexistent-book")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestEnsureDefaultVault(t *testing.T) {
	dir := newTestDir(t)
	t.Setenv("HOME", dir)

	if err := EnsureDefaultVault(); err != nil {
		t.Fatalf("EnsureDefaultVault: %v", err)
	}
	if _, err := os.Stat(DefaultVaultPath()); err != nil {
		t.Error("default vault directory should exist after EnsureDefaultVault")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func createFakeProject(t *testing.T, vaultDir, name string, cfg *config.BookConfig) {
	t.Helper()
	projectDir := filepath.Join(vaultDir, name)
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveBook(projectDir, cfg); err != nil {
		t.Fatal(err)
	}
	// Touch book.yaml with a distinct time so sort order is deterministic
	_ = os.Chtimes(filepath.Join(projectDir, "book.yaml"), time.Now(), time.Now())
}
