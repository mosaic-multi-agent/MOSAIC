---
mosaic_id: 55
mosaic_version: 2.3.0
mosaic_harness_version: 1.2.0
mosaic_role: subagent
description: Looks up tracker issues and pull requests
mode: subagent
model: anthropic/claude-sonnet-4-5
permissions:
  - action: "*"
    resource: "*"
    effect: deny
  - action: read
    resource: "*"
    effect: allow
  - action: contact-user*
    resource: "*"
    effect: allow
  - action: tracker_get_issue
    resource: "*"
    effect: allow
  - action: azure-devops_get_pull_request
    resource: "*"
    effect: allow
  - action: external_directory
    resource: "*"
    effect: ask
---

# McpAgent

<Identity type="core">
You are the **McpAgent** agent.
</Identity>
