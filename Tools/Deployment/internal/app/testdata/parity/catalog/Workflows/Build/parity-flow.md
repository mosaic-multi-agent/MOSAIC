---
version: "1.0"
name: "Parity Flow Workflow"
description: "Runs the tests once."
hint: "Sample workflow for the parity fixtures."
author: MOSAIC
id: parity-flow
referenced_agents:
  - parity-runner
artifacts:
  - TestResults.md
---

<Workflow type="core" name="parity-flow" version="1.0">
## Parity Flow Workflow

**Use when:** Running the tests once.

| Phase | Subagent | HITL | On Success | On Findings | Input | Output |
|-------|----------|:----:|------------|-------------|-------|--------|
| REVIEW | parity-runner | FALSE | COMPLETE | - | - | TestResults.md |
</Workflow>

---

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.0 | 2026-10-10 | MOSAIC | Sample workflow. |
