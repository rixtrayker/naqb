package style

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/llm"
)

// Extract analyzes a set of texts and produces a StyleImage representing
// their shared stylistic characteristics. Requires an LLM for qualitative
// analysis (rhetorical mode, voice, Arabic profile).
func Extract(ctx context.Context, texts []string, client llm.Provider, id, name, author string) (*StyleImage, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("style extract: at least one text required")
	}

	// Deterministic analysis (no LLM)
	img := &StyleImage{
		ID:      id,
		Name:    name,
		Author:  author,
		Version: "1.0",
	}
	img.Linguistic = extractLinguistic(texts)
	img.Structural = extractStructural(texts)

	// LLM analysis for qualitative profiles
	if client != nil {
		sample := buildSample(texts, 3000)
		if err := extractQualitative(ctx, client, sample, img); err != nil {
			// Non-fatal: return what we have
			_ = err
		}
	}

	return img, nil
}

// extractLinguistic computes linguistic statistics from texts.
func extractLinguistic(texts []string) LinguisticProfile {
	var totalWords, totalSentences, totalPunct int
	uniqueWords := make(map[string]struct{})

	for _, text := range texts {
		words := strings.Fields(text)
		totalWords += len(words)
		for _, w := range words {
			w = strings.Trim(strings.ToLower(w), ".,;:!?\"'()")
			if w != "" {
				uniqueWords[w] = struct{}{}
			}
		}
		// Count sentences (crude: split on ., !, ?, ؟)
		for _, r := range text {
			if r == '.' || r == '!' || r == '?' || r == '؟' {
				totalSentences++
			}
			if r == ',' || r == ';' || r == ':' || r == '،' || r == '؛' {
				totalPunct++
			}
		}
	}

	if totalWords == 0 {
		return LinguisticProfile{}
	}

	avgSentLen := 0.0
	if totalSentences > 0 {
		avgSentLen = float64(totalWords) / float64(totalSentences)
	}

	return LinguisticProfile{
		AvgSentenceLength:  avgSentLen,
		VocabularyRichness: float64(len(uniqueWords)) / float64(totalWords),
		PunctuationDensity: float64(totalPunct) / float64(totalWords) * 100,
	}
}

// extractStructural computes structural statistics from texts.
func extractStructural(texts []string) StructuralProfile {
	var totalParas, totalSentences, totalHeadings, totalWords int

	for _, text := range texts {
		lines := strings.Split(text, "\n")
		currentParaSentences := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				if currentParaSentences > 0 {
					totalParas++
					totalSentences += currentParaSentences
					currentParaSentences = 0
				}
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				totalHeadings++
				continue
			}
			for _, r := range trimmed {
				if r == '.' || r == '!' || r == '?' || r == '؟' {
					currentParaSentences++
				}
			}
		}
		if currentParaSentences > 0 {
			totalParas++
			totalSentences += currentParaSentences
		}
		totalWords += len(strings.Fields(text))
	}

	avgParaLen := 0.0
	if totalParas > 0 {
		avgParaLen = float64(totalSentences) / float64(totalParas)
	}
	sectionDensity := 0.0
	if totalWords > 0 {
		sectionDensity = float64(totalHeadings) / float64(totalWords) * 1000
	}

	return StructuralProfile{
		AvgParagraphLength: avgParaLen,
		SectionDensity:     sectionDensity,
	}
}

// buildSample creates a representative sample from multiple texts.
func buildSample(texts []string, maxRunes int) string {
	var sb strings.Builder
	perText := maxRunes / len(texts)
	for _, t := range texts {
		runes := []rune(t)
		if len(runes) > perText {
			runes = runes[:perText]
		}
		sb.WriteString(string(runes))
		sb.WriteString("\n\n---\n\n")
	}
	return sb.String()
}

// styleAnalysisJSON is the JSON schema the LLM returns for qualitative analysis.
type styleAnalysisJSON struct {
	ArgumentationMode string `json:"argumentation_mode"`
	AssertionStrength string `json:"assertion_strength"`
	Register          string `json:"register"`
	Persona           string `json:"persona"`
	DiacriticalPolicy string `json:"diacritical_policy"`
	RegisterBoundary  string `json:"register_boundary"`
}

// extractQualitative calls the LLM to fill qualitative profile fields.
func extractQualitative(ctx context.Context, client llm.Provider, sample string, img *StyleImage) error {
	systemPrompt := `You are a linguistic style analyst. Analyze the given text and return ONLY a JSON object with these exact fields:
{
  "argumentation_mode": "deductive|inductive|abductive|dialectical",
  "assertion_strength": "tentative|moderate|confident|declarative",
  "register": "formal|semi-formal|colloquial",
  "persona": "scholarly|journalistic|pedagogical|poetic",
  "diacritical_policy": "full|partial|minimal|none",
  "register_boundary": "classical|modern-standard|mixed"
}
Return ONLY the JSON, no explanation.`

	userMsg := "Analyze this text's style:\n\n" + sample
	msg := llm.Message{Role: "user", Content: userMsg}

	var sb strings.Builder
	_, err := client.Stream(ctx, "", systemPrompt, []llm.Message{msg}, 512, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		return fmt.Errorf("style extract: LLM: %w", err)
	}

	// Extract JSON from response
	raw := sb.String()
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < 0 || end <= start {
		return fmt.Errorf("style extract: no JSON in LLM response")
	}

	var analysis styleAnalysisJSON
	if err := json.Unmarshal([]byte(raw[start:end+1]), &analysis); err != nil {
		return fmt.Errorf("style extract: parse JSON: %w", err)
	}

	img.Rhetorical.ArgumentationMode = analysis.ArgumentationMode
	img.Rhetorical.AssertionStrength = analysis.AssertionStrength
	img.Voice.Register = analysis.Register
	img.Voice.Persona = analysis.Persona
	if analysis.DiacriticalPolicy != "" {
		img.Arabic.DiacriticalPolicy = analysis.DiacriticalPolicy
	}
	if analysis.RegisterBoundary != "" {
		img.Arabic.RegisterBoundary = analysis.RegisterBoundary
	}

	return nil
}
