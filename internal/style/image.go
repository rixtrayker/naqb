// Package style provides StyleImage: a multi-dimensional representation of an
// author's writing style, with extraction, application, blending, and registry.
package style

// LinguisticProfile captures low-level linguistic characteristics.
type LinguisticProfile struct {
	// AvgSentenceLength is mean words per sentence.
	AvgSentenceLength float64 `yaml:"avg_sentence_length,omitempty"`
	// VocabularyRichness is type-token ratio (unique/total words).
	VocabularyRichness float64 `yaml:"vocabulary_richness,omitempty"`
	// PunctuationDensity is punctuation marks per 100 words.
	PunctuationDensity float64 `yaml:"punctuation_density,omitempty"`
	// ClauseNestingDepth is average syntactic clause depth.
	ClauseNestingDepth float64 `yaml:"clause_nesting_depth,omitempty"`
}

// StructuralProfile captures document-level structure patterns.
type StructuralProfile struct {
	// AvgParagraphLength is mean sentences per paragraph.
	AvgParagraphLength float64 `yaml:"avg_paragraph_length,omitempty"`
	// SectionDensity is headings per 1000 words.
	SectionDensity float64 `yaml:"section_density,omitempty"`
	// TransitionFrequency is transitional phrases per paragraph.
	TransitionFrequency float64 `yaml:"transition_frequency,omitempty"`
	// ListUsage is bullet/numbered list frequency (0=never, 1=frequent).
	ListUsage float64 `yaml:"list_usage,omitempty"`
}

// RhetoricalProfile captures argumentation and evidence patterns.
type RhetoricalProfile struct {
	// ArgumentationMode: "deductive", "inductive", "abductive", "dialectical".
	ArgumentationMode string `yaml:"argumentation_mode,omitempty"`
	// EvidenceTypes is a frequency map: "citation", "example", "analogy", etc.
	EvidenceTypes map[string]float64 `yaml:"evidence_types,omitempty"`
	// HedgingFrequency is hedging words per 100 words ("perhaps", "may", "likely").
	HedgingFrequency float64 `yaml:"hedging_frequency,omitempty"`
	// AssertionStrength: "tentative", "moderate", "confident", "declarative".
	AssertionStrength string `yaml:"assertion_strength,omitempty"`
}

// VoiceProfile captures the authorial voice and register.
type VoiceProfile struct {
	// Register: "formal", "semi-formal", "colloquial".
	Register string `yaml:"register,omitempty"`
	// Formality is a 0–1 score (0=most informal, 1=most formal).
	Formality float64 `yaml:"formality,omitempty"`
	// Persona: "scholarly", "journalistic", "pedagogical", "poetic".
	Persona string `yaml:"persona,omitempty"`
	// DirectAddressFrequency is second-person pronouns per 1000 words.
	DirectAddressFrequency float64 `yaml:"direct_address_frequency,omitempty"`
}

// ArabicProfile captures Arabic-specific stylistic patterns.
type ArabicProfile struct {
	// DiacriticalPolicy: "full", "partial", "minimal", "none".
	DiacriticalPolicy string `yaml:"diacritical_policy,omitempty"`
	// RegisterBoundary: "classical", "modern-standard", "mixed".
	RegisterBoundary string `yaml:"register_boundary,omitempty"`
	// BalaghaDensity is rhetorical figures (badīʿ) per 100 words.
	BalaghaDensity float64 `yaml:"balagha_density,omitempty"`
	// IsnadFrequency is hadith citation chains per chapter.
	IsnadFrequency float64 `yaml:"isnad_frequency,omitempty"`
	// SajFrequency is rhymed-prose (sajʿ) passages per page.
	SajFrequency float64 `yaml:"saj_frequency,omitempty"`
}

// StyleImage is a complete multi-dimensional style profile for an author or corpus.
// It is serialized to YAML for persistence and diffing.
type StyleImage struct {
	// ID is a unique identifier (typically a short slug).
	ID string `yaml:"id"`
	// Name is the human-readable style name.
	Name string `yaml:"name"`
	// Author is the source author or corpus.
	Author string `yaml:"author,omitempty"`
	// Version tracks schema evolution.
	Version string `yaml:"version,omitempty"`

	Linguistic  LinguisticProfile  `yaml:"linguistic"`
	Structural  StructuralProfile  `yaml:"structural"`
	Rhetorical  RhetoricalProfile  `yaml:"rhetorical"`
	Voice       VoiceProfile       `yaml:"voice"`
	Arabic      ArabicProfile      `yaml:"arabic,omitempty"`
}
