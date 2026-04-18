// Package arabic provides standard ContextLayer definitions for classical Arabic
// text analysis. These cover the primary analytical lenses used in Islamic and
// Arabic scholarly tradition.
package arabic

import "github.com/amr/naqb/internal/context"

// Standard Arabic analytical layer names.
const (
	LayerIsnadChain       = "isnād-chain"
	LayerManuscriptCensus = "manuscript-census"
	LayerDiacriticalVar   = "diacritical-variants"
	LayerClassicalGrammar = "classical-grammar-rules"
	LayerEtymological     = "etymological-field"
	LayerBalagha          = "balāgha"
	LayerIjaza            = "ijāza-chain"
	LayerSemanticShift    = "semantic-shift"
	LayerAllusion         = "allusion-recognition"
	LayerMaqamat          = "maqāmāt"
	LayerMorphology       = "arabic-morphology"
)

// StandardLayers returns the set of standard Arabic analytical layers.
// Each layer is provided with a description; Content must be populated
// by the user with project-specific material.
func StandardLayers() []context.ContextLayer {
	return []context.ContextLayer{
		{
			Position:    0,
			Name:        LayerIsnadChain,
			Description: "Analysis of hadith transmission chains (isnād): narrator reliability, continuity, and rijāl criticism.",
			Language:    "ar",
		},
		{
			Position:    0,
			Name:        LayerManuscriptCensus,
			Description: "Survey of manuscript witnesses, variant readings, and their geographical and temporal distribution.",
			Language:    "ar",
		},
		{
			Position:    1,
			Name:        LayerDiacriticalVar,
			Description: "Cataloguing of diacritical variants (tashkeel) and their implications for meaning and exegesis.",
			Language:    "ar",
		},
		{
			Position:    1,
			Name:        LayerClassicalGrammar,
			Description: "Application of classical Arabic grammar (naḥw and ṣarf) rules to parse ambiguous constructions.",
			Language:    "ar",
		},
		{
			Position:    2,
			Name:        LayerEtymological,
			Description: "Tracing the etymological field of key terms: roots, derivations, and semantic evolution.",
			Language:    "ar",
		},
		{
			Position:    2,
			Name:        LayerBalagha,
			Description: "Rhetorical analysis using classical Arabic balāgha: maʿānī, bayān, and badīʿ categories.",
			Language:    "ar",
		},
		{
			Position:    3,
			Name:        LayerIjaza,
			Description: "Documentation of ijāza transmission chains linking the text to authoritative sources.",
			Language:    "ar",
		},
		{
			Position:    3,
			Name:        LayerSemanticShift,
			Description: "Identification of semantic shifts in key terms across historical periods and textual communities.",
			Language:    "ar",
		},
		{
			Position:    4,
			Name:        LayerAllusion,
			Description: "Recognition of intertextual allusions: Quranic citations, hadith echoes, classical poetry references.",
			Language:    "ar",
		},
		{
			Position:    4,
			Name:        LayerMaqamat,
			Description: "Analysis of maqāma genre conventions, rhymed prose structures, and rhetorical game-playing.",
			Language:    "ar",
		},
		{
			Position:    5,
			Name:        LayerMorphology,
			Description: "Morphological analysis using classical Arabic derivational patterns (awzān) and root-meaning networks.",
			Language:    "ar",
		},
	}
}

// NewArabicStack creates a ContextStack with all standard Arabic layers.
// Use the returned stack as a base and populate layer Content fields with
// project-specific material.
func NewArabicStack(name string) *context.ContextStack {
	return &context.ContextStack{
		Name:    name,
		Version: "1.0",
		Layers:  StandardLayers(),
	}
}

// LayerByName returns the standard layer definition for a given layer name.
// Returns nil if not found.
func LayerByName(name string) *context.ContextLayer {
	for _, layer := range StandardLayers() {
		if layer.Name == name {
			l := layer // copy
			return &l
		}
	}
	return nil
}
