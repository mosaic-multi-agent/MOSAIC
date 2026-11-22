---
id: quick-fix
version: "2.1"
name: Quick Fix Workflow
description: Fixture workflow that applies a small fix and runs the tests.
hint: Fixture hint for the quick fix workflow.
referenced_agents:
  - code-writer
  - test-runner
---

# Quick Fix Workflow

<Workflow type="core" name="quick-fix">
| Row | Input | Agent | Output |
|-----|-------|-------|--------|
| 1 | Requirements.md | code-writer | code |
| 2 | code | test-runner | results |
</Workflow>
