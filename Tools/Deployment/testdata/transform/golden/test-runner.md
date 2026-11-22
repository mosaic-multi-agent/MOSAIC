---
mosaic_id: 2
mosaic_version: 1.0.0
mosaic_harness_version: 3.0.0
description: Tool-heavy fixture for golden file tests — exercises all seven generic tools including terminal
mode: subagent
model: claude/claude-sonnet
tools:
  - read-file
  - write-file
  - edit-file
  - search-file
  - search-text
  - run-terminal
  - ask-user
required_skills: []
mosaic_role: subagent
---

<Identity type="core">
# TestRunner Agent (Test Fixture)

This is a frozen test fixture. It exercises the tool-heavy transform profile:
all seven generic tools are declared, including `terminal`. The output must list all
corresponding harness tools in universe order.

This file intentionally carries minimal body content. Its purpose is to exercise the
transform engine's tool-mapping logic, not to document agent behaviour.
</Identity>
---

<CommunicationProtocol type="managed" version="1.9">
## Communication Protocol (Subagent)

Subagent-specific protocol content.
</CommunicationProtocol>
