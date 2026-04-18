// Package vault manages the global vault registry for sbr.
// The default vault lives at ~/sabr/ and contains all books.
// Additional vaults can be registered pointing to any directory.
package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/amr/naqb/pkg/config"
	"github.com/amr/naqb/pkg/log"
)

// DefaultVaultName is the name of the built-in default vault.
const DefaultVaultName = "default"

// DefaultVaultPath returns the preferred default book storage location.
// On macOS/Linux it uses ~/Documents/naqb-books when ~/Documents exists;
// otherwise falls back to ~/.naqb/projects.
func DefaultVaultPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		docs := filepath.Join(home, "Documents")
		if info, err := os.Stat(docs); err == nil && info.IsDir() {
			return filepath.Join(docs, "naqb-books")
		}
	}
	return filepath.Join(config.NaqbDir(), "projects")
}

// FallbackVaultPath returns the legacy vault path inside ~/.naqb.
// Used when the user explicitly wants to keep books alongside the config.
func FallbackVaultPath() string {
	return filepath.Join(config.NaqbDir(), "projects")
}

// RegistryPath returns the path to the vault registry YAML.
func RegistryPath() string {
	return filepath.Join(config.NaqbDir(), "vault.yaml")
}

// VaultEntry represents one registered vault.
type VaultEntry struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// RecentProject tracks recently opened books.
type RecentProject struct {
	Name      string    `yaml:"name"`
	Path      string    `yaml:"path"`
	Language  string    `yaml:"language,omitempty"`
	OpenedAt  time.Time `yaml:"opened_at"`
}

// Registry is the global vault registry stored at ~/.naqb/vault.yaml.
type Registry struct {
	Vaults  []VaultEntry    `yaml:"vaults"`
	Recents []RecentProject `yaml:"recents,omitempty"`
}

// LoadRegistry reads the vault registry, creating defaults if absent.
func LoadRegistry() (*Registry, error) {
	path := RegistryPath()
	reg := &Registry{}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		log.Debug("vault registry not found, using defaults", "path", path)
		reg.Vaults = []VaultEntry{
			{Name: DefaultVaultName, Path: DefaultVaultPath()},
		}
		return reg, nil
	}
	if err != nil {
		log.Error("failed to read vault registry", "path", path, "err", err)
		return nil, fmt.Errorf("reading vault registry: %w", err)
	}
	if err := yaml.Unmarshal(data, reg); err != nil {
		log.Warn("corrupt vault registry, resetting to defaults", "path", path, "err", err)
		reg.Vaults = []VaultEntry{
			{Name: DefaultVaultName, Path: DefaultVaultPath()},
		}
		return reg, nil
	}
	log.Debug("vault registry loaded", "vaults", len(reg.Vaults))
	// Always ensure default vault exists in list
	found := false
	for _, v := range reg.Vaults {
		if v.Name == DefaultVaultName {
			found = true
			break
		}
	}
	if !found {
		reg.Vaults = append([]VaultEntry{{Name: DefaultVaultName, Path: DefaultVaultPath()}}, reg.Vaults...)
	}
	return reg, nil
}

// SaveRegistry writes the registry to disk.
func SaveRegistry(reg *Registry) error {
	path := RegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Project holds metadata about a discovered book project.
type Project struct {
	Name      string
	Path      string
	VaultName string
	Title     string
	Language  string
	Domain    string
	Chapters  int
	Written   int
	ModTime   time.Time
}

// ListProjects scans all registered vaults and returns all book projects.
func ListProjects() ([]Project, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	var projects []Project
	for _, vault := range reg.Vaults {
		entries, err := os.ReadDir(vault.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectPath := filepath.Join(vault.Path, entry.Name())
			bookYAML := filepath.Join(projectPath, "book.yaml")
			info, err := os.Stat(bookYAML)
			if err != nil {
				continue
			}
			p := Project{
				Name:      entry.Name(),
				Path:      projectPath,
				VaultName: vault.Name,
				ModTime:   info.ModTime(),
			}
			// Load book.yaml for metadata
			if cfg, err := config.LoadBook(projectPath); err == nil {
				p.Title = cfg.Title
				p.Language = cfg.Language
				p.Domain = cfg.Domain
				p.Chapters = len(cfg.Chapters)
				for _, ch := range cfg.Chapters {
					if ch.Status == "written" || ch.Status == "reviewed" || ch.Status == "done" {
						p.Written++
					} else if ch.File != "" {
						// count by file existence
						if _, err := os.Stat(filepath.Join(projectPath, "chapters", ch.File)); err == nil {
							p.Written++
						}
					}
				}
			}
			if p.Title == "" {
				p.Title = entry.Name()
			}
			projects = append(projects, p)
		}
	}

	// Sort by mod time descending (most recent first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ModTime.After(projects[j].ModTime)
	})
	return projects, nil
}

// RecordRecent adds or updates a project in the recent list.
func RecordRecent(projectPath, name, language string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	entry := RecentProject{
		Name:     name,
		Path:     projectPath,
		Language: language,
		OpenedAt: time.Now(),
	}

	// Remove existing entry with same path
	filtered := reg.Recents[:0]
	for _, r := range reg.Recents {
		if r.Path != projectPath {
			filtered = append(filtered, r)
		}
	}
	// Prepend
	reg.Recents = append([]RecentProject{entry}, filtered...)
	// Keep last 20
	if len(reg.Recents) > 20 {
		reg.Recents = reg.Recents[:20]
	}
	return SaveRegistry(reg)
}

// AddVault registers a new vault directory.
func AddVault(name, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	for _, v := range reg.Vaults {
		if v.Name == name {
			return fmt.Errorf("vault %q already exists (path: %s)", name, v.Path)
		}
	}
	reg.Vaults = append(reg.Vaults, VaultEntry{Name: name, Path: absPath})
	log.Info("vault added", "name", name, "path", absPath)
	return SaveRegistry(reg)
}

// RemoveVault unregisters a vault by name (cannot remove default).
func RemoveVault(name string) error {
	if name == DefaultVaultName {
		return fmt.Errorf("cannot remove the default vault")
	}
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	filtered := reg.Vaults[:0]
	for _, v := range reg.Vaults {
		if v.Name != name {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == len(reg.Vaults) {
		return fmt.Errorf("vault %q not found", name)
	}
	reg.Vaults = filtered
	log.Info("vault removed", "name", name)
	return SaveRegistry(reg)
}

// ProjectNames returns all project names across all vaults (for autocomplete).
func ProjectNames() []string {
	projects, err := ListProjects()
	if err != nil {
		return nil
	}
	names := make([]string, len(projects))
	for i, p := range projects {
		names[i] = p.Name
	}
	return names
}

// FindProject looks up a project by name across all vaults.
func FindProject(name string) (*Project, error) {
	projects, err := ListProjects()
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].Name == name {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("project %q not found in any vault", name)
}

// EnsureDefaultVault creates the default vault directory if it doesn't exist.
// It also updates the registry entry so the default vault points to the
// path that was chosen during setup (config.DefaultVaultPath override).
func EnsureDefaultVault() error {
	path := DefaultVaultPath()

	// Check if the user configured a custom vault path in global config.
	if cfg, err := config.LoadGlobal(); err == nil && cfg.DefaultVaultPath != "" {
		path = cfg.DefaultVaultPath
	}

	if err := os.MkdirAll(path, 0o750); err != nil {
		return err
	}

	// Keep the registry entry in sync with the chosen path.
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}
	changed := false
	for i, v := range reg.Vaults {
		if v.Name == DefaultVaultName && v.Path != path {
			reg.Vaults[i].Path = path
			changed = true
		}
	}
	if changed {
		if err := SaveRegistry(reg); err != nil {
			log.Warn("vault: registry save failed during sync", "err", err)
		}
	}
	return nil
}
