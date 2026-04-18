package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StackRegistry manages ContextStack YAML files in a directory.
// Default location: ~/.naqb/stacks/
type StackRegistry struct {
	dir string
}

// NewStackRegistry creates a registry backed by the given directory.
func NewStackRegistry(dir string) (*StackRegistry, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("context registry: mkdir %s: %w", dir, err)
	}
	return &StackRegistry{dir: dir}, nil
}

// DefaultStacksDir returns the default stacks directory (~/.naqb/stacks/).
func DefaultStacksDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".naqb", "stacks")
}

// Save writes a ContextStack to <registry_dir>/<stack.Name>.yaml.
func (r *StackRegistry) Save(stack *ContextStack) error {
	if stack.Name == "" {
		return fmt.Errorf("context registry: stack name is required")
	}
	data, err := yaml.Marshal(stack)
	if err != nil {
		return fmt.Errorf("context registry: marshal %s: %w", stack.Name, err)
	}
	path := filepath.Join(r.dir, stack.Name+".yaml")
	return os.WriteFile(path, data, 0o644)
}

// Load reads a ContextStack by name.
func (r *StackRegistry) Load(name string) (*ContextStack, error) {
	path := filepath.Join(r.dir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("context registry: load %s: %w", name, err)
	}
	var stack ContextStack
	if err := yaml.Unmarshal(data, &stack); err != nil {
		return nil, fmt.Errorf("context registry: parse %s: %w", name, err)
	}
	return &stack, nil
}

// List returns the names of all available stacks.
func (r *StackRegistry) List() ([]string, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("context registry: list: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			names = append(names, strings.TrimSuffix(name, ".yaml"))
		}
	}
	return names, nil
}

// Delete removes a stack from the registry.
func (r *StackRegistry) Delete(name string) error {
	path := filepath.Join(r.dir, name+".yaml")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("context registry: delete %s: %w", name, err)
	}
	return nil
}
