package context

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/amr/naqb/pkg/llm"
)

// StrandResult holds the output from running a single context strand.
type StrandResult struct {
	// StackName is the strand that produced this result.
	StackName string
	// Analysis is the LLM-generated analytical commentary.
	Analysis string
	// Error holds any error that occurred during processing.
	Error error
}

// RunStrands processes a text input through each context stack's lenses in
// parallel, then runs a synthesis pass if multiple strands are active.
// Returns a map of stack name → analytical output.
func RunStrands(ctx context.Context, field *BraidedField, input string, client llm.Provider) (map[string]string, error) {
	if field == nil || len(field.Strands) == 0 {
		return nil, nil
	}

	results := make(chan StrandResult, len(field.Strands))
	var wg sync.WaitGroup

	for _, strand := range field.Strands {
		wg.Add(1)
		go func(s ContextStack) {
			defer wg.Done()
			analysis, err := runSingleStrand(ctx, &s, input, client)
			results <- StrandResult{
				StackName: s.Name,
				Analysis:  analysis,
				Error:     err,
			}
		}(strand)
	}

	wg.Wait()
	close(results)

	outputs := make(map[string]string, len(field.Strands))
	var firstErr error
	for r := range results {
		if r.Error != nil && firstErr == nil {
			firstErr = r.Error
		}
		if r.Analysis != "" {
			outputs[r.StackName] = r.Analysis
		}
	}

	if len(outputs) == 0 && firstErr != nil {
		return nil, firstErr
	}

	// Synthesis pass when multiple strands produce output
	if len(outputs) > 1 {
		synthesis, err := runSynthesis(ctx, outputs, input, client)
		if err == nil && synthesis != "" {
			outputs["_synthesis"] = synthesis
		}
	}

	return outputs, nil
}

// runSingleStrand processes a text through one context stack's layers.
func runSingleStrand(ctx context.Context, stack *ContextStack, input string, client llm.Provider) (string, error) {
	if client == nil {
		return fmt.Sprintf("[%s: no LLM client available]", stack.Name), nil
	}

	systemPrompt := stack.PromptContext()
	if systemPrompt == "" {
		return "", nil // strand is silent for this input
	}

	prompt := fmt.Sprintf("Analyze the following passage through the lens of %s:\n\n%s", stack.Name, input)

	msg := llm.Message{Role: "user", Content: prompt}
	var sb strings.Builder
	_, err := client.Stream(ctx, "", systemPrompt, []llm.Message{msg}, 2048, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("strand %s: LLM error: %w", stack.Name, err)
	}
	return sb.String(), nil
}

// runSynthesis produces a unified synthesis of multiple strand outputs.
func runSynthesis(ctx context.Context, outputs map[string]string, original string, client llm.Provider) (string, error) {
	if client == nil {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("You have multiple analytical perspectives on the following passage. Synthesize them into a unified scholarly commentary.\n\n")
	sb.WriteString("Original passage:\n")
	sb.WriteString(original)
	sb.WriteString("\n\nPerspectives:\n")
	for name, analysis := range outputs {
		sb.WriteString("### ")
		sb.WriteString(name)
		sb.WriteString("\n")
		sb.WriteString(analysis)
		sb.WriteString("\n\n")
	}

	msg := llm.Message{Role: "user", Content: sb.String()}
	var result strings.Builder
	_, err := client.Stream(ctx, "", "You are a scholarly synthesis engine.", []llm.Message{msg}, 2048, func(delta string) error {
		result.WriteString(delta)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("synthesis: LLM error: %w", err)
	}
	return result.String(), nil
}
