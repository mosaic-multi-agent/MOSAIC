---
version: "1.0"
name: "Alpha Pair Workflow"
description: "Runs the tests for the alpha sample."
hint: "Sample workflow for the orchestrator pair fixtures."
author: MOSAIC
id: pair-flow-alpha
referenced_agents:
  - parity-runner
artifacts:
  - TestResults.md
---

<Workflow type="core" name="pair-flow-alpha" version="1.0">
## Alpha Pair Workflow

**Use when:** Running the tests for the alpha sample.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| REVIEW | parity-runner | FALSE | COMPLETE | - | - | TestResults.md |
</Workflow>

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-10 | MOSAIC | Sample workflow. |
