package context

import (
	"context"
	"fmt"
)

// BraidType classifies how two context strands relate at a given paragraph.
type BraidType string

const (
	BraidAgreement  BraidType = "AGREEMENT"  // strands reach the same conclusion
	BraidConflict   BraidType = "CONFLICT"   // strands contradict each other
	BraidResonance  BraidType = "RESONANCE"  // strands amplify each other
	BraidSilence    BraidType = "SILENCE"    // one strand has nothing to say
)

// BraidPoint records where two strands interact in a paragraph.
type BraidPoint struct {
	// Type classifies the interaction.
	Type BraidType
	// StrandA is the name of the first context stack.
	StrandA string
	// StrandB is the name of the second context stack.
	StrandB string
	// Paragraph is the 0-based paragraph index where the braid point occurs.
	Paragraph int
	// Note is a brief explanation of the braid point.
	Note string
}

// BraidedField holds multiple context strands and their braid points.
type BraidedField struct {
	Strands     []ContextStack
	BraidPoints []BraidPoint
}

// Weave analyzes a paragraph and identifies where the strands interact.
// This is a structural operation — it classifies existing braid points rather
// than generating new content.
func (b *BraidedField) Weave(_ context.Context, paragraph string, paraIdx int) ([]BraidPoint, error) {
	if len(b.Strands) < 2 {
		return nil, nil
	}

	var points []BraidPoint
	for i := 0; i < len(b.Strands)-1; i++ {
		for j := i + 1; j < len(b.Strands); j++ {
			a := &b.Strands[i]
			bStrand := &b.Strands[j]

			pt := classifyBraidPoint(paragraph, a, bStrand, paraIdx)
			if pt != nil {
				points = append(points, *pt)
			}
		}
	}

	b.BraidPoints = append(b.BraidPoints, points...)
	return points, nil
}

// classifyBraidPoint applies heuristic rules to classify how two stacks
// interact with a paragraph. This is a lightweight structural analysis
// without LLM calls — the detailed content analysis happens in processor.go.
func classifyBraidPoint(paragraph string, a, b *ContextStack, paraIdx int) *BraidPoint {
	// Check if either strand has nothing to say (SILENCE)
	aEmpty := a.PromptContext() == ""
	bEmpty := b.PromptContext() == ""

	if aEmpty && bEmpty {
		return nil // both silent — no braid point
	}
	if aEmpty || bEmpty {
		name := a.Name
		if aEmpty {
			name = b.Name
		}
		return &BraidPoint{
			Type:      BraidSilence,
			StrandA:   a.Name,
			StrandB:   b.Name,
			Paragraph: paraIdx,
			Note:      fmt.Sprintf("strand %q has no content for this paragraph", name),
		}
	}

	// Default: mark as RESONANCE when both strands contribute
	// Deeper classification (AGREEMENT vs CONFLICT) requires LLM analysis
	return &BraidPoint{
		Type:      BraidResonance,
		StrandA:   a.Name,
		StrandB:   b.Name,
		Paragraph: paraIdx,
		Note:      "both strands active — run processor for detailed classification",
	}
}
