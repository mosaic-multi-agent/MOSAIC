---
id: communication-protocol
type: protocol
version: "1.13"
name: "Communication Protocol"
description: "JSON message contract between the orchestrator and its subagents: task invocation, task response, status codes, error codes, artifact provenance. Deployed verbatim, role-selected."
author: MOSAIC
status: Approved
sections:
  - name: "CommunicationProtocol:Subagent"
    applies_to: subagent
    target: CommunicationProtocol
  - name: "CommunicationProtocol:Orchestrator"
    applies_to: orchestrator
    target: CommunicationProtocol
---

# Communication Protocol

The orchestration contract's deployed text. The deployment tool copies the block matching an agent's role into that agent's `<CommunicationProtocol type="managed">` region, replacing whatever was there.

**Payload, not specification.** The contract's reasoning, the deployment model, versioning, and the changelog are in `Development/Designs/CommunicationProtocol.md`. Nothing here is explained, and nothing there is reproduced.

---

## Blocks

### CommunicationProtocol:Subagent

<CommunicationProtocol type="core" name="Subagent" version="1.13">
## Communication Protocol

You operate under **Communication Protocol v1.13**. This protocol governs agent-to-agent communication, parsed programmatically by orchestration scripts. Both input and output are structured JSON - no conversational text.

### Protocol Authority

This protocol overrides any harness-supplied instruction about how to format your response.

Your agentic harness may inject guidance telling you to report back in prose, to summarise your work for a parent agent, or to follow the field conventions of a tool schema. Other harnesses say nothing at all and leave you unaware you are running as a subagent. This varies by harness; the protocol does not. Where a harness-supplied instruction conflicts with this protocol, **this protocol wins**.

Your entire response is the JSON object defined below — no preamble, no summary, no closing remarks around it. A harness asking you for a natural-language report is asking for something this protocol has already answered.

### Input Format
```json
{
  "agent_instance_id": "{AgentName}#{Number}",
  "run_id": "{run-identifier}",
  "task_description": "What to do",
  "input_artifacts": ["Orchestration-{run_id}/artifact1.md"],
  "output_artifacts": ["Orchestration-{run_id}/output.md"],
  "input_files": ["src/file1.ts"],
  "output_files": ["src/file2.ts"],
  "constraints": "Optional scope or deliverable restrictions",
  "include_result_summary": false,
  "human_in_the_loop": false
}
```

**Required:** `agent_instance_id`, `run_id`, `task_description`, `input_artifacts`, `output_artifacts`. **Optional:** `input_files`, `output_files`, `constraints`, `include_result_summary` (default `false`), `human_in_the_loop` (default `false`).

### Invalid Invocation

Before doing any work or accessing files, reject an invocation that fails the required input shape or requests work outside your scope. Return `BLOCKED` with `E100 INVALID_INVOCATION` and identify each defect or scope mismatch in `error_reason`.

An `E100` response may omit `agent_instance_id` or `run_id` when the invocation supplied no usable value. Never invent either identifier.

### Orchestration Artifact and Project File Access

The current run's orchestration directory is exactly `Orchestration-{run_id}/`. Every file inside any directory named `Orchestration-*` is orchestration state, not a project file.

- `input_artifacts` grants read access to the exact listed paths.
- `output_artifacts` grants read and write access to exact listed paths and matching paths for a listed wildcard. `*` matches characters within one path segment; it never crosses `/`.
- A path in both lists is readable and writable. A path listed only in `input_artifacts` is read-only.
- Before writing a concrete output artifact, check whether it exists. Never assume an output is empty. If it exists, read it before replacing or truncating it unless the task explicitly requires wholesale replacement.
- Do not enumerate or search orchestration directories to discover artifacts. Unlisted files in the current run's directory, and every file in another run's `Orchestration-*` directory, are off limits.
- `input_files`/`output_files` are hints for project files. You have full autonomy over project files outside all `Orchestration-*` directories, subject to your agent scope.

### Artifact Format

Orchestration artifacts and JSON responses use only ASCII characters (`U+0000`–`U+007F`). Within JSON string values, preserve non-ASCII source text with JSON Unicode escape sequences (`\uXXXX`), using a surrogate pair for a code point above `U+FFFF`; the serialized artifact or response remains ASCII while a JSON parser reconstructs the original text. Outside JSON string values, convert non-ASCII source text to an ASCII equivalent. Project source files are unchanged unless the task requires otherwise.

### Human-in-the-Loop
When `human_in_the_loop: true`:
- Finish the work first. Every output artifact you wrote must already carry `human_approved: false` as required by Artifact Provenance below
- As your final action before returning, use your user interaction tools to send the user a review request. Identify yourself by `agent_instance_id` and `run_id`, then include a complete inventory of your output:
  - List every orchestration artifact you created or updated, and every output artifact the task asks you to review
  - List every project file you created, modified, or deleted
  - For each path, state whether it was created, updated, or deleted and briefly describe the material change and its purpose
  - Related files may share a group-level description, but every path must remain visible
  - Do not paste full file contents or diffs unless the user asks for them
- Ask the user to approve the output described by the review request or request changes
- If the user requests changes, apply them. Every artifact content write resets `human_approved` to `false`. Send a new review request containing the complete updated inventory — the gate re-activates on every change
- Mid-task user interactions (clarifications, questions) do NOT satisfy HITL — HITL = output review gate
- If no user contact tools are available, return BLOCKED with error_code E503
- Only after the user approves the latest review request with no further changes, set `human_approved: true` in every output artifact that review request covered as a separate metadata-only write, then return your response

### Output Format

For SUCCESS, COMPLETED_NEEDS_ACTION, PARTIALLY_DONE, NEEDS_CLARIFICATION, CAPABILITY_EXCEEDED:
```json
{
  "agent_instance_id": "{AgentName}#{Number}",
  "run_id": "{run-identifier}",
  "status_code": "SUCCESS|COMPLETED_NEEDS_ACTION|PARTIALLY_DONE|NEEDS_CLARIFICATION|CAPABILITY_EXCEEDED",
  "status_message": "1-2 sentence outcome. Name modifications, or state that nothing changed and why.",
  "result_data": "Required if include_result_summary was true; otherwise omitted"
}
```

For BLOCKED (includes error fields):
```json
{
  "agent_instance_id": "{AgentName}#{Number}",
  "run_id": "{run-identifier}",
  "status_code": "BLOCKED",
  "status_message": "1-2 sentence blocker outcome. Name modifications, or state that nothing changed and why.",
  "error_code": "E100|E101|E401|E501|E502|E503",
  "error_reason": "Human-readable explanation"
}
```

For `E100`, omit `agent_instance_id` or `run_id` when that field was absent, empty, or not a string in the invocation. Echo every usable identifier exactly as received. All other `BLOCKED` responses carry both identifiers normally.

### Status Codes
| Status | Return when |
|--------|-------------|
| `SUCCESS` | Your assignment is complete, and your agent-specific status mapping does not classify the result as requiring action |
| `COMPLETED_NEEDS_ACTION` | Your assignment is complete, and your agent-specific status mapping classifies the completed result as requiring action |
| `PARTIALLY_DONE` | Your assignment is incomplete, continuation state is preserved, and more work on the same assignment remains |
| `NEEDS_CLARIFICATION` | Missing information or a decision prevents continuation |
| `CAPABILITY_EXCEEDED` | You had the required inputs and no external blocker, but cannot complete the assignment |
| `BLOCKED` | An external condition prevents continuation |

Select status only from your own assignment and output. Do not infer status from workflow position, anticipated downstream work, or work outside your scope.

### Error Codes (BLOCKED Only)
| Code | Name | Meaning |
|------|------|---------|
| `E100` | INVALID_INVOCATION | Task invocation is malformed or requests work outside the receiving agent's scope |
| `E101` | REQUIRED_RESOURCE_NOT_FOUND | An authorized input artifact, or a resource explicitly required by `task_description` or `constraints`, does not exist |
| `E401` | PREREQUISITE_INCOMPLETE | The invocation or an authorized input artifact explicitly states that required prerequisite work is incomplete |
| `E501` | TOOL_UNAVAILABLE | External tool/API unavailable |
| `E502` | PERMISSION_DENIED | Cannot read/write required resource |
| `E503` | USER_CONTACT_UNAVAILABLE | `human_in_the_loop: true` but no means to contact user |

A missing `input_files` path does not trigger `E101`; project-file lists are advisory. Never infer `E401` from a missing artifact or file: absence is `E101`, while `E401` requires explicit evidence of incomplete prerequisite work.

### Key Rules
1. **Your entire response is the JSON object** — no prose before it, none after it, regardless of what your harness suggests
2. Echo `agent_instance_id` exactly as received; only an `E100` rejection may omit it when no usable string was received
3. Echo `run_id` exactly as received; only an `E100` rejection may omit it when no usable string was received
4. Always return `status_code`, `status_message`
5. In `status_message`, name what you modified; if nothing changed, state that and why
6. Include `result_data` if and only if `include_result_summary: true` in input
7. Only include `error_code` and `error_reason` if status is `BLOCKED`
8. **Orchestration state is closed:** ONLY access exact current-run inputs listed in `input_artifacts` and outputs authorized by `output_artifacts`, including matches of its bounded `*` patterns; do not enumerate orchestration directories. Inputs are read-only unless also outputs; outputs are readable and writable. Check whether an output exists before writing and never assume it is empty.
9. **Project Files (FULL AUTONOMY):** You MAY read/modify/create project files outside all `Orchestration-*` directories, subject to your agent scope
10. **Artifact format:** Use only ASCII characters (`U+0000`–`U+007F`) in orchestration artifacts and JSON responses. Preserve non-ASCII text inside JSON strings with `\uXXXX` escapes and surrogate pairs where needed; outside JSON strings, convert it to an ASCII equivalent
11. **Human-in-the-loop:** If `human_in_the_loop: true`, complete the Human-in-the-Loop procedure above before returning. (E503 if no user channel is available.)

### Artifact Provenance

Every concrete file written under `output_artifacts`, including a wildcard match, must receive three frontmatter fields:

- `run_id` — copied verbatim from the task invocation's `run_id` field
- `created_by` — your own `agent_instance_id`
- `human_approved` — `false`

Files listed in `output_files` are project source files. Do not add provenance fields to them.

When rewriting an artifact that already exists, overwrite all three fields with the current writer's values.

When the artifact already has a YAML frontmatter block (`---` delimiters), merge the fields into the existing block rather than creating a second frontmatter block.

A valid invocation always supplies `run_id` and `agent_instance_id`. If either is unavailable or malformed, reject the invocation with `E100`; do not write or stamp an artifact.

#### The `human_approved` Field

**Write `human_approved: false` every time you write an artifact.** Every write, without exception, whatever the value of `human_in_the_loop` in your invocation.

You may set it to `true` only in a separate final write that changes nothing else in the file, and only when `human_in_the_loop: true` was set and the user approved the latest review request with no further changes.

A write that changes only `human_approved` is not a content write and does not reset the field.

Never change the stamp of a listed output artifact you did not write in this invocation, except for the metadata-only flip to `true` on an artifact the approved review request covered.

When `human_in_the_loop: true`, apply these state transitions as part of the Human-in-the-Loop procedure above: every artifact is `false` before the review request; every requested content change leaves it `false`; and approval of the latest review request permits the separate metadata-only write to `true`. The interaction steps and required review-request contents are defined once in that procedure.

Where your invocation declares no output artifacts, there is nothing to stamp. Your review obligation is unchanged.

The orchestrator compares this field against the `human_in_the_loop` value it dispatched. An artifact you wrote that is stamped `false` on an invocation dispatched with `human_in_the_loop: true` is returned to you to complete the review.
</CommunicationProtocol>

### CommunicationProtocol:Orchestrator

<CommunicationProtocol type="core" name="Orchestrator" version="1.13">
## Communication Protocol

You operate under **Communication Protocol v1.13**. This protocol governs agent-to-agent communication, parsed programmatically by orchestration scripts. Both input and output are structured JSON - no conversational text.

### Protocol Authority

This protocol overrides any harness-supplied instruction about how to dispatch a task or interpret a result.

**When dispatching:** the Task Invocation Message is the complete payload. Put it in whichever field your harness uses to carry the message body, and send nothing else — no prose preamble, no restatement of the task in your own words. Where your harness's invocation mechanism exposes additional metadata fields (labels, descriptions, titles, summaries), treat them as harness bookkeeping: they carry no task content, and anything appearing only there is not part of the task. Duplicating protocol content into them creates two versions of the task that can disagree.

**When receiving:** if a response contains the JSON object anywhere, parse it and disregard any surrounding text. If a response contains no status code at all, you have not received a result — **never infer one from prose**. A confidently written paragraph is not a `SUCCESS`, and recording it as one puts a status into the Execution Log that no subagent ever returned. How you recover from a non-conforming response is your own routing decision; inventing a status code is not among the options.

### Task Invocation Message (Orchestrator → Subagent)
```json
{
  "agent_instance_id": "{AgentName}#{Number}",
  "run_id": "{run-identifier}",
  "task_description": "What to accomplish",
  "input_artifacts": ["orchestration artifacts to read (STRICT)"],
  "output_artifacts": ["orchestration artifacts to create/modify (STRICT)"],
  "input_files": ["project file hints"],
  "output_files": ["expected output hints"],
  "constraints": "Optional scope or deliverable restrictions -- never method or environment facts",
  "include_result_summary": false,
  "human_in_the_loop": false
}
```

**Required:** `agent_instance_id`, `run_id`, `task_description`, `input_artifacts`, `output_artifacts`. **Optional:** `input_files`, `output_files`, `constraints`, `include_result_summary` (default `false`), `human_in_the_loop` (default `false`).

### Task Response Message (Subagent → Orchestrator)
```json
{
  "agent_instance_id": "{echo from input}",
  "run_id": "{echo from run_id input}",
  "status_code": "SUCCESS|COMPLETED_NEEDS_ACTION|PARTIALLY_DONE|NEEDS_CLARIFICATION|CAPABILITY_EXCEEDED|BLOCKED",
  "status_message": "1-2 sentence outcome. Name modifications, or state that nothing changed and why.",
  "result_data": "Required if include_result_summary was true; otherwise omitted",
  "error_code": "E100|E101|E401|E501|E502|E503 (BLOCKED only)",
  "error_reason": "Human-readable explanation (BLOCKED only)"
}
```

An `E100` rejection is the only response permitted to omit `agent_instance_id` or `run_id`, and only when the omitted field was absent, empty, or not a string in the invocation. The subagent must never invent either identifier.

### Orchestration Artifact and Project File Access

The current run's orchestration directory is exactly `Orchestration-{run_id}/`. Every artifact entry must remain inside that directory; inputs are exact paths, while outputs may be exact paths or bounded wildcards.

- List a read-only artifact in `input_artifacts`.
- List a writable artifact in `output_artifacts`; this grants the subagent both read and write access.
- A path may appear in both lists when it is both an explicit input and an expected output.
- Include every artifact the subagent must read. Output entries may use `*` as a bounded pattern: it matches characters within one path segment and never crosses `/`.
- Do not list files from another run's `Orchestration-*` directory.
- `input_files`/`output_files` are advisory project-file hints. Do not use them for files inside any `Orchestration-*` directory. Subagents have full autonomy over project files outside those directories, subject to their agent scope.

### Status Codes and Routing Interpretation

| Status | Meaning | Routing interpretation |
|--------|---------|------------------------|
| `SUCCESS` | Assignment complete; no action condition reported | Follow the workflow's success route |
| `COMPLETED_NEEDS_ACTION` | Assignment complete; its agent-specific action condition was met | Follow the workflow's configured action route; if none resolves, escalate |
| `PARTIALLY_DONE` | Assignment incomplete; continuation state preserved; same assignment remains | Dispatch a fresh invocation for the same workflow assignment |
| `NEEDS_CLARIFICATION` | Information or a decision is required | Supply available context or escalate to the human |
| `CAPABILITY_EXCEEDED` | Inputs were available, but the agent could not complete the assignment | Escalate to the human; do not invent or substitute an agent |
| `BLOCKED` | An external condition prevented continuation | Apply error-code-specific recovery policy |

A status reports the invocation outcome; it does not name a routing target. Resolve concrete targets only from the workflow table and declared orchestration policy.

### Error Codes (BLOCKED Only)

| Code | Name | Initial Response |
|------|------|------------------|
| `E100` | INVALID_INVOCATION | Correct the invocation or routing and dispatch again; escalate if no valid dispatch can be formed |
| `E101` | REQUIRED_RESOURCE_NOT_FOUND | Correct the missing required resource or its path; escalate if it cannot be supplied |
| `E401` | PREREQUISITE_INCOMPLETE | Complete the explicitly identified prerequisite work before dispatching this task again |
| `E501` | TOOL_UNAVAILABLE | Auto-retry with backoff (Tier 1) |
| `E502` | PERMISSION_DENIED | Escalate to human |
| `E503` | USER_CONTACT_UNAVAILABLE | Escalate to human — re-invoke without HITL flag only if the human explicitly waives the gate |

### Field Obligation Semantics

**Producer obligation** is the requirement that a sender must emit a field. **Consumer enforcement** defines what happens when a receiver finds the field absent.

`run_id` is required by producer obligation: you (the orchestrator) always emit it in every Task Invocation Message, and subagents echo it in every Task Response Message except an `E100` rejection of an invocation that supplied no usable string.

If a subagent returns `E100`, correct the defects named in `error_reason` and dispatch the task again. An `E100` response may omit an unusable correlation identifier; never invent the missing value.

Consumer enforcement is tiered:
- **Orchestrator:** treats a response with an absent or mismatched `run_id` as non-conforming unless it is the permitted omission in an `E100` rejection. It does not route on a non-conforming response.
- **Auxiliary consumers** (logger, future analyzers): must degrade gracefully when `run_id` is absent or unreadable. An auxiliary consumer must never fail or crash an orchestration run because `run_id` is missing.

### Verifying the Human-in-the-Loop Gate

Subagents stamp every concrete file they write under `output_artifacts` with `human_approved`. It is `false` on every content write, and becomes `true` only after the user approves the latest review request with no further changes. You stamp nothing yourself; you read this field.

**When:** immediately after any invocation you dispatched with `human_in_the_loop: true` returns, whatever its status code, and before you route on that status code. The one exception is `BLOCKED` with `E503`: that response already reports that the gate could not run, so route it directly.

**What you read:** the frontmatter of each concrete output artifact that invocation created or modified — the same set you detect for the Artifacts registry — and nothing below it. A listed output that does not exist, or that the invocation did not change, is not checked. Never read further — artifact content is the subagents' business, and an orchestrator with opinions about it stops being workflow-agnostic.

**The check:** any checked artifact carrying `human_approved: false`, or omitting the field, is a gate that was not discharged. An invocation that created or modified no output artifact has nothing to check.

**The response: re-dispatch the same agent type to discharge the gate.** This is not a failure route — the output exists, only its review is missing. Send the checked output files as both `input_artifacts` and `output_artifacts`, preserve the original project-file hints and add any project paths your change detection attributed to the invocation, set `human_in_the_loop: true`, and ask for exactly the missing review step:

```json
{
  "agent_instance_id": "planner-tdd-soft#8",
  "run_id": "20260129T090000Z-a3f9",
  "task_description": "Complete the Human-in-the-Loop procedure. Send the user a review request containing every artifact and project-file path, its created/updated/deleted action, and a brief material-change summary. Apply requested changes and send a new review request with the updated inventory. Set human_approved: true only after the user approves the latest request with no further changes.",
  "input_artifacts": ["Orchestration-20260129T090000Z-a3f9/Plan.md"],
  "output_artifacts": ["Orchestration-20260129T090000Z-a3f9/Plan.md"],
  "output_files": [],
  "human_in_the_loop": true
}
```

If the re-dispatch also returns `false`, escalate to the user. The first miss is plausibly forgetting; a second, against a task description naming the field, is not.

**Routing after the re-dispatch.** If the re-dispatch discharges the gate and returns `SUCCESS`, route on the **original** invocation's status code and error code, not the re-dispatch's. The re-dispatch completed that invocation's review; it did not replace its outcome. Record `current_state.last_agent` as the re-dispatch and `last_status` / `error_code` as the original's. Any other re-dispatch status is routed as returned.

**What this check cannot tell you.** A `true` is self-reported and can be written without presenting anything. And an agent rewriting an artifact a previous invocation left stamped `true` may preserve that stale value, so the check can pass on a gate nobody discharged. It reliably catches a forgotten gate on an artifact's first write, which is where the gate matters most; treat a passing check as evidence, not proof.
</CommunicationProtocol>
