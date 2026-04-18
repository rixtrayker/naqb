package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/pkg/agents"
	"github.com/amr/naqb/pkg/llm"
	"github.com/amr/naqb/pkg/log"
	"gopkg.in/yaml.v3"
)

// DAGPlanResult holds a generated pipeline template.
type DAGPlanResult struct {
	Template *Template
	RawYAML  string
}

// RunDAGPlanner asks an LLM to generate a custom pipeline DAG for a given goal.
func RunDAGPlanner(ctx context.Context, client llm.Provider, goal string) (*DAGPlanResult, error) {
	log.Info("dag planner start", "goal", goal)

	systemPrompt := `You are an expert workflow planner for a book-writing AI system.
Given a user goal, generate a pipeline template as YAML.

Available stage types:
- CONTEXT: assemble context file for a chapter
- WRITE: write the chapter draft
- QA: quality assurance checks
- CONFLICT: cross-chapter conflict detection
- GAP: outline coverage gap analysis
- RESEARCH: research pipeline (Scout → Explorer → Scribe)
- SYNTHESIZE: synthesize research notes into outline

Rules:
1. Respond with ONLY valid YAML (no markdown fences, no explanations).
2. The root must have keys: name, description, stages.
3. Each stage must have: id, type, depends_on (list of ids).
4. Optional per-stage: model (overrides default model) and human_gate (NONE, AUTO, ALWAYS).
5. CONFLICT and GAP stages may use depends_on: [write] to run in parallel after WRITE.
6. Keep the DAG acyclic.

Example:
name: deep-dive
description: Research-heavy chapter with full checks
stages:
  - id: research
    type: RESEARCH
    depends_on: []
  - id: context
    type: CONTEXT
    depends_on: [research]
  - id: write
    type: WRITE
    depends_on: [context]
  - id: qa
    type: QA
    depends_on: [write]
  - id: conflict
    type: CONFLICT
    depends_on: [write]
  - id: gap
    type: GAP
    depends_on: [write]
`

	userMsg := fmt.Sprintf("Goal: %s\n\nGenerate the optimal pipeline DAG as YAML.", goal)

	response, err := client.Complete(ctx, agents.ModelFor(agents.StagePlan, nil), systemPrompt, []llm.Message{
		{Role: "user", Content: userMsg},
	}, llm.TokensPlan)
	if err != nil {
		return nil, fmt.Errorf("dag planner LLM failed: %w", err)
	}

	raw := strings.TrimSpace(response)
	// Strip markdown fences if the LLM ignored instructions
	raw = stripMarkdownFences(raw)

	var tmpl Template
	if err := yaml.Unmarshal([]byte(raw), &tmpl); err != nil {
		return nil, fmt.Errorf("dag planner parse YAML: %w", err)
	}
	if tmpl.Name == "" {
		tmpl.Name = "generated"
	}

	// Validate DAG shape
	if _, err := tmpl.ToDAG(); err != nil {
		return nil, fmt.Errorf("dag planner invalid DAG: %w", err)
	}

	log.Info("dag planner done", "name", tmpl.Name, "stages", len(tmpl.Stages))
	return &DAGPlanResult{Template: &tmpl, RawYAML: raw}, nil
}

func stripMarkdownFences(s string) string {
	s = strings.TrimPrefix(s, "```yaml")
	s = strings.TrimPrefix(s, "```yml")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
