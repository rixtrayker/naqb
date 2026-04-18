package style

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry manages StyleImage YAML files in a directory.
// Default location: ~/.naqb/styles/
type Registry struct {
	dir string
}

// NewRegistry creates a Registry backed by the given directory.
func NewRegistry(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("style registry: mkdir %s: %w", dir, err)
	}
	return &Registry{dir: dir}, nil
}

// DefaultStylesDir returns the default styles directory (~/.naqb/styles/).
func DefaultStylesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".naqb", "styles")
}

// Save writes a StyleImage to <registry_dir>/<img.ID>.yaml.
func (r *Registry) Save(img *StyleImage) error {
	if img.ID == "" {
		return fmt.Errorf("style registry: ID is required")
	}
	data, err := yaml.Marshal(img)
	if err != nil {
		return fmt.Errorf("style registry: marshal %s: %w", img.ID, err)
	}
	return os.WriteFile(filepath.Join(r.dir, img.ID+".yaml"), data, 0o644)
}

// Get loads a StyleImage by ID.
func (r *Registry) Get(id string) (*StyleImage, error) {
	data, err := os.ReadFile(filepath.Join(r.dir, id+".yaml"))
	if err != nil {
		return nil, fmt.Errorf("style registry: get %s: %w", id, err)
	}
	var img StyleImage
	if err := yaml.Unmarshal(data, &img); err != nil {
		return nil, fmt.Errorf("style registry: parse %s: %w", id, err)
	}
	return &img, nil
}

// List returns all StyleImages in the registry.
func (r *Registry) List() ([]StyleImage, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("style registry: list: %w", err)
	}
	var imgs []StyleImage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		img, err := r.Get(id)
		if err != nil {
			continue // skip malformed files
		}
		imgs = append(imgs, *img)
	}
	return imgs, nil
}

// Delete removes a StyleImage by ID.
func (r *Registry) Delete(id string) error {
	path := filepath.Join(r.dir, id+".yaml")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("style registry: delete %s: %w", id, err)
	}
	return nil
}
