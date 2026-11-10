---
mosaic_id: 55
mosaic_version: 2.3.0
mosaic_harness_version: 0.9.0
mosaic_bundle_version: 1.5.0
mosaic_role: subagent
name: code-reviewer
description: Reviews source code for quality
model: claude-sonnet-4-5
tools: Read, Grep, Bash
---

<Identity type="core">
# CodeReviewer Agent

BODY-IDENTITY-TEXT

<AuthorityHierarchy type="managed">
CARRIED-AUTHORITY-TEXT
</AuthorityHierarchy>

<ClosingProcedure type="managed">
CARRIED-CLOSING-TEXT
</ClosingProcedure>

<AvailableWorkflows type="managed">
<Workflow type="managed" name="demo-flow" version="4.0">
## Demo Flow

| Row | Phase | Subagent |
|-----|-------|----------|
| 1 | WORK | code-reviewer |
</Workflow>
</AvailableWorkflows>

<InfrastructureAgents type="managed">
<InfrastructureAgent type="core" name="commit-manager-git" version="2.5.0">

| Class | Trigger | Param | On Failure | Description |
|-------|---------|-------|------------|-------------|
| commit | ITERATION_END | - | continue | Commits completed work |

</InfrastructureAgent>
</InfrastructureAgents>

<TeamNotes type="custom">
CUSTOM-REGION-TEXT
</TeamNotes>

</Identity>

<CommunicationProtocol type="managed" version="0.9">
CARRIED-PROTOCOL-TEXT

<ProtocolLocalRule type="custom">
NESTED-CUSTOM-IN-PROTOCOL
</ProtocolLocalRule>

</CommunicationProtocol>

<Capabilities type="core">
## Capabilities

<CodebaseContext type="project">
PROJECT-REGION-TEXT
</CodebaseContext>

</Capabilities>

<Constraints type="core">
## Constraints

<HarnessConstraints type="managed" version="0.5.0">
SOURCE-HARNESS-CONSTRAINT-TEXT
</HarnessConstraints>

</Constraints>

<ErrorHandling type="core">
## Error Handling

<ErrorHandlingCommon type="managed">
CARRIED-ERROR-COMMON-TEXT
</ErrorHandlingCommon>

</ErrorHandling>

<ExecutionPhilosophy type="core">
## Execution Philosophy

<ExecutionPhilosophyCommon type="managed">
CARRIED-PHILOSOPHY-TEXT
</ExecutionPhilosophyCommon>

</ExecutionPhilosophy>
