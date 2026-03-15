package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/amr/naqb/internal/log"
)

// WordCountRules holds the word count limits from config/rules.yaml.
type WordCountRules struct {
	Min    int `yaml:"min"`
	Max    int `yaml:"max"`
	Target int `yaml:"target"`
}

// ResearchRules controls the automated research pipeline.
type ResearchRules struct {
	// SearchProvider selects which API to use: "tavily", "exa", or "none" (default).
	SearchProvider string `yaml:"search_provider,omitempty"`
	// MaxQueriesPerChapter caps the number of Scout-generated queries per chapter.
	MaxQueriesPerChapter int `yaml:"max_queries_per_chapter,omitempty"`
	// MaxResultsPerQuery caps results fetched per query.
	MaxResultsPerQuery int `yaml:"max_results_per_query,omitempty"`
}

// QARules controls conflict detection and gap analysis levels.
type QARules struct {
	// ConflictLevel: off | light | moderate | max
	ConflictLevel string `yaml:"conflict_level,omitempty"`
	// GapLevel: off | light | moderate | max
	GapLevel string `yaml:"gap_level,omitempty"`
}

// Rules is the parsed config/rules.yaml for a book project.
type Rules struct {
	Language  string         `yaml:"language,omitempty"`
	WordCount WordCountRules `yaml:"word_count"`
	Research  ResearchRules  `yaml:"research,omitempty"`
	QA        QARules        `yaml:"qa,omitempty"`
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
	if r.Research.MaxQueriesPerChapter == 0 {
		r.Research.MaxQueriesPerChapter = 5
	}
	if r.Research.MaxResultsPerQuery == 0 {
		r.Research.MaxResultsPerQuery = 3
	}
	if r.QA.ConflictLevel == "" {
		r.QA.ConflictLevel = "light"
	}
	if r.QA.GapLevel == "" {
		r.QA.GapLevel = "light"
	}
	validateRules(r)
	return r, nil
}

// validLevels are the accepted values for gap_level and conflict_level.
var validLevels = map[string]bool{
	"off": true, "light": true, "moderate": true, "max": true,
}

// validateRules corrects invalid enum values in-place and logs warnings.
func validateRules(r *Rules) {
	if r.QA.ConflictLevel != "" && !validLevels[r.QA.ConflictLevel] {
		log.Warn("rules.yaml: invalid conflict_level, defaulting to light",
			"value", r.QA.ConflictLevel)
		r.QA.ConflictLevel = "light"
	}
	if r.QA.GapLevel != "" && !validLevels[r.QA.GapLevel] {
		log.Warn("rules.yaml: invalid gap_level, defaulting to light",
			"value", r.QA.GapLevel)
		r.QA.GapLevel = "light"
	}
}
