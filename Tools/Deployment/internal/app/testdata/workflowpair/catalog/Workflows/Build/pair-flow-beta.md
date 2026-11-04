---
version: "1.0"
name: "Beta Pair Workflow"
description: "Runs the tests for the beta sample."
hint: "Sample workflow for the orchestrator pair fixtures."
author: MOSAIC
id: pair-flow-beta
referenced_agents:
  - parity-runner
artifacts:
  - TestResults.md
---

<Workflow type="core" name="pair-flow-beta" version="1.0">
## Beta Pair Workflow

**Use when:** Running the tests for the beta sample.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| REVIEW | parity-runner | FALSE | COMPLETE | - | - | TestResults.md |
</Workflow>

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-10 | MOSAIC | Sample workflow. |
