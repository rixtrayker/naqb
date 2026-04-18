package style

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/llm"
)

// ApplyMode controls how a style is applied to content.
type ApplyMode int

const (
	// PromptMode injects style constraints into a compose prompt.
	// The style guidance is prepended to the chapter-writing system prompt.
	PromptMode ApplyMode = iota
	// PostprocessMode runs a separate LLM rewrite pass on existing content.
	PostprocessMode
)

// StyleConstraints serializes a StyleImage into natural-language style guidelines
// suitable for injection into an LLM system prompt.
func StyleConstraints(img *StyleImage) string {
	if img == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Style Guidelines\n\n")

	if img.Author != "" {
		sb.WriteString(fmt.Sprintf("Write in the style of **%s** (%s).\n\n", img.Author, img.Name))
	}

	// Voice
	if img.Voice.Register != "" {
		sb.WriteString(fmt.Sprintf("- Register: %s\n", img.Voice.Register))
	}
	if img.Voice.Persona != "" {
		sb.WriteString(fmt.Sprintf("- Authorial persona: %s\n", img.Voice.Persona))
	}

	// Linguistic
	if img.Linguistic.AvgSentenceLength > 0 {
		sb.WriteString(fmt.Sprintf("- Target sentence length: ~%.0f words per sentence\n", img.Linguistic.AvgSentenceLength))
	}
	if img.Linguistic.VocabularyRichness > 0.5 {
		sb.WriteString("- Use rich, varied vocabulary\n")
	} else if img.Linguistic.VocabularyRichness > 0 {
		sb.WriteString("- Prefer clear, accessible vocabulary\n")
	}

	// Rhetorical
	if img.Rhetorical.ArgumentationMode != "" {
		sb.WriteString(fmt.Sprintf("- Argumentation style: %s\n", img.Rhetorical.ArgumentationMode))
	}
	if img.Rhetorical.AssertionStrength != "" {
		sb.WriteString(fmt.Sprintf("- Assertion strength: %s\n", img.Rhetorical.AssertionStrength))
	}

	// Arabic
	if img.Arabic.DiacriticalPolicy != "" && img.Arabic.DiacriticalPolicy != "none" {
		sb.WriteString(fmt.Sprintf("- Arabic diacritical policy: %s tashkeel\n", img.Arabic.DiacriticalPolicy))
	}
	if img.Arabic.RegisterBoundary != "" {
		sb.WriteString(fmt.Sprintf("- Arabic register: %s\n", img.Arabic.RegisterBoundary))
	}

	return sb.String()
}

// Apply applies a StyleImage to content according to the given mode.
// PromptMode: returns the style constraints as a string to prepend to a prompt.
// PostprocessMode: calls the LLM to rewrite the content in the target style.
func Apply(ctx context.Context, img *StyleImage, content string, mode ApplyMode, client llm.Provider) (string, error) {
	switch mode {
	case PromptMode:
		return StyleConstraints(img), nil
	case PostprocessMode:
		return applyPostprocess(ctx, img, content, client)
	default:
		return "", fmt.Errorf("style apply: unknown mode %d", mode)
	}
}

// applyPostprocess rewrites content through the LLM to match the target style.
func applyPostprocess(ctx context.Context, img *StyleImage, content string, client llm.Provider) (string, error) {
	if client == nil {
		return content, fmt.Errorf("style apply: LLM client required for PostprocessMode")
	}

	systemPrompt := "You are a style editor. Rewrite the provided text to match the given style guidelines exactly. Preserve all factual content and meaning — change only the style.\n\n" + StyleConstraints(img)
	userMsg := "Rewrite the following text to match the style guidelines:\n\n" + content
	msg := llm.Message{Role: "user", Content: userMsg}

	var sb strings.Builder
	_, err := client.Stream(ctx, "", systemPrompt, []llm.Message{msg}, 8192, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		return content, fmt.Errorf("style apply postprocess: LLM: %w", err)
	}
	return sb.String(), nil
}
