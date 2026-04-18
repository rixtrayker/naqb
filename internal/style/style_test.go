package style

import (
	"testing"
)

func TestExtractLinguistic(t *testing.T) {
	texts := []string{
		"This is a sentence. This is another sentence. And a third one!",
		"More text here. With some variety.",
	}
	p := extractLinguistic(texts)
	if p.AvgSentenceLength <= 0 {
		t.Error("expected positive avg sentence length")
	}
	if p.VocabularyRichness <= 0 {
		t.Error("expected positive vocabulary richness")
	}
}

func TestExtractStructural(t *testing.T) {
	texts := []string{
		"## Heading\n\nParagraph one. Sentence two.\n\nParagraph two. Another sentence.",
	}
	p := extractStructural(texts)
	if p.SectionDensity <= 0 {
		t.Error("expected positive section density (heading present)")
	}
}

func TestBlend_Midpoint(t *testing.T) {
	a := &StyleImage{
		ID:   "a",
		Name: "A",
		Linguistic: LinguisticProfile{AvgSentenceLength: 10, VocabularyRichness: 0.4},
		Voice:      VoiceProfile{Formality: 0.2},
	}
	b := &StyleImage{
		ID:   "b",
		Name: "B",
		Linguistic: LinguisticProfile{AvgSentenceLength: 20, VocabularyRichness: 0.8},
		Voice:      VoiceProfile{Formality: 0.8},
	}
	result := Blend(a, b, 0.5)
	if result.Linguistic.AvgSentenceLength != 15.0 {
		t.Errorf("expected 15.0, got %f", result.Linguistic.AvgSentenceLength)
	}
	if result.Voice.Formality != 0.5 {
		t.Errorf("expected formality 0.5, got %f", result.Voice.Formality)
	}
}

func TestBlend_PureA(t *testing.T) {
	a := &StyleImage{ID: "a", Linguistic: LinguisticProfile{AvgSentenceLength: 5}}
	b := &StyleImage{ID: "b", Linguistic: LinguisticProfile{AvgSentenceLength: 15}}
	result := Blend(a, b, 0.0)
	if result.Linguistic.AvgSentenceLength != 5.0 {
		t.Errorf("weight=0 should return pure A: %f", result.Linguistic.AvgSentenceLength)
	}
}

func TestBlend_PureB(t *testing.T) {
	a := &StyleImage{ID: "a", Linguistic: LinguisticProfile{AvgSentenceLength: 5}}
	b := &StyleImage{ID: "b", Linguistic: LinguisticProfile{AvgSentenceLength: 15}}
	result := Blend(a, b, 1.0)
	if result.Linguistic.AvgSentenceLength != 15.0 {
		t.Errorf("weight=1 should return pure B: %f", result.Linguistic.AvgSentenceLength)
	}
}

func TestFork(t *testing.T) {
	base := &StyleImage{
		ID:   "original",
		Name: "Original",
		Linguistic: LinguisticProfile{AvgSentenceLength: 12},
	}
	forked := Fork(base)
	if forked.ID == base.ID {
		t.Error("fork should have different ID")
	}
	if forked.Linguistic.AvgSentenceLength != base.Linguistic.AvgSentenceLength {
		t.Error("fork should preserve linguistic profile")
	}
}

func TestDiff(t *testing.T) {
	a := &StyleImage{
		ID:   "a",
		Name: "A",
		Voice: VoiceProfile{Register: "formal"},
		Linguistic: LinguisticProfile{AvgSentenceLength: 10},
	}
	b := &StyleImage{
		ID:   "b",
		Name: "B",
		Voice: VoiceProfile{Register: "colloquial"},
		Linguistic: LinguisticProfile{AvgSentenceLength: 20},
	}
	diff := Diff(a, b)
	if diff.Changes["voice.register"][0] != "formal" {
		t.Errorf("expected voice.register A='formal', got %v", diff.Changes["voice.register"])
	}
	if _, ok := diff.Changes["linguistic.avg_sentence_length"]; !ok {
		t.Error("expected sentence length in diff")
	}
}

func TestDiff_NoDiff(t *testing.T) {
	a := &StyleImage{ID: "a", Linguistic: LinguisticProfile{AvgSentenceLength: 10}}
	b := &StyleImage{ID: "b", Linguistic: LinguisticProfile{AvgSentenceLength: 10}}
	diff := Diff(a, b)
	if len(diff.Changes) != 0 {
		t.Errorf("expected no changes for identical profiles, got %v", diff.Changes)
	}
}

func TestFingerprint(t *testing.T) {
	a := &StyleImage{ID: "x", Linguistic: LinguisticProfile{AvgSentenceLength: 10}}
	b := &StyleImage{ID: "y", Linguistic: LinguisticProfile{AvgSentenceLength: 10}}
	if Fingerprint(a) != Fingerprint(b) {
		t.Error("identical profiles should produce identical fingerprints")
	}

	c := &StyleImage{ID: "z", Linguistic: LinguisticProfile{AvgSentenceLength: 99}}
	if Fingerprint(a) == Fingerprint(c) {
		t.Error("different profiles should produce different fingerprints")
	}
}

func TestStyleConstraints(t *testing.T) {
	img := &StyleImage{
		Author: "Ibn Khaldun",
		Name:   "Muqaddimah style",
		Voice:  VoiceProfile{Register: "formal", Persona: "scholarly"},
		Linguistic: LinguisticProfile{AvgSentenceLength: 25},
	}
	constraints := StyleConstraints(img)
	if constraints == "" {
		t.Error("expected non-empty constraints")
	}
	if !contains(constraints, "Ibn Khaldun") {
		t.Error("expected author name in constraints")
	}
}

func TestRegistrySaveAndGet(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	img := &StyleImage{
		ID:   "test-style",
		Name: "Test Style",
		Linguistic: LinguisticProfile{AvgSentenceLength: 15},
	}
	if err := reg.Save(img); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := reg.Get("test-style")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Name != "Test Style" {
		t.Errorf("name mismatch: %q", loaded.Name)
	}
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewRegistry(dir)
	for _, id := range []string{"s1", "s2", "s3"} {
		_ = reg.Save(&StyleImage{ID: id, Name: id})
	}
	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 styles, got %d", len(list))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
