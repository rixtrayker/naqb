package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WordCountRules holds the word count limits from config/rules.yaml.
type WordCountRules struct {
	Min    int `yaml:"min"`
	Max    int `yaml:"max"`
	Target int `yaml:"target"`
}

// Rules is the parsed config/rules.yaml for a book project.
type Rules struct {
	Language  string         `yaml:"language,omitempty"`
	WordCount WordCountRules `yaml:"word_count"`
}

// LoadRules reads config/rules.yaml from the book directory.
// Returns safe defaults if the file is absent or unparseable.
func LoadRules(bookDir string) (*Rules, error) {
	path := filepath.Join(bookDir, "config", "rules.yaml")
	r := &Rules{
		WordCount: WordCountRules{Min: 1500, Max: 5000, Target: 3000},
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if err := yaml.Unmarshal(data, r); err != nil {
		return r, err
	}
	// Fill in zero values with defaults
	if r.WordCount.Target == 0 {
		r.WordCount.Target = 3000
	}
	if r.WordCount.Min == 0 {
		r.WordCount.Min = r.WordCount.Target / 2
	}
	if r.WordCount.Max == 0 {
		r.WordCount.Max = r.WordCount.Target * 3
	}
	return r, nil
}
