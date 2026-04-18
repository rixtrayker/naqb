package style

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// StyleDiff describes the differences between two StyleImages.
type StyleDiff struct {
	AName string
	BName string
	// Changes maps field path → (A value, B value)
	Changes map[string][2]string
}

// Blend creates a new StyleImage by interpolating between a and b.
// weight=0.0 → pure A, weight=1.0 → pure B.
func Blend(a, b *StyleImage, weight float64) *StyleImage {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	w := weight
	wA := 1 - w

	result := &StyleImage{
		ID:      fmt.Sprintf("blend-%s-%s-%.2f", a.ID, b.ID, weight),
		Name:    fmt.Sprintf("Blend: %s × %.0f%% + %s × %.0f%%", a.Name, wA*100, b.Name, w*100),
		Version: "1.0",
	}

	// Blend numeric fields
	result.Linguistic = LinguisticProfile{
		AvgSentenceLength:  lerp(a.Linguistic.AvgSentenceLength, b.Linguistic.AvgSentenceLength, w),
		VocabularyRichness: lerp(a.Linguistic.VocabularyRichness, b.Linguistic.VocabularyRichness, w),
		PunctuationDensity: lerp(a.Linguistic.PunctuationDensity, b.Linguistic.PunctuationDensity, w),
		ClauseNestingDepth: lerp(a.Linguistic.ClauseNestingDepth, b.Linguistic.ClauseNestingDepth, w),
	}
	result.Structural = StructuralProfile{
		AvgParagraphLength:  lerp(a.Structural.AvgParagraphLength, b.Structural.AvgParagraphLength, w),
		SectionDensity:      lerp(a.Structural.SectionDensity, b.Structural.SectionDensity, w),
		TransitionFrequency: lerp(a.Structural.TransitionFrequency, b.Structural.TransitionFrequency, w),
		ListUsage:           lerp(a.Structural.ListUsage, b.Structural.ListUsage, w),
	}
	result.Voice = VoiceProfile{
		Formality:              lerp(a.Voice.Formality, b.Voice.Formality, w),
		DirectAddressFrequency: lerp(a.Voice.DirectAddressFrequency, b.Voice.DirectAddressFrequency, w),
	}
	result.Arabic = ArabicProfile{
		BalaghaDensity: lerp(a.Arabic.BalaghaDensity, b.Arabic.BalaghaDensity, w),
		IsnadFrequency: lerp(a.Arabic.IsnadFrequency, b.Arabic.IsnadFrequency, w),
		SajFrequency:   lerp(a.Arabic.SajFrequency, b.Arabic.SajFrequency, w),
	}

	// Categorical fields: pick A when weight < 0.5, B otherwise
	if w < 0.5 {
		result.Rhetorical.ArgumentationMode = a.Rhetorical.ArgumentationMode
		result.Rhetorical.AssertionStrength = a.Rhetorical.AssertionStrength
		result.Voice.Register = a.Voice.Register
		result.Voice.Persona = a.Voice.Persona
		result.Arabic.DiacriticalPolicy = a.Arabic.DiacriticalPolicy
		result.Arabic.RegisterBoundary = a.Arabic.RegisterBoundary
	} else {
		result.Rhetorical.ArgumentationMode = b.Rhetorical.ArgumentationMode
		result.Rhetorical.AssertionStrength = b.Rhetorical.AssertionStrength
		result.Voice.Register = b.Voice.Register
		result.Voice.Persona = b.Voice.Persona
		result.Arabic.DiacriticalPolicy = b.Arabic.DiacriticalPolicy
		result.Arabic.RegisterBoundary = b.Arabic.RegisterBoundary
	}

	return result
}

// Fork creates a copy of a StyleImage with a new ID and name.
func Fork(base *StyleImage) *StyleImage {
	copy := *base
	copy.ID = base.ID + "-fork"
	copy.Name = base.Name + " (fork)"
	// Deep copy maps
	if base.Rhetorical.EvidenceTypes != nil {
		copy.Rhetorical.EvidenceTypes = make(map[string]float64, len(base.Rhetorical.EvidenceTypes))
		for k, v := range base.Rhetorical.EvidenceTypes {
			copy.Rhetorical.EvidenceTypes[k] = v
		}
	}
	return &copy
}

// Diff computes the differences between two StyleImages.
func Diff(a, b *StyleImage) StyleDiff {
	diff := StyleDiff{
		AName:   a.Name,
		BName:   b.Name,
		Changes: make(map[string][2]string),
	}

	// Compare numeric fields
	compareFloat := func(field string, av, bv float64) {
		if av != bv {
			diff.Changes[field] = [2]string{
				fmt.Sprintf("%.3f", av),
				fmt.Sprintf("%.3f", bv),
			}
		}
	}
	compareStr := func(field, av, bv string) {
		if av != bv {
			diff.Changes[field] = [2]string{av, bv}
		}
	}

	compareFloat("linguistic.avg_sentence_length", a.Linguistic.AvgSentenceLength, b.Linguistic.AvgSentenceLength)
	compareFloat("linguistic.vocabulary_richness", a.Linguistic.VocabularyRichness, b.Linguistic.VocabularyRichness)
	compareFloat("linguistic.punctuation_density", a.Linguistic.PunctuationDensity, b.Linguistic.PunctuationDensity)
	compareFloat("structural.avg_paragraph_length", a.Structural.AvgParagraphLength, b.Structural.AvgParagraphLength)
	compareFloat("structural.section_density", a.Structural.SectionDensity, b.Structural.SectionDensity)
	compareFloat("voice.formality", a.Voice.Formality, b.Voice.Formality)
	compareStr("rhetorical.argumentation_mode", a.Rhetorical.ArgumentationMode, b.Rhetorical.ArgumentationMode)
	compareStr("rhetorical.assertion_strength", a.Rhetorical.AssertionStrength, b.Rhetorical.AssertionStrength)
	compareStr("voice.register", a.Voice.Register, b.Voice.Register)
	compareStr("voice.persona", a.Voice.Persona, b.Voice.Persona)
	compareStr("arabic.diacritical_policy", a.Arabic.DiacriticalPolicy, b.Arabic.DiacriticalPolicy)
	compareStr("arabic.register_boundary", a.Arabic.RegisterBoundary, b.Arabic.RegisterBoundary)

	return diff
}

// Fingerprint returns a deterministic SHA-256 hash of the StyleImage.
// Images with the same fingerprint have identical style profiles.
func Fingerprint(img *StyleImage) string {
	// Use a canonical JSON representation (marshal is deterministic for flat structs)
	canonical := struct {
		Linguistic  LinguisticProfile  `json:"linguistic"`
		Structural  StructuralProfile  `json:"structural"`
		Rhetorical  RhetoricalProfile  `json:"rhetorical"`
		Voice       VoiceProfile       `json:"voice"`
		Arabic      ArabicProfile      `json:"arabic"`
	}{
		Linguistic: img.Linguistic,
		Structural: img.Structural,
		Rhetorical: img.Rhetorical,
		Voice:      img.Voice,
		Arabic:     img.Arabic,
	}
	data, _ := json.Marshal(canonical)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16])
}

// lerp linearly interpolates between a and b at weight w (0=a, 1=b).
func lerp(a, b, w float64) float64 {
	return a*(1-w) + b*w
}
