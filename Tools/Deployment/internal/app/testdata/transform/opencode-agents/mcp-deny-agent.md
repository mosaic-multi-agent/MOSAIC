---
mosaic_id: 56
mosaic_version: 2.3.0
mosaic_harness_version: 1.2.0
mosaic_role: subagent
description: Denies an ambiguous MCP name
mode: subagent
model: anthropic/claude-sonnet-4-5
permission:
  read: allow
  contact-user*: allow
  denied_get_item: deny
---

# McpDenyAgent

<Identity type="core">
You are the **McpDenyAgent** agent.
</Identity>
