---
id: feature-build
version: "1.0"
name: Feature Build Workflow
description: Fixture workflow that plans, implements and tests a feature.
hint: Fixture hint for the feature build workflow.
referenced_agents:
  - plan-designer
  - code-writer
  - test-runner
---

# Feature Build Workflow

Maintainer prose outside the section block.

<Workflow type="core" name="feature-build">
| Row | Input | Agent | Output |
|-----|-------|-------|--------|
| 1 | Requirements.md | plan-designer | Plan.md |
| 2 | Plan.md | code-writer | code |
| 3 | code | test-runner | results |
</Workflow>
