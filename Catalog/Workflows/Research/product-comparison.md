---
version: "2.0"
name: "Product Comparison Workflow"
description: "Compares two or more existing product codebases across a chosen set of dimensions — each product researched independently, then synthesized into a side-by-side comparison."
hint: "Easy to use for any comparison: the planner agrees products and dimensions with you, and only the chosen dimensions run. Adding a brand-new dimension takes a research agent, a group row in this workflow and an entry in comparison-planner's dimension list."
author: MOSAIC
id: product-comparison
referenced_agents:
  - comparison-planner
  - product-research
  - end-user-experience-research
  - maintainability-research
  - extensibility-research
  - design-quality-research
  - quality-mechanisms-research
  - human-oversight-research
  - cost-research
  - security-research
  - performance-research
  - topic-analyst
  - comparison-analyst
  - comparison-review
artifacts:
  - Requirements.md
  - ComparisonPlan.md
  - "{Product}-ProductMap.md"
  - "*-ProductMap.md"
  - "{Research}-Findings.md"
  - "*-{Topic}-Findings.md"
  - "{Topic}Comparison.md"
  - "*Comparison.md"
  - ComparisonAnalysis.md
  - comparison-review.md
---

<Workflow type="core" name="product-comparison" version="2.0">
## Product Comparison Workflow

**Use when:** Comparing two or more **existing product codebases** that solve a similar problem. Each product is researched independently across the chosen dimensions, then synthesized into a single side-by-side comparison.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| PLANNING | comparison-planner | TRUE | [Product] | - | Requirements.md | ComparisonPlan.md |
| RESEARCH.[Product] | product-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md | {Product}-ProductMap.md |
| RESEARCH.[Research].EndUserExperience | end-user-experience-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].Maintainability | maintainability-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].Extensibility | extensibility-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].DesignQuality | design-quality-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].QualityMechanisms | quality-mechanisms-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].HumanOversight | human-oversight-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].Cost | cost-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].Security | security-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| RESEARCH.[Research].Performance | performance-research | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-ProductMap.md | {Research}-Findings.md |
| EXECUTION.[Topic] | topic-analyst | FALSE | COMPLETE | - | Requirements.md, ComparisonPlan.md, *-{Topic}-Findings.md | {Topic}Comparison.md |
| EXECUTION | comparison-analyst | FALSE | NEXT | - | Requirements.md, ComparisonPlan.md, *Comparison.md | ComparisonAnalysis.md |
| REVIEW | comparison-review | FALSE | COMPLETE | comparison-analyst | Requirements.md, ComparisonPlan.md, ComparisonAnalysis.md, *Comparison.md | comparison-review.md |

**Blocks:**

| Block | Source | Section |
|-------|--------|---------|
| Product | ComparisonPlan.md | Products |
| Research | ComparisonPlan.md | Research |
| Topic | ComparisonPlan.md | Topics |

**Groups:**

| Block | Approach | Groups |
|-------|----------|--------|
| Research | EndUserExperience | EndUserExperience |
| Research | Maintainability | Maintainability |
| Research | Extensibility | Extensibility |
| Research | DesignQuality | DesignQuality |
| Research | QualityMechanisms | QualityMechanisms |
| Research | HumanOversight | HumanOversight |
| Research | Cost | Cost |
| Research | Security | Security |
| Research | Performance | Performance |
</Workflow>

---

## Design Rationale

This workflow was designed for and successfully tested on comparing multi-agent orchestration frameworks — two complex software products analyzed across nine quality dimensions. The three-tier architecture (findings then per-topic comparison then synthesis) emerged from the insight that fair comparison requires strict separation between evidence gathering and evaluation. Tier 1 agents (`product-research` and the dimension research agents) are verdict-free — they document *what is* for a single product, with no quality judgments. Evaluation starts at tier 2 (`topic-analyst`).

**Seed.** `Requirements.md` is user-written free text: the products (name and repo path) and the comparison criteria. The analysts read the criteria from it directly.

**Why a planner (2.0).** Version 1.0 had none — the product list came straight from Requirements.md and all nine dimensions always ran, so a planner added "zero decision value". WorkflowFormat 2.x changed that: products, product × dimension research and topics are block iterations, and each block reads its iterations from a schedule table. `comparison-planner` turns the user's free text into those tables, and because it runs with HITL it also lets the user choose which dimensions are worth running for this comparison — the planner lists the available dimensions, proposes the ones that fit the stated goals, and writes what the user agrees to.

**Three flat blocks instead of a fork.** In 1.0, each product's `product-research` forked into the nine dimension agents. A block row cannot fork (B1), and nesting a dimension block inside a product block is not yet supported by the orchestrators. Instead the planner writes one `Research` iteration per product × dimension (`Acme-Security`), and each iteration runs exactly one group — the dimension's research agent, chosen by `Approach`. All products × dimensions still run in parallel, as before, with three ordinary blocks in sequence: `[Product]` → `[Research]` → `[Topic]`.

**Artifact names.** Each research iteration reads every product map (`*-ProductMap.md`) — a row in the Research block cannot fill in a Product ID — and takes its own product from the `Product` column of its Research row. The maps are short, so the extra reading is small. Findings are `{Research}-Findings.md` (`Acme-Security-Findings.md`), so each `topic-analyst` iteration finds its dimension across all products as `*-{Topic}-Findings.md`. The names are chosen so no wildcard Input can match another block's per-iteration artifact.

**Why separate dimension research agents instead of one generic dimension-research agent:** Each dimension agent carries dimension-specific guidance (what to look for, where evidence typically lives, illustrative investigation angles) baked into its instructions. A single generic agent would need this guidance injected per-invocation, which the current architecture doesn't support — injections are set at deploy time, not at dispatch time. The trade-off is more agents in the catalog; the gain is that each dimension agent knows its domain cold without runtime parameterization.

**Why topic-analyst is generic (one agent, one iteration per dimension) while dimension research agents are specialized:** Different design pressures. A dimension research agent reads raw code and needs dimension-specific guidance on what to look for. A topic-analyst reads structured findings artifacts that already share a common format — it compares products within a dimension, and the comparison discipline is identical regardless of dimension. Specializing it would multiply agents with no quality gain.

**Adding a dimension** takes three edits that must agree on one token: a new research agent, a group row (plus its approach-table row) in this workflow, and an entry in `comparison-planner`'s dimension list.

**Fix targets.** Only `comparison-review` has one (`comparison-analyst`). Missing or weak tier-1 findings surface as `NEEDS_CLARIFICATION` from the analysts; where that work goes is the orchestrator's decision.

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-08-20 | MOSAIC | Initial catalog version — extracted and converted from the old orchestrator-embedded workflow that was successfully tested on multi-agent framework comparison |
| 2.0 | 2026-10-10 | MOSAIC | Migrated to WorkflowFormat 2.16 and restructured. New `comparison-planner` row (HITL) writes `ComparisonPlan.md` with three block schedules — `Products`, `Research` (product × dimension) and `Topics` — agreed with the user, who now chooses the dimensions per run. The per-product fork into nine research rows (not allowed inside a block, B1) became the `[Research]` block with one group per dimension selected by `Approach`; `{Product}`/`{Topic}` fill-ins now come from declared blocks. Artifacts renamed (`{Product}-ProductMap.md`, `{Research}-Findings.md`) so wildcards never cross blocks. `HITL` `-` → `FALSE` (T7). Invalid fix targets removed (`{Topic}-research`, `topic-analyst` across scopes). Notes removed — redundant with the tables or moved to Design Rationale. |

---

## Open Ideas / Dead Ends

Capture ideas that were explored but not adopted, and future improvements worth considering. This prevents the same dead ends from being revisited unknowingly.

**Ideas under consideration:**
- **HITL on comparison-review or comparison-analyst:** The workflow runs autonomously after planning. For high-stakes comparisons, adding HITL on `comparison-review` or `comparison-analyst` would let a human validate the synthesis before completion. Not added because the first successful test ran without HITL and the review gate caught issues adequately.
- **Redoing a completed topic.** A completed Topic iteration cannot be re-run under its own ID (WorkflowFormat §9.3), and a new ID such as `Security-2` would no longer match the findings pattern `*-{Topic}-Findings.md`. Rare enough to leave to the orchestrator for now; solve if it shows up in practice.
- **Nested `[Product].[Dimension]` blocks.** Would let each research iteration read only its own product's map (`{Product}-ProductMap.md`). Revisit once orchestrators support nested blocks.

**Dead ends (tried and rejected):**
- **Configurable dimension sets via conditional rows** (1.0 idea) — superseded in 2.0: the planner's `Research` and `Topics` schedules select the dimensions per run, with no conditional-row support needed.
