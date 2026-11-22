---
id: design-audit
version: "1.3"
name: Design Audit Workflow
description: Fixture workflow that reviews an interface design.
hint: Fixture hint for the design audit workflow.
referenced_agents:
  - api-reviewer
---

# Design Audit Workflow

<Workflow type="core" name="design-audit">
| Row | Input | Agent | Output |
|-----|-------|-------|--------|
| 1 | Design.md | api-reviewer | Review.md |
</Workflow>
