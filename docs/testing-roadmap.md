# Testing Roadmap

This document tracks test coverage progress across the `pkg/*` modules and the root module.

## Coverage Dashboard

| Package | Coverage | Status |
|---------|----------|--------|
| `pkg/runtime` | **87.8%** | ✅ Excellent |
| `pkg/wordcount` | **84.8%** | ✅ Excellent |
| `pkg/agent` | **70.6%** | ✅ Good |
| `pkg/pipeline` | **52.9%** | ✅ Decent |
| `pkg/search` | **53.2%** | ✅ Decent |
| `pkg/youtube` | **46.4%** | ✅ Decent |
| `pkg/agents` | **40.7%** | 🟡 Moderate |
| `pkg/config` | **26.6%** | 🔴 Low |
| `pkg/booktools` | **16.5%** | 🔴 Low |
| `pkg/llm` | **11.8%** | 🔴 Low |
| `pkg/research` | **0.0%** | ⚫ None |
| `pkg/log` | **0.0%** | ⚫ None |

*Last updated: 2026-04-15*

## Completed Work

### Phase 1 — Reflection Pipeline Fix
- Fixed `pkg/pipeline/reflection_test.go` mock provider exhaustion
- Added `defaultResponse` fallback to multi-loop mock providers

### Phase 2 — Booktools Plan/Execute
- **File**: `pkg/booktools/plan_execute_test.go` (new)
- `TestPlanAndExecuteTool_Success` — full plan→execute flow with 2 tools
- `TestPlanAndExecuteTool_UnknownTool` — unregistered tool handling
- `TestPlanNode_Resumption` — pre-populated steps skip LLM call
- `TestPlanAndExecuteTool_ToolError` — tool.Invoke returns error
- `TestPlanAndExecuteTool_MalformedPlan` — non-JSON plan response
- `TestSerializeDeserializePlanState` — JSON round-trip
- `TestStripMarkdownFencesPlan` — markdown fence edge cases
- **Bug fix**: `stripMarkdownFencesPlan` now handles ` ```json ` and trailing text

### Phase 3 — Agent Streaming & Tool-Use
- **File**: `pkg/agent/agent_run_test.go` (new)
- `TestAgent_Run_Streaming` — `onDelta` callback + `RunResult.Output` assembly
- `TestAgent_Run_SessionPersistence` — `SessionStore` records CreateSession/AppendMessage/TouchSession
- `TestAgent_Run_TokenCounting` — `Usage` propagation to TokensIn/TokensOut/Steps
- `TestAgent_Run_WithTool` — agent runs with tools registered
- `TestAgent_Run_NilProvider` / `TestAgent_Run_ProviderError` — error paths

### Phase 4 — Agents Conflict & Gap Analysis
- **File**: `pkg/agents/conflict_test.go` (new)
- `TestRunConflictCheck_OffLevel` / `NoPreceding` — early return paths
- `TestRunConflictCheck_WithMockLLM` — clean run, `HasIssues=false`
- `TestRunConflictCheck_FindsIssues` — detects contradiction keywords
- `TestRunConflictCheck_LLMError` — wrapped error propagation
- `TestLooksLikeConflictHeuristic` — keyword matching logic
- **File**: `pkg/agents/gap_test.go` (new)
- `TestRunGapAnalysis_OffLevel` / `NoOutlineFile` — early/fallback paths
- `TestRunGapAnalysis_WithMockLLM` — clean run, `HasGaps=false`
- `TestRunGapAnalysis_FindsGaps` — detects gap keywords
- `TestRunGapAnalysis_LLMError` — wrapped error propagation
- `TestLooksLikeGapHeuristic` — keyword matching logic

### Phase 5 — Runtime Core
- **File**: `pkg/runtime/graph_test.go` (new)
- `Invoke`: linear flow, conditional edges, conditional exit, single node
- Error handling: node error, unknown node, max steps guard (infinite loop)
- Checkpoint integration: save/load, resume from checkpoint, skip load
- Callback lifecycle: OnNodeStart/OnNodeEnd, error callbacks
- `InvokeParallel`: linear DAG, concurrent batch, cycle detection, node error
- `topologicalBatches`: simple chain, fan-out/fan-in, cycle detection
- Edge cases: duplicate node overwrite, edge to empty string
- **File**: `pkg/runtime/registry_test.go` (new) — ToolRegistry register/resolve/list
- **File**: `pkg/runtime/interrupt_test.go` (new) — InterruptedError formatting, IsInterrupted

### Phase 6 — Pipeline Core
- **File**: `pkg/pipeline/registry_test.go` (new) — StageRegistry builtins
- **File**: `pkg/pipeline/template_test.go` (new) — builtin templates, file loading, invalid YAML/DAG
- **File**: `pkg/pipeline/debt_test.go` (new) — ContextDebt budget tracking, violations, summaries
- **File**: `pkg/pipeline/state_test.go` (new) — PipelineState.toInput, containsString, stageNode with mock stages (run, skip, GateAlways blocks/passes, GateAuto triggered/passes, error)
- **File**: `pkg/pipeline/dag_planner_test.go` (new) — RunDAGPlanner with mock LLM, fence stripping
- **Bug fix**: `stripMarkdownFences` in `dag_planner.go` now handles ` ```json ` and trailing text

## Remaining Priorities

### Priority 7 — `pkg/agents` Writer + Planner + Fixer
**Goal**: Bring coverage from 40.7% to ~60%+

These are the **core writing tools** users interact with directly. All require mock `llm.Provider` (pattern established in conflict/gap tests).

**Files to test**:
- `pkg/agents/writer.go` (~2.7 KB) — `WriteChapter`, chapter generation prompts
- `pkg/agents/planner.go` (~5.5 KB) — `PlanBook`, outline generation, chapter breakdowns
- `pkg/agents/fixer.go` (~11.6 KB) — `FixChapter`, rewrite based on QA feedback
- `pkg/agents/analysis.go` (~2.4 KB) — `AnalyzeProject`, directory scanning
- `pkg/agents/classifier.go` (~2.2 KB) — complexity classification
- `pkg/agents/model_selector.go` (~3.1 KB) — `ModelFor`, budget-aware model selection

**Test cases**:
- `TestWriteChapter_Success` — mock LLM returns chapter text, file written
- `TestWriteChapter_WithContext` — context file included in prompt
- `TestPlanBook_Success` — mock LLM returns outline JSON
- `TestPlanBook_MalformedJSON` — non-JSON response handling
- `TestFixChapter_Success` — mock LLM returns fixed chapter
- `TestAnalyzeProject` — scan book directory, count chapters/words
- `TestModelFor_BudgetDegraded` — fallback to cheaper model when budget exceeded

### Priority 8 — `pkg/config`
**Goal**: Bring coverage from 26.6% to ~70%+

Foundation layer — every module reads config.

**Files to test**:
- `pkg/config/book.go` — `LoadBookConfig`, `BookConfig` validation
- `pkg/config/global.go` — `LoadGlobalConfig`, env var overrides
- `pkg/config/rules.go` — `LoadRules`, default rule application

**Test cases**:
- `TestLoadBookConfig_Valid` — parse valid `book.yaml`
- `TestLoadBookConfig_Missing` — missing file returns error
- `TestLoadBookConfig_InvalidYAML` — malformed YAML
- `TestLoadRules_Defaults` — missing rules.yaml uses defaults (min=1500)
- `TestLoadRules_Custom` — custom rules.yaml overrides defaults

### Priority 9 — `pkg/llm`
**Goal**: Bring coverage from 11.8% to ~50%+

Operational layer — provider wrappers, token budget, cost estimation.

**Files to test**:
- `pkg/llm/provider.go` — Provider interface, provider constructors
- `pkg/llm/budget.go` — `SessionBudget`, `Degraded()`, `Record()`
- `pkg/llm/cost.go` — cost estimation per model

**Test cases**:
- `TestSessionBudget_Record` — accumulate tokens, check degraded
- `TestSessionBudget_Degraded` — threshold crossing
- `TestEstimateCost` — per-model pricing
- `TestProvider_OpenRouter` — constructor with valid/invalid API keys

### Priority 10 — Research + Log (Optional)
- `pkg/research` (0%) — research pipeline (Scout → Explorer → Scribe)
- `pkg/log` (0%) — structured logging wrapper

These are lower priority unless research pipeline becomes a primary user path.

## Running Coverage

```bash
# All pkg modules
for d in pkg/*/; do
  echo "=== $(basename $d) ==="
  (cd "$d" && go test -cover ./...)
done

# Single package
cd pkg/agent && go test -cover ./...

# With HTML report
cd pkg/runtime && go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

## Notes

- All `pkg/*` modules are standalone Go modules with their own `go.mod`
- `go.work` enables local cross-module development
- Mock providers follow the pattern: `responses []string` + `idx int` + optional `defaultResponse`
- The `fantasy.Provider` mock requires `TextStart`/`TextEnd` stream parts for the fantasy agent to assemble `AgentResult.Response.Content`
