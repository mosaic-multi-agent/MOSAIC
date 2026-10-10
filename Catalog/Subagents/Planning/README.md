# Planning

Work breakdown, sequencing, and architecture agents.

## Purpose

Planning agents define WHAT work to do, WHEN to do it, and HOW it should be structured. They create plans and designs that guide downstream implementation - they do not implement.

## Agents

| ID | Agent | Version | Description |
|----|-------|---------|-------------|
| 6 | [planner-tdd-soft](./planner-tdd-soft.md) | 7.4.2 | Creates implementation plans with task breakdown, sequencing, and dependencies |
| 7 | [planner-audit](./planner-audit.md) | 5.3.1 | Creates audit plans splitting changed files into typed stages (Implementation, Tests, Architecture, Contracts), always including whole-PR Architecture and Contracts stages, for parallel auditing |
| 8 | [contracts-designer](./contracts-designer.md) | 4.3.1 | Defines interfaces, contracts, data structures, and component specifications |
| 5 | [system-designer](./system-designer.md) | 3.2.2 | Designs system architecture, component interactions, and high-level structure |
| 4 | [requirements-refinement](./requirements-refinement.md) | 3.4.1 | Refines requirements into typed specifications at a user-declared depth through user dialogue |
| 24 | [knowledge-base-flag-sorter](./knowledge-base-flag-sorter.md) | 3.3.1 | Collects correction flags from KBFlags.md, organizes bottom-up by tier, writes the Corrections schedule in KBProgress.md |
| 31 | [hw-schema-planner](./hw-schema-planner.md) | 3.3.1 | Discovers all sheets of a hardware schematic project and writes HWResearchProgress.md with one research stage per sheet |
| 26 | [verification-questions-preparer](./verification-questions-preparer.md) | 2.3.1 | Creates, populates, and validates Q/A verification artifacts and prepares batched attempted-answer prompts |
| 33 | [pr-requirements-analyzer](./pr-requirements-analyzer.md) | 1.0.0 | Analyzes PR context — fetches changed file list and stats, summarizes existing comment threads, confirms audit scope with user, enriches Requirements.md with PR metadata |
| 44 | [test-scenario-designer](./test-scenario-designer.md) | 1.1.0 | Enumerates the complete space of test scenarios a resolved requirement implies, as an explicit structured model with justified exclusions |
| 66 | [comparison-planner](./comparison-planner.md) | 1.0.1 | Turns the user's product list and comparison goals into the Products, Research and Topics block schedules of ComparisonPlan.md, agreed with the user |

## Planning vs Design Distinction

| Aspect | Planner | ContractsDesigner | SystemDesigner |
|--------|---------|-------------------|----------------|
| Focus | WHAT and WHEN | HOW (detailed) | HOW (high-level) |
| Output | Plan.md, Stage-{N}/Plan.md, Stage-{N}/PlanProgress.md | ContractsDesign.md | SystemDesign.md |
| Defines | Tasks, stages, sequencing | Interfaces, contracts, schemas | Architecture, components, interactions |
| Example | "Stage 1: Create auth service" | "IAuthService.login(LoginRequest) → AuthResult" | "Auth service communicates with User service via REST" |

## What Planning Agents Do

- Analyze validated requirements and research findings
- Break down work into discrete, implementable tasks
- Define task sequencing and dependencies
- Identify milestones and checkpoints
- Define interfaces, contracts, and data structures
- Document architectural decisions with rationale

## What Planning Agents Do NOT Do

- Gather requirements (that's Research)
- Validate requirements (that's Validation)
- Write implementation code or tests (that's Creation)
- Execute tests (that's Execution)
