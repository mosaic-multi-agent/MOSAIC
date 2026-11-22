# Codex CLI — Active Issues

> Last updated: 2026-11-21 (run 2)

---

### CX-001: `codex exec` refuses/cancels MCP tool calls needing approval (no stdin, approval never) before dispatch

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/24135 |
| **Reported** | 2026-05-22 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.130.0 through 0.169.x (re-tested on 0.155.1, 0.162.1, 0.169.0/0.169.1; not explicitly re-tested on 0.170.0) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | enhancement, mcp, sandbox, exec, CLI, config |

**Summary:**
In headless `codex exec` an MCP tool call that needs approval is not run: it surfaces as "user cancelled MCP tool call" (early versions) or "MCP tool call requires approval, but approval policy is never", and the call never reaches the MCP server. `approval_policy = "never"` alone does not allow it. Labeled enhancement, no maintainer comment; behavior is currently the designed fail-closed path with no per-call host-approval channel. Pre-approval config works.

**Impact on Orchestration:**
MOSAIC MCP servers such as `contact-user` (human-in-the-loop) and any other Runner-attached MCP tool will silently fail in `codex exec` unless pre-approved or accurately annotated. A step relying on an MCP tool fails or the model works around it.

**Evidence:**
- Original reporter reproduction with a minimal stdio server (server child spawns, call cancelled); several later commenters, including a Windows `codex exec --json` integrator (0.154.0) who needed explicit per-tool `approval_mode = "approve"`, and a 0.160.0 report of the refusal before dispatch.
- 2026-11-18 independent test on 0.155.1/0.169.0/0.169.1 with a call-logging server: refused calls never reach the server; documented the working options below. Source review (0.162.1) confirms `exec` cancels elicitation/rejects input requests rather than forwarding them.
- No maintainer engagement on the issue.

**Workaround(s):**
1. (best) Per-tool: `approval_mode = "approve"` under `[mcp_servers.NAME.tools.TOOL]` in config.toml (confirmed by two independent users, 0.154-0.169).
2. Per-server: `default_tools_approval_mode = "approve"` plus `enabled_tools` allowlist (tested 0.155.1 and 0.169.x).
3. Annotate MCP tools accurately: on 0.169.0 in `auto` mode tools with `readOnlyHint: true`, or `destructiveHint: false` and `openWorldHint: false`, run without approval; unannotated tools are treated as risky and refused. Before 0.169.0, `destructiveHint: true` overrode `readOnlyHint: true`.
4. `codex exec --approve-for-me` (automatic reviewer; switches to workspace-write sandbox, rejected together with `--sandbox` on 0.169.0).
5. `--dangerously-bypass-approvals-and-sandbox` (also disables sandbox).
Did not help: `approval_policy = "on-request"` alone, `default_tools_approval_mode = "writes"`, changing sandbox.

**Notes:**
Single-source workaround detail (comment by a third party blog author, 2026-11-18) is consistent with other commenters but not maintainer-confirmed; verify on 0.170.0. Related: #29857 (plugin identification/config). No per-call host approval protocol exists for `exec`; `request_user_input`/elicitation also auto-cancel in exec (see other CX entries if tracked). MCP `contact-user` HITL must therefore be pre-approved in Runner config.

---

### CX-002: `codex exec` silently declines/cancels MCP elicitations: exit code 0, no event in `--json`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/60400 |
| **Reported** | 2026-11-09 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.0 (reproduced on 0.167.0 by two users; source on 0.162.1/main shows same path) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, mcp, exec, CLI |

**Summary:**
When an MCP server requests user confirmation via elicitation, headless `codex exec` answers it automatically (decline under the default `never` approval path; forwarded requests are auto-cancelled in `exec/src/lib.rs`) without surfacing any event in the `--json` stream. The tool gets a "not confirmed" result, the model asks a user who is not there, and the run exits 0. Confirmation-gated tools (drop-collection, delete-many, etc.) never execute. No maintainer response yet.

**Impact on Orchestration:**
A Runner step using an MCP server that gates actions behind elicitation will look successful (exit 0) while the work was not performed; the Runner cannot tell why from the JSONL stream. Elicitation-based human-in-the-loop does not work in `codex exec`.

**Evidence:**
- Reporter: 3/3 runs failed, plus follow-up table for three more gated tools; second user reproduced offline with a synthetic MCP server and Responses stub, confirming decline reaches the server and no JSONL elicitation event is emitted, and traced source paths.
- Related: #35387 (exec auto-cancels plugin-install elicitation), #39149 (headless app-server elicitations can hang), #24135 (see CX-001).

**Workaround(s):**
1. Disable server-side confirmation for unattended use (e.g. `MDB_MCP_CONFIRMATION_REQUIRED_TOOLS=""` for mongodb-mcp-server); server specific.
2. Do not rely on elicitation for HITL in exec; use a plain MCP tool (as `contact-user` does) with pre-approval (see CX-001).

**Notes:**
Reported on a custom provider (OpenRouter) but the behavior is harness-side and independently reproduced with a local stub. Verify on 0.170.0.

---

### CX-003: `codex exec` panics (exit 101) with "failed printing to stdout/stderr: Resource temporarily unavailable" when the pipe reader is slow (Linux)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/55242 |
| **Reported** | 2026-10-19 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.156.1, 0.162.1, 0.166.0 (Linux x86_64 musl); Windows not reported |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI |

**Summary:**
Codex sets `O_NONBLOCK` on its own stdout/stderr when they are pipes or sockets (confirmed by syscall trace in a comment). If the supervisor stops reading while Codex writes a large JSONL line (31-65 KB `command_execution` `aggregated_output`) or large prompt echo, the `print!` hits EAGAIN and the process panics with exit code 101, mid-run. A retry usually succeeds. No maintainer response, no fix PR.

**Impact on Orchestration:**
A Runner that reads the `--json` stream with backpressure (pausing while persisting each event) can see random step crashes on Linux with partial output. Reported rate 9 of 71 runs.

**Evidence:**
- Reporter pattern (9/71 runs, always after large events); second independent user traced the syscall to the codex process itself and reproduced with a 400 KB prompt and unread stderr.

**Workaround(s):**
1. Redirect fds 1 and 2 to regular files and tail them (confirmed in comment; regular-file writes cannot return EAGAIN).
2. Drain stdout/stderr continuously (dedicated reader thread, no pause while persisting).
3. Retry the step on exit 101.

**Notes:**
Linux-specific mechanism; on Windows (MOSAIC's primary platform) no report found, so impact there is unknown. Verify on 0.170.0.

---

### CX-004: `codex exec --json` stream omits some tool items (unmapped ThreadItem variants, MCP calls nested in code-mode `exec`)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/54782 (also https://github.com/openai/codex/issues/55926) |
| **Reported** | 2026-10-17 |
| **Last Activity** | 2026-10-21 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.162.0-alpha.17.2 (Windows), 0.160.0 (Linux); not re-checked on 0.170.0; no activity for 4 weeks (needs re-verification) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI, tool-calls (#54782); bug, mcp, exec, CLI (#55926) |

**Summary:**
The JSONL item mapper in `codex-rs/exec/src/event_processor_with_jsonl_output.rs` maps only eight `ThreadItem` variants and ends in `_ => None`, so DynamicToolCall, FunctionCallOutput, SubAgentActivity, ImageView, etc. never appear in `--json`. Separately, MCP calls nested inside code-mode `exec` cells run but emit no `mcp_tool_call` events (app-server path does emit them). In the same mapper, config warnings and real errors both arrive as `error` items. No maintainer response; a community fork patch exists, no PR (repo is collaborators-only).

**Impact on Orchestration:**
A Runner that derives tool activity, MCP outcomes, or failures from the JSONL stream can miss tool attempts and nested MCP results, and cannot treat an `error` item as a hard failure without also catching warnings.

**Evidence:**
- #54782: reporter read the source at a pinned commit and saw a run with a missing tool attempt; bot-flagged duplicate verified as a distinct but related facet (#55926 is about nested MCP outcomes). Two independent reporters, no maintainer ack.

**Workaround(s):**
1. Do not rely on the stream alone for MCP outcomes; use `-o/--output-last-message` and server-side logs/MCP server call counters (reporter-suggested pattern, unconfirmed).
2. Remove tools from the model catalog (`model_catalog_json`, tool mode `direct`) when fail-closed semantics are needed (reporter workaround, #54782).
3. For nested MCP outcomes: use `codex app-server` notifications instead of `exec --json` (reporter, #55926).

**Notes:**
Related items about the stream: #54055 (terminal usage can be missing/stale), #22570 (structured CodexErrorInfo not forwarded in JSONL error events). Not tracked separately. Run 2: #55774 (open, 0.163.0, single reporter with a minimal stdio-MCP repro, no comments) adds that an MCP call left in flight by a code-mode script that throws/terminates gets `item.started` but never `item.completed`, the turn still completes, and the MCP server never receives `notifications/cancelled`; with no await before the throw not even `item.started` appears. Only reachable when the model drives MCP via code-mode scripts.

---

### CX-005: `--output-schema` constrains every assistant message, not only the final one; intermediate progress messages look like valid final output

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/19816 |
| **Reported** | 2026-04-27 |
| **Last Activity** | 2026-08-29 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.125.0 reported; maintainer says server-side Responses limitation (no fix expected); not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec |

**Summary:**
With `codex exec --output-schema`, the schema is sent as `text.format.schema` on every Responses request in the turn, so intermediate `agent_message` items (progress notes before tool calls) are also shaped to the schema. A consumer that takes the first schema-valid JSON `agent_message` from `--json` as the result will pick up a progress message. A Codex contributor (etraut-openai) stated this is a limitation of the Responses endpoint, cannot be fixed in the client, and is unlikely ever to be fixed.

**Impact on Orchestration:**
Runner steps using `--json --output-schema` must treat only the last `agent_message` (or the `-o` file) as the result; picking the first valid-looking message yields wrong results intermittently depending on whether the model emitted progress messages.

**Evidence:**
- Maintainer-side comment (contributor, 2026-04-28) confirming limitation and no planned fix; 3 thumbs-up; reproduction with explicit command and output.
- A community suggestion (synthetic `final_answer` tool) is unanswered. No workaround offered by maintainers; a user asked for one.

**Workaround(s):**
1. Read the result from `-o/--output-last-message` file or take the last `agent_message` of the turn (derived from the issue evidence; not explicitly blessed by maintainers).
2. Prompt the model to avoid progress messages (unreliable, model-dependent).

**Notes:**
Limitation by design of the API layer: MOSAIC must adapt permanently. Related: #15451 (schema/tool behavior), #35596 (closed: `codex exec review` ignored `--output-schema`), #54285 (open: model-dependent failures with minLength/maxLength constraints, not investigated).

---

### CX-006: `codex exec --output-schema` rejects schema files with a UTF-8 BOM (Windows)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/61060 |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-12 |
| **Confidence** | Likely |
| **Orchestration Impact** | LOW |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.2, 0.170.0-alpha.2, source at current main at filing time |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, exec, CLI |

**Summary:**
`load_output_schema` in `codex-rs/exec/src/lib.rs` reads the file and calls `serde_json::from_str` with no BOM handling, so a UTF-8-BOM schema file fails with "Output schema file ... is not valid JSON: expected value at line 1 column 1" (loud, exits before the run). Prompt decoding already strips a BOM. Source cause was confirmed by a commenter; no maintainer response, no fix PR.

**Impact on Orchestration:**
Any Windows tooling that writes the schema file with a BOM (PowerShell 5 `Out-File`/`Set-Content -Encoding utf8`, some editors) makes the step fail immediately; a loud, easily diagnosed failure.

**Evidence:**
- Reporter PowerShell repro with a mock server; a commenter traced the code path. Single reporter plus code review.

**Workaround(s):**
1. Write the schema file as UTF-8 without BOM (reported working).

**Notes:**
Verify on 0.170.0 stable.

---

### CX-007: `-o/--output-last-message` binds an empty file before the run; failed runs leave a 0-byte file (since 0.168.0)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/61271 |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.168.0 onward (0.163.0/0.167.1 left the file absent on failure) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | (closed as expected behavior) |

**Summary:**
Since 0.168.0, `codex exec -o FILE` opens (creates) the output file before the agent starts to stop sandboxed tools from redirecting the final write. A run that fails before producing a final message (401, usage limit, network failure) leaves an empty 0-byte file behind; an existing file is preserved, not truncated. A Codex contributor (etraut-openai) closed the issue as expected behavior: scripts must check exit status, not file existence.

**Impact on Orchestration:**
A Runner that treats "output file exists" as "agent answered" will read an empty result from failed steps. Pre-0.168 logic using existence checks silently breaks.

**Evidence:**
- Maintainer-side statement of intended behavior (2026-11-14); independent repro on Linux 0.167.1 vs 0.168.0, and root-cause trace to `BoundOutput::open`.

**Workaround(s):**
1. Gate on the process exit code and a non-empty file (maintainer guidance: check exit status).
2. Delete any pre-existing `-o` file before each call so a stale file from an earlier step is not read after a failure (a failed run does not overwrite it).

**Notes:**
Limitation, by design. See CX-008 for the related exit-code-0 problem when the write itself fails.

---

### CX-008: `--output-last-message` write failure (e.g. missing parent dir) is only logged; `codex exec` still exits 0 and the final message is lost

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/33898 |
| **Reported** | 2026-07-17 |
| **Last Activity** | 2026-07-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.144.5 reported; source trace by a second user; output binding was reworked in 0.168.0 (#58974) so current behavior is unverified. Needs re-verification (no activity for 4 months) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI |

**Summary:**
`-o PATH` does not create missing parent directories; the failure is raised only after the turn finishes (stderr line "Failed to write last message file ..."), the write error is logged but not propagated, and the process exits 0. No maintainer response; a commenter has a validated patch but cannot open a PR.

**Impact on Orchestration:**
A Runner step can spend all its tokens and lose the final message with a success exit code.

**Evidence:**
- Reporter repro with exit code echo (macOS 0.144.5); second user confirmed in source (`write_last_message_file` swallows the error).

**Workaround(s):**
1. Create the parent directory before every dispatch (reporter does `mkdir -p`).
2. Verify the file exists and is non-empty after exit 0.

**Notes:**
The 0.168.0 early-bind change (see CX-007) likely changes the missing-directory path (open now happens before the agent starts); re-test on 0.170.0 before relying on this.


---

### CX-020: Windows sandbox: `codex exec` shell commands fail (CreateProcessAsUserW error 5 / runner pipe-in timeout) in read-only and workspace-write

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/60397 (cross-ref #60336 flagged as possible duplicate, not verified) |
| **Reported** | 2026-11-09 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.162.0-alpha.17, 0.170.0 (latest comment) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, exec, CLI, tool-calls, app |

**Summary:**
On Windows, any shell command run under the Windows sandbox (`read-only` or `workspace-write`) can fail before the shell starts. Two modes reported: `CreateProcessAsUserW failed: 5 (Access is denied.)` when Codex picks the MSIX/WindowsApps `pwsh.exe`; and, on 0.170.0, `Failed to create unified exec process: timed out after 15000ms connecting runner pipe-in`. Reproduces with standalone `codex exec`, not only the VS Code extension. No maintainer response; root cause unestablished.

**Impact on Orchestration:**
A headless `codex exec` step on Windows may be unable to run any tool command (build, test, git) when sandboxed, so the agent step cannot do its work or hangs ~15-75 s per attempt and reports failure.

**Evidence:**
- Original reporter on Windows 10 with `codex exec --sandbox read-only`; second reporter on 0.170.0 (Windows 11, Remote SSH) reproduces with both CLI and extension.
- A/B test: same `codex exec` with `--sandbox danger-full-access` succeeds in 66 ms; runner binary signed and launches fine standalone. Sandbox log shows ACL setup error "being used by another process (os error 32); continuing setup".
- No maintainer comment; only bot duplicate suggestion.

**Workaround(s):**
1. Run the step with sandbox disabled (`--sandbox danger-full-access` / Full access) - reported working by the 0.170.0 reporter (removes sandbox protections).
2. For the WindowsApps `pwsh.exe` error: remove `\Microsoft\WindowsApps` from PATH so Codex selects `powershell.exe` 5.1 (reporter says this only changed the failure mode, not a fix).
3. Run 2, from #18937 (open since 2026-04, 8 comments, 5 thumbs-up, 3 independent A/B confirmations on Desktop 26.727 / CLI): the root cause is Codex selecting the Store/MSIX `pwsh.exe` App Execution Alias in `%LOCALAPPDATA%\Microsoft\WindowsApps` (errors 5 or 1312 `CreateProcessAsUserW`, or `CreateProcessW` 5 in the non-sandbox path). Fix: uninstall the MSIX PowerShell package and install PowerShell 7 via MSI so `C:\Program Files\PowerShell\7\pwsh.exe` exists (a side-by-side MSI was NOT enough while the MSIX alias remained); or disable the pwsh App Execution Alias in Windows Settings so Codex falls back to Windows PowerShell 5.1. Restart Codex afterwards. Check with `where.exe pwsh`.

**Notes:**
Run 2: #18937 also reports an explicit absolute `exec_command.shell` (even System32 powershell.exe) is rediscovered and replaced by the WindowsApps `pwsh.exe` (0.149.0, 0.163.0 source trace: `shell_detect.rs` rediscovers via PATH), so pinning the shell in the prompt does not help; and `codex exec` still exits 0 after the model reports the failed command (detect via the `command_execution` item status). Fix PR/branch exists only as an external fork; no maintainer comment. Cross-ref #60336 (Windows MXC launcher rejects MSIX-packaged `pwsh.exe`: "Packaged apps cannot be launched inside a sandboxed container"; Desktop-app focused, 3 reporters, no maintainer reply) - same packaged-PowerShell root symptom as the CreateProcessAsUserW 5 variant. See also CX-021 and CX-025. Related open Windows sandbox issues seen in scan: #18620 (CreateProcessWithLogonW 1326/1909), #54746, #62185. The `.codex/.sandbox/sandbox.<date>.log` file is the diagnostic source.

---

### CX-021: Windows elevated sandbox: CreateProcessWithLogonW 1326/1909 lockout of CodexSandbox accounts, worsened by parallel `codex exec`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/18620 |
| **Reported** | 2026-04-20 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.133.0 through 0.139.0 explicitly; app builds 26.930 (Oct 2026) and comments through 2026-11-16 (CLI version not stated); not confirmed fixed on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, app |

**Summary:**
With `[windows] sandbox = "elevated"`, sandboxed shell commands fail before PowerShell starts with `CreateProcessWithLogonW failed: 1326` (bad credentials) or `1909` (account locked out), or `helper_unknown_error: apply deny-read ACLs`. One reporter shows that ~6 concurrent `codex exec` runs trip the Windows lockout policy on the sandbox account, after which every run (even serial) fails with 1909 until the lockout clears. File edits via apply_patch keep working; only shell execution fails. No maintainer comment in thread.

**Impact on Orchestration:**
(Applies only when `[windows] sandbox = "elevated"` is configured; the 0.170.0 default for `codex exec` with no setting is `unelevated`, see CX-029 Notes. A Desktop-app-provisioned CODEX_HOME commonly sets elevated; see CX-160 for the CLI/Desktop provisioning ping-pong on 0.170.0.)
Parallel fan-out of `codex exec` steps on Windows with the elevated sandbox can lock the sandbox user and break all shell tool calls in every later step. The agent may continue "blind" (edits work, git/rg/tests fail) and hallucinate results instead of erroring.

**Evidence:**
- 10+ independent comments across 7 months, 6 thumbs-up, several variants (1326, 1907, 1909, 740, deny-read ACL error), Windows Security log 4625/4740 evidence.
- Parallel-exec lockout mechanism reported by HDickenson (0.139.0) with an A/B result.
- No maintainer engagement; OpenAI Support (per a commenter) advised reprovisioning or switching to unelevated.

**Workaround(s):**
1. (best) Set `[windows] sandbox = "unelevated"` in `config.toml` - restricted token of the current user, no dedicated sandbox account; reported verified with a 6-way parallel run with 0 failures (single reporter, 0.139.0).
2. Unlock accounts (`IsAccountLocked=$false` via ADSI) after lockout - does not help for 1326.
3. Reprovision: exit Codex, rename `.sandbox`, `.sandbox-secrets`, `.sandbox-bin` under CODEX_HOME, then run one sandboxed command from an elevated shell (e.g. `codex sandbox windows --permissions-profile ":read-only" -- whoami.exe`); reported resolving 1326 (single reporter).
4. For `apply deny-read ACLs`: a corrupted `.sandbox\deny_read_acl_state.json` (null bytes) can be renamed away (single reporter).

**Notes:**
Related to CX-020 (different error codes on the same Windows sandbox stack). Version evidence is older than 0.170.0; recheck on current stable.

---

### CX-022: `codex exec` aborts at startup when `installation_id` in CODEX_HOME is not writable

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/42398 |
| **Reported** | 2026-09-03 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.151.0 through 0.165.0-alpha.1 (source of 0.152/0.159 inspected); not re-verified on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, sandbox, exec, CLI, app-server |

**Summary:**
Codex opens `<CODEX_HOME>/installation_id` with read+write+create before checking whether the existing UUID is valid. If the file/dir is read-only (outer sandbox, EROFS, or another OS user owning the file) `codex exec` exits 1 before emitting any JSONL: `failed to initialize in-process app-server client: Operation not permitted`. Redirecting log_dir/sqlite_home and `--ephemeral` do not avoid it. An unmerged third-party fix (read-only fast path) exists on a fork; no maintainer comment.

**Impact on Orchestration:**
A Runner step launched with a read-only or shared CODEX_HOME (e.g. inside an outer sandbox, a second user, or a read-only per-call home) fails instantly with no JSON event stream, which looks like a startup crash rather than a sandbox error.

**Evidence:**
- Four independent reproductions (macOS nested sandbox, Codex Cloud EROFS, WSL2 DrvFS multi-user, macOS restricted lab) with strace-level evidence and code inspection of the resolver.
- Third-party fix PR on a fork (argszero/codex #1); no maintainer acknowledgement in the thread.

**Workaround(s):**
1. Ensure CODEX_HOME (and `installation_id`) is writable by the user running `codex exec`; give each user/sandbox its own CODEX_HOME.
2. Not available: flags such as `--ephemeral`, `analytics.enabled=false` were tried and do not avoid it.

**Notes:**
Only relevant if MOSAIC ever runs Codex with a non-writable home. Re-check on 0.170.0 before acting.

---

### CX-024: Windows `apply_patch` fails "path contains a reparse point" for regular files on non-C: drives

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/53466 (see also #48612, #52978 not investigated) |
| **Reported** | 2026-10-12 |
| **Last Activity** | 2026-11-09 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.160.0 and 0.162.0-alpha.17; app 26.1007; not verified on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, tool-calls, app |

**Summary:**
In a `workspace-write` sandbox on Windows, `apply_patch` on existing regular files in a workspace on a secondary NTFS drive (E:, D:) fails with `Failed to read file to update <path>: path contains a reparse point`, while patching files on C: works. Three independent reporters verified with fsutil/Get-Item that there is no reparse point. Persists in fresh sessions and after elevated sandbox re-provisioning. No maintainer response.

**Impact on Orchestration:**
A `codex exec` agent working on a repo on D:/E: cannot edit existing files via the built-in `apply_patch`; the agent may retry or fall back to shell-based writes (which may also be sandbox-limited).

**Evidence:**
- Three independent reproductions (two with Codex CLI 0.160.0/0.162.0, different drives D:/E:, Windows 10 build 19045).
- OpenAI Support (per reporter) asked for logs; no maintainer comment in thread.

**Workaround(s):**
1. None confirmed. Place the workspace on C: (inferred from reports that C: patches succeed; not confirmed as a fix).

**Notes:**
Windows 10 only in reports so far (MOSAIC primary platform is Windows 11, repo on C:), so materiality is conditional on workspace drive. Reporter cited #30009 and #29072 as possibly related.

---

### CX-025: Windows MXC sandbox: rename and/or delete denied inside writable workspace (breaks atomic temp-file writes)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/52978 (upstream: https://github.com/microsoft/mxc/issues/1204, https://github.com/microsoft/mxc/issues/1676) |
| **Reported** | 2026-10-10 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.162.0-alpha.17 (MXC backend); Codex Desktop on 2026-11-17 (CLI version not stated); not verified on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, CLI |

**Summary:**
With the Windows MXC sandbox backend (`[windows] sandbox = "mxc"` / `prefer_mxc`), an explicitly writable workspace allows create/edit/read but rename (and for some, delete) fails with `UnauthorizedAccessException`. Reproduced through `codex sandbox` directly. Appears rooted in Microsoft's MXC/PSEC enforcement (Microsoft acknowledged a related rename bug and promised a Windows cumulative update). One later reporter sees delete succeed but rename fail, which broke `pixi`/`uv` cache temp-file persist.

**Impact on Orchestration:**
Any tool inside a sandboxed `codex exec` step that writes via temp file + rename (package managers, formatters, git operations, build tools) can fail with "Access denied", causing test/build steps to fail although plain writes work.

**Evidence:**
- Direct CLI repro with script (no model); second reporter with a different failing combination (rename only) on Desktop on 2026-11-17.
- Microsoft MXC issue #1204 confirmed by Microsoft (fix in a Windows CU, per commenter); #1676 reports delete denied in `readwritePaths`. No Codex maintainer comment.

**Workaround(s):**
1. Redirect tool caches/temp dirs into the workspace (e.g. `PIXI_CACHE_DIR` to a repo-local dir) - reported working for the pixi case (single reporter).
2. Use the non-MXC Windows sandbox (`unelevated`/`elevated`) or no sandbox; not tested by the reporters.

**Notes:**
MXC is NOT the default Windows sandbox in 0.170.0 (source-verified run 2: used only when `[windows] sandbox = "mxc"` is set or the dev feature `prefer_mxc` is enabled, default off; with no setting `codex exec` uses `unelevated`). Relevance to MOSAIC is therefore limited to machines whose CODEX_HOME was configured for mxc (e.g. by the Desktop app); the #62185 report shows Desktop reaching `Windows sandbox selection: mxc`. Related older rename issue #22044. Re-check after Windows CU lands.

---

### CX-026: `notify` hook silently stops firing once cumulative payload exceeds argv limit (Windows 32K, Linux 128K); Desktop re-adds a notify hook to config.toml

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/18309 (related per commenters: #25141, #30873, #38985, #32315, #41809, #40245, not investigated) |
| **Reported** | 2026-04-17 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | LOW |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.121.0 through 0.167.0 (cumulative-payload repro on 0.167.0, incl. `codex exec resume`); Desktop-managed notify seen with bundled CLI up to 0.167 and Desktop 26.1111 (2026-11-15); source analysis by commenters on main |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, exec |

**Summary:**
The legacy `notify` program receives the turn-complete JSON as the last argv element, and that payload includes every user message in the thread (`input-messages`, cumulative). Once the command line exceeds the OS limit (Windows 32,767 chars; Linux 131,072 bytes per argument), process creation fails (os error 206) and notify never fires again for that thread, even across `codex exec resume`; exit code stays 0 and nothing is printed (only a `warn!` in logs, hook treated as FailedContinue). On Windows Desktop, the app re-writes a `notify = [codex-computer-use.exe, "turn-ended"]` entry into the user `config.toml` on startup, so the warning recurs even after the user removes it. No maintainer response; commenters have proposed/tested stdin transport on forks.

**Impact on Orchestration:**
Turn completion itself is unaffected (hook failure is non-fatal), but any MOSAIC tooling that relied on `notify` to learn that a `codex exec` turn finished would silently go deaf on long or resumed threads. Runner uses the JSONL stream and `-o`, so direct impact is low; watch for hook-warning noise in logs if CODEX_HOME is shared with Desktop.

**Evidence:**
- Many independent reproductions (Desktop, CLI, synthetic spawn tests), exact limits measured, source references (`hook_runtime.rs`, `legacy_notify.rs`), 0.167.0 `codex exec resume` table showing notify never fires after crossing the limit.
- Open for 7 months, no maintainer comment found.

**Workaround(s):**
1. Do not depend on `notify`; use `--json` event stream / `-o` last message for completion detection.
2. Start a fresh thread (not resume) for long conversations (reporter, 0.167.0).
3. Desktop: commenting out the notify entry in `config.toml` works only until Desktop re-installs it; project-level `notify = []` is ignored.

**Notes:**
Maintained at LOW because MOSAIC Runner does not use notify. Promote if a MOSAIC feature starts using notify hooks.

---

### CX-027: `workspace-write` sandbox makes `.git` / worktree gitdir read-only: `git add`/`commit`/`fetch` fail (by design)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/14338 (related: #21869 push variant, #61989, #61936 not investigated) |
| **Reported** | 2026-03-11 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | All versions since March 2026 (Linux bwrap, macOS Seatbelt); latest comment 2026-11-17; Windows reports of read-only `.git` in workspace-write exist (#61989) but not verified here |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | enhancement, sandbox |

**Summary:**
In `workspace-write`, `.git` and resolved `gitdir:` targets (including a linked worktree's gitdir and common dir) are kept read-only even inside writable roots. Git cannot create `index.lock`, update `FETCH_HEAD`, refs, etc., so `git add/commit/fetch` fail. An OpenAI contributor confirmed it is intentional (aligns Linux with macOS; earlier Linux behavior was accidental). The thread is a feature request for a scoped override; none exists. `--add-dir`/`writable_roots` do not override the protected carve-out.

**Impact on Orchestration:**
A sandboxed `codex exec` step that must commit (e.g. a commit-manager style agent) or run git operations that write metadata will fail or stop waiting for approval, which in headless mode means failure. A related variant (#21869): `git push` can succeed remotely while the local tracking-ref update fails, leaving a stale state with exit code 0.

**Evidence:**
- OpenAI contributor (viyatb-oai) explains the carve-out is intended; 14 thumbs-up on the issue; multiple reporters on macOS/Linux, worktree variant reported July 2026.

**Workaround(s):**
1. Run git-writing steps with a wider sandbox per call (`--sandbox danger-full-access`) or perform git operations outside Codex (the orchestration layer commits after the step).
2. Third-party narrow MCP git tool (e.g. `codex-safe-git`, unreviewed, single author) to keep the sandbox.
3. Adding the `.git` dir to `writable_roots` is reported as not effective for the carve-out; for linked worktrees commenters call it an over-broad tradeoff.

**Notes:**
Permanent by-design behavior; plan MOSAIC git write steps accordingly. After a push-like step, verify with `git ls-remote`.

---

### CX-028: Windows `exec_command` rejected before launch: "CreateProcess Rejected ... blocked by policy" (also under danger-full-access / never)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/54746 (also #59314 CLI 0.166.0 `codex exec`, #60191, #59781, #56722 not individually investigated) |
| **Reported** | 2026-10-17 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Bundled CLI 0.163.0-alpha through 0.166.0 (incl. standalone `codex exec` 0.166.0); not confirmed fixed on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, app (#59314: exec, CLI, tool-calls) |

**Summary:**
On Windows, the shell tool (`exec_command` launching PowerShell) is intermittently rejected before any process is created with `CreateProcess Rejected(... rejected: blocked by policy)`, with no rule, reason or approval request. Seen with commands such as `Remove-Item`, process/browser launches, git or deploy scripts, even when the turn context shows `approval_policy=never` and `danger-full-access`; user execpolicy rules contain no matching forbid rule and `--ignore-rules` does not avoid it. A CLI repro (`codex exec --sandbox read-only`, 0.166.0) is rejected for the WindowsApps `pwsh.exe` wrapper. Commenters suspect a Windows command-safety classifier (PR #54076 is cited as related false-positive work); no maintainer confirms the cause.

**Impact on Orchestration:**
A headless Runner step (`approval=never`) cannot ask for approval, so a rejected command simply fails the tool call with an opaque message; the agent may stall, loop or abandon the task. Diagnosing the denying layer is not possible from the error.

**Evidence:**
- 12-comment thread with ~9 independent Windows reports (Oct-Nov 2026), plus separate issues with the same text; one CLI `codex exec` reproduction (#59314).
- Several reporters verified rules files had no forbid/prompt rules and `execpolicy check` returned no matches.
- No maintainer comment; only duplicate-bot suggestions.

**Workaround(s):**
1. Switch the session to Ask for approval (`workspace-write`) and approve the specific command via `require_escalated` - reported working by 3 independent Desktop users; not applicable to headless `approval=never`.
2. Avoid compound scripts mixing URL/HTTP checks with process launches (suspected classifier trigger; unconfirmed).
3. For the WindowsApps `pwsh.exe` variant, see CX-020 (PATH without WindowsApps).

**Notes:**
Mostly reported from the Desktop app; only #59314 is a plain `codex exec` repro. #59314 also reports, in `windows.sandbox="unelevated"` with `workspace-write`, `UnsupportedOperation("windows unelevated restricted-token sandbox cannot enforce split writable root sets directly; refusing to run unsandboxed")` (references #35864; not investigated here); `prefer_mxc` then failed with error 668. Related: CX-020, CX-025. Run 2 addition: #62670 (open, 2026-11-20, plain `codex exec` 0.153.4 and desktop 0.169.1) shows the same Rejected text for EVERY shell call, including pure reads, under `-s read-only` on Windows; a review-only step cannot read any file in the npm CLI build (sandbox selection logged as `disabled` there vs `mxc` in the desktop build, which has a `node_repl` read path). Only `workspace-write` lets the session read. No maintainer comment. See also CX-160 (marker mismatch with desktop app).

---

### CX-029: Windows `unelevated` sandbox refuses split writable roots: `exec_command` and `apply_patch` update/delete fail ("refusing to run unsandboxed")

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/35864 (CLI repro on 0.166.0: https://github.com/openai/codex/issues/59314) |
| **Reported** | 2026-07-29 |
| **Last Activity** | 2026-11-04 (#59314); #35864 last 2026-10-10 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.145.0-0.166.0 reported (0.162.0, 0.163.0, 0.166.0 in #59314); Desktop builds up to 26.730 reproduced; one Desktop user reports fixed in 26.810.52044 (Aug); CLI 0.166.0 still failing in Nov; 0.170.0 not verified |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, CLI |

**Summary:**
With `[windows] sandbox = "unelevated"` and managed `workspace-write` permissions that yield split writable root sets, the restricted-token sandbox raises `windows unelevated restricted-token sandbox cannot enforce split writable root sets directly; refusing to run unsandboxed`. `apply_patch` Add File works but Update/Delete fail (verification read is sandboxed), and in #59314 plain `codex exec` shell commands fail too. A third-party patch (fork commit 381acb1) fixes it; no maintainer comment. Evidence on whether it was fixed upstream conflicts: a Desktop user reported success in 26.810, but a CLI 0.166.0 reproduction exists from November.

**Impact on Orchestration:**
A Runner step on Windows with workspace-write under the unelevated sandbox may be unable to modify or delete existing files via `apply_patch` or to run shell commands at all. It fails closed (no unsandboxed fallback), so it is a hard step failure rather than a silent one.

**Evidence:**
- #35864: controlled Add/Update/Delete probe plus root-cause trace and reference fix; four independent confirmations; one user retest showing a fix in Desktop 26.810.
- #59314: `codex exec --sandbox workspace-write -c windows.sandbox="unelevated"` fails on CLI 0.166.0 (also 0.162/0.163); `codex sandbox --permission-profile :workspace` works.
- No Codex maintainer response in either thread.

**Workaround(s):**
1. Set `[windows] sandbox = "elevated"` (reported restoring sandboxed execution by one commenter; but see CX-021 for elevated-mode account lockout risk).
2. Use PowerShell/shell-based edits as fallback for updates (works per reporters, unless shell also fails).
3. Self-compile Codex with the fork commit (not practical for MOSAIC; single-author).

**Notes:**
Related: CX-020, CX-021, CX-025, CX-028. The backend selection (`elevated`, `unelevated`, `mxc`) clearly matters for which Windows failures appear; MOSAIC should record which backend its Windows Runner uses.
Backend default (source-verified at rust-v0.170.0, run 2): with NO `[windows] sandbox` key in config.toml, `codex exec` on a Windows cwd resolves to `unelevated` (restricted token; core/tools/sandboxing.rs upgrades `Disabled` to `RestrictedToken`). `elevated` and `mxc` are used only when `[windows] sandbox = "elevated"/"mxc"` is set (the Desktop app/TUI setup flow writes this into the shared `~/.codex/config.toml`). So this entry (unelevated) is the weight-bearing one for a default Runner on Windows; CX-021 applies only if elevated is configured (or inherited from a Desktop-app-configured CODEX_HOME), and CX-025 only if mxc is configured or the dev flag `prefer_mxc` is on (default off, UnderDevelopment).


---

### CX-040: Full ChatGPT-apps tool catalog sent every request; fresh session exceeds context window and auto-compaction loops

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/61679 |
| **Reported** | 2026-11-15 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.0-alpha to 0.169.0-alpha (reporters); not confirmed fixed in 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, mcp, CLI, context |

**Summary:**
With `features.apps` on (default) and many ChatGPT apps connected, every request carries the whole apps tool catalog (718 tools = 288k input tokens on the first request). A brand-new session is over the window, and auto-compaction fires after every tool call without being able to shrink the tool definitions, looping and burning quota. `codex exec` shows the same token count. A second user traced a trigger: when the models cache is stale (>5 min) and refresh times out/401s, Codex uses fallback model metadata (258,400 window) and sends apps tools in full instead of via code mode.

**Impact on Orchestration:**
A headless `codex exec` agent step on an account with many connected apps can burn its entire budget in a compaction loop without progress, or fail on the first request. Fixed context (instructions + tool definitions) is invisible in the MOSAIC prompt, so the cause is hard to spot.

**Evidence:**
- Detailed measurements from rollouts and logs; independent Windows reproduction (893 tools, 5 compaction cycles/2.4M input tokens in 5 minutes).
- No maintainer response; 3 comments, 0 reactions at time of capture.

**Workaround(s):**
1. Set `[features] apps = false` (per project `.codex/config.toml` or via `-c features.apps=false` in the exec call): reporter measured 288k -> ~42k tokens.
2. `[apps.<connector_id>] enabled = false` for the largest apps (288k -> 173k).
3. If "Unknown model ... fallback model metadata" appears in logs: point `model_catalog_json` at a copy of the `models` array from `~/.codex/models_cache.json` (freezes the model list).

**Notes:**
Only affects accounts using ChatGPT login with connected apps; an API-key-only setup may not be exposed. Related: #34719 (tools lost after compaction), #38185 (no opt-out for plugin injection), CX-041 (#59534 remote compaction loop).

---

### CX-041: Remote compaction v2 loops: next request larger than pre-compaction request, no no-progress guard

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/59534 |
| **Reported** | 2026-11-05 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.166.0 (reporter); a second Desktop reproduction on unknown version 2026-11-16 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, context |

**Summary:**
In long threads a successful remote compaction (one `compaction` item, valid `replacement_history` ~30k tokens) is followed by a request that is still larger than before compaction (244k -> 272k -> 292k -> 312k vs 258,400 window), immediately re-triggering compaction; 37 compactions in under an hour until aborted. A second reporter saw 27 compactions in 71 minutes, with retained multi-MB inline images in replacement history and post-compaction requests still ~242k tokens. No maintainer response.

**Impact on Orchestration:**
A long-running `codex exec` step (large implementation or review) can spin in compaction without progress, consuming quota and wall time, with no endpoint error to detect it. Steps that read images or large outputs are more exposed.

**Evidence:**
- Two independent reports with rollout-level evidence; no maintainer acknowledgement; no fix PR linked.

**Workaround(s):**
1. None confirmed. Keep agent steps short/scoped so they finish before reaching the compaction threshold; avoid inlining large images into long sessions (second reporter's hypothesis, unconfirmed).
2. Supervise with an external timeout/no-progress kill on the exec process.

**Notes:**
Distinct from CX-040 (fixed tool-catalog overhead), but the symptom (compaction loop) is the same; check both when diagnosing.

---

### CX-042: `codex exec` reports untrusted/modified hooks as bare "Failed"; plugin with symlink silently loses all hooks

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/61741 |
| **Reported** | 2026-11-16 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.169.0 (reporter); not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI, skills, hooks |

**Summary:**
Hooks whose trust status is `modified` (e.g. after a plugin update) are not run by `codex exec`, but are printed as `hook: SessionStart Failed` with no hook name, plugin or reason; the reason is not in the rollout JSONL or trace logs. Separately, a symlink (e.g. `node_modules`) in a plugin directory makes fingerprinting fail and silently drops all of that plugin's hooks (only a warning in app-server `hooks/list`). A commenter confirmed the two code paths from source, without reproduction.

**Impact on Orchestration:**
If MOSAIC relies on Codex hooks (logging, guards, Stop/SessionStart handlers) in headless exec, hooks that fail the trust check are skipped without a diagnostic, and nobody is there to approve the review prompt, so expected hook side effects silently do not happen.

**Evidence:**
- Single reporter with concrete repro; one commenter's source-level analysis; no maintainer response. Included as Unverified because hooks/trust affect automated headless runs on stock infrastructure.

**Workaround(s):**
1. `--dangerously-bypass-hook-trust` made all 67 hook runs complete (reporter; security trade-off, only for controlled workspaces).
2. Re-review/approve hooks interactively after any plugin update; avoid symlinks inside plugin directories.

**Notes:**
Plugin-delivered hooks; applicability to MOSAIC depends on whether hooks are used in Runner steps. Related: #38295 (bare "Failed" on Windows, shell resolution cause).

---

### CX-043: Config.toml default `model` / `model_reasoning_effort` / `service_tier` ignored at startup unless `-m` is passed (ChatGPT workspace auth)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/36322 |
| **Reported** | 2026-07-31 |
| **Last Activity** | 2026-11-17 |
| **Confidence** | Likely |
| **Orchestration Impact** | LOW |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.146.0 through 0.169.0 (five independent reproductions, latest 2026-11-17 on 0.169.0 Windows); not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, auth, CLI, config |

**Summary:**
With ChatGPT (Enterprise/workspace) authentication, plain `codex` starts on a different model/reasoning effort than `~/.codex/config.toml` specifies, even though `codex doctor` resolves the configured value. Passing `-m <model>` makes the configured `model_reasoning_effort` honored again; `-c model_reasoning_effort=...` also works. Reports are about interactive startup (TUI and Desktop); `codex exec` was not tested by any reporter. Cause unknown (possible managed new-thread defaults); no maintainer response.

**Impact on Orchestration:**
If a Runner step relied on config.toml defaults (not explicit `-m`/`-c`/agent TOML) the model or reasoning effort could silently differ from intent, changing quality and cost. Likely limited if MOSAIC always passes model and effort explicitly.

**Evidence:**
- Five independent reports across CLI/Desktop, macOS and Windows; bot-suggested duplicate #35136 was disputed by the reporter (not verified here).
- Not shown to affect `codex exec`.

**Workaround(s):**
1. Always pass `-m <model>` and `-c model_reasoning_effort="..."` explicitly (confirmed by reporters).

**Notes:**
Verify with a `codex exec` smoke test whether agent-TOML `model`/`model_reasoning_effort` are honored on ChatGPT-auth workspaces before relying on defaults. Related window-misreport: #62239 (Desktop).


---

### CX-060: Concurrent first-run `codex exec` sessions on a fresh CODEX_HOME intermittently lose rows in `state_5.threads`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/42447 |
| **Reported** | 2026-09-03 |
| **Last Activity** | 2026-09-03 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.150.1 (macOS); not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI, session |

**Summary:**
When several `codex exec` processes start simultaneously against a brand-new `CODEX_HOME`, some never get a row in the `threads` table of `state_5.sqlite`. All processes exit 0 with correct output and all rollout JSONL files are complete; only the SQLite thread index is missing rows (observed 1/6, 6/6, 5/6 threads on three fresh homes). One warm-up session first (or a previously used home) avoids it.

**Impact on Orchestration:**
Parallel Runner fan-out into a fresh or freshly provisioned CODEX_HOME (CI, new machine, clean test bench) can leave sessions missing from the thread index, so they may not be resumable or listed by id, with no error signal. Steady-state homes are unaffected per the report.

**Evidence:**
- Single reporter, concrete reproduction script and measurement table, stock OpenAI backend.
- No comments, no maintainer response, no linked PR. Dead since 2026-09 (6+ weeks).

**Workaround(s):**
1. Run one throwaway `codex exec` against the CODEX_HOME before launching parallel runs (reporter-validated, single reporter).
2. Rollout files under `sessions/` remain intact, so data is recoverable.

**Notes:**
Needs re-verification on 0.170.0. Related but distinct: #38150 (thread-writer-locks under CODEX_HOME on shared/NAS storage; enhancement, not tracked) and #31184 (`database is locked`).

---

### CX-061: Completed subagents stay resident (pending mailbox / `pending_init`) and block later `spawn_agent` with "agent thread limit reached" (watch-item)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/34518 (related: #32353, #22779, #33777, #39694, #49986) |
| **Reported** | 2026-07-21 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Likely |
| **Orchestration Impact** | LOW |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Desktop with runtime 0.148.0-alpha.15 through app-server 0.167.0-alpha.4.1 (multi-agent V2); one CLI confirmation (version not stated) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, app, subagent |

**Summary:**
In multi-agent sessions, a subagent shown as `completed` can remain a hidden resident (queue-only `send_message` leaves a pending mailbox item; `context_compacted`/viewing history can revive a completed child as `pending_init`), so a later `spawn_agent` fails with the generic `collab spawn failed: agent thread limit reached` even with few active workers. The root cannot close or see the blockers. Recovery reported: drain via `followup_task`, raise `[agents] max_concurrent_threads_per_session`, restart. Still open, no maintainer response; later comments (Nov 2026) show it on app-server 0.167.0-alpha.4.1.

**Impact on Orchestration:**
MOSAIC uses Codex Runner-only (one `codex exec` per step), so this only matters if a Runner-launched agent itself spawns subagents in a long session. Watch-item for any future native multi-agent use: fan-out waves in one session can silently exhaust the spawn cap.

**Evidence:**
- Controlled repro in the original report (baseline completed agents are reclaimable; pending-mailbox variant blocks spawn); 6 independent comments on Desktop incl. one CLI confirmation.
- No maintainer acknowledgment found; several cross-referenced issues.

**Workaround(s):**
1. Avoid queue-only `send_message` to completed agents; use `followup_task` to drain (reporter-validated, original report).
2. Raise `[agents] max_concurrent_threads_per_session` in config and start a new session/restart (observed to restore capacity; not isolated).
3. Keep each `codex exec` run short-lived with a small number of spawned agents.

**Notes:**
Mostly Desktop-reported; CLI relevance from one comment. Not re-checked on 0.170.0 stable.


---

### CX-080: `codex exec` stalls 5-17 min when the response stream goes silent; idle timeout (300 s default) is not configurable for the built-in provider

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/50775 |
| **Reported** | 2026-10-04 |
| **Last Activity** | 2026-10-22 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.160.0 (both reporters); not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, exec, CLI, connectivity |

**Summary:**
A headless `codex exec` run can sit with no events for 5 to 17 minutes while the process is idle (0% CPU, one open API connection). A maintainer says this is consistent with the existing 300 s stream idle timeout times up to 5 retries, not a proven infinite hang (retry events are not persisted in the rollout). A second reporter with debug logs shows 4 of 120 parallel (8-16 way) calls (3.3%) going silent after the final `message` output item started, with no `response.completed`; the same calls run serially (24) had 0 stalls. `stream_idle_timeout_ms` cannot be changed for the built-in `openai` provider.

**Impact on Orchestration:**
In parallel Runner fan-out a small share of steps can lose several minutes per stall (and the tail of an already-started final message), with no error until the idle timeout fires. Without a Runner-side no-output timeout the step looks hung.

**Evidence:**
- Maintainer (CONTRIBUTOR, etraut-openai) response: 300 s idle timeout and 5 retries by default; asked for logs to prove a longer open request.
- Second reporter: debug-log signature identical across 4 stalls, rates table (parallel 3.3% vs serial 0%), transport `responses_websocket`, fresh CODEX_HOME per call.
- No fix PR; no activity since 2026-10-22.

**Workaround(s):**
1. Runner-side: kill/retry a step after N seconds of no JSONL events (the 2nd reporter's hedge: start a duplicate call after 45 s silence and take the first answer; recovered a stall in 63 s vs 309 s).
2. `codex exec resume <session id>` after killing a stalled process (original reporter; second stall happened again in their case).
3. Lower parallelism (serial runs showed no stalls in one data set).

**Notes:**
Treated as Limitation because the idle window is by design and not user-configurable for the built-in provider. Needs re-verification on 0.170.0. Related: CX-060 (parallel exec concurrency), #41085/#31184 if tracked.

---

### CX-081: Session start fails with `state_5.sqlite` "database is locked" (code 5) when many processes share one CODEX_HOME

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/31184 (related: #30105 bot-suggested duplicate, not verified; #41085 WAL growth, not tracked) |
| **Reported** | 2026-07-05 |
| **Last Activity** | 2026-08-28 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.142.5 (reporter, Linux); no data for 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, session |

**Summary:**
With many Codex processes in parallel on one CODEX_HOME, a new process can fail at startup with "Codex couldn't start because another Codex process is using its local data ... failed to open state DB at state_5.sqlite ... (code: 5) database is locked". Reporters attribute it to heavy disk I/O (HDD; gone after moving to SSD; one prompt appeared after 15 min) or to an orphaned backend of a crashed front-end still holding the DB. The message does not name the blocking PID. No maintainer response; a third-party fork fix exists (not a PR).

**Impact on Orchestration:**
Parallel Runner fan-out of `codex exec` on a shared CODEX_HOME may see intermittent startup failures (or long waits) before any model work, especially on slow disks or after a killed/crashed exec leaves a lingering process.

**Evidence:**
- Original reporter plus 2 other independent commenters (different triggers: I/O pressure; orphaned process found via `fuser`); fork commit with Docker reproduction (not upstream).
- No maintainer comment, 0 reactions; last activity 2026-08-28; unreproduced on current versions (needs re-verification).

**Workaround(s):**
1. Use SSD storage / a separate CODEX_HOME per parallel exec (reporter: SSD removed the errors).
2. After a crash, kill leftover `codex` processes holding the DB (`fuser state_5.sqlite logs_2.sqlite` on Linux) before retrying; make the Runner kill the whole process tree on abort.
3. Retry the start after a short delay.
4. Recovery of a corrupted/locked DB: back up via sqlite3 `.backup`, replace the DB, delete `-shm`/`-wal` (single reporter).

**Notes:**
Reports are Linux; Windows behavior untested here. Related: CX-060 (fresh-home concurrent first run loses thread rows) and CX-080.

---

### CX-082: `codex exec resume` silently starts a fresh thread (or picks the wrong session) instead of failing when the selector does not match

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/61013 (related: #61003 `--last` picks an older session when the state index is incomplete; #61035 explicit ID silently ignored when `--last` is also given; the latter two not read in full) |
| **Reported** | 2026-11-12 |
| **Last Activity** | 2026-11-13 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.2 (Windows) and 0.168.0 (Linux) for #61013; 0.167.0-alpha.4.1 for #61003; not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, exec, CLI, session |

**Summary:**
`codex exec resume <name> <prompt>` where the name is not found exits 0, starts a brand-new thread and runs the prompt without the intended history (an unknown UUID or an unknown fork name exit 1 correctly). Separately, `resume --last` can resume an older session when a newer rollout is missing from the `state_5.sqlite` index. No maintainer response on either; reproductions use a local mock Responses server.

**Impact on Orchestration:**
A Runner that resumes a Codex session by name or `--last` can silently lose conversation context (new thread, exit 0) rather than getting an error, so a later step runs without its prior state. Resume by explicit thread UUID fails closed. Combined with CX-060 (missing index rows after concurrent first runs) the `--last` mismatch is plausible in fan-out.

**Evidence:**
- #61013: original Windows reproduction (0.167.2) plus an independent Linux confirmation on 0.168.0 with a table of exit codes and request counts.
- #61003: Windows reproduction by deleting the index row; the reporter notes it does not establish how often natural index gaps occur.
- #61035 (run 2, read in full): `codex exec resume --last <ID> <prompt>` is accepted without error and silently resumes the NEWEST session, ignoring the explicit ID (0.167.2 Windows, 0.170.0-alpha.2; independently confirmed on Linux 0.168.0 with a mock model; ID-only control resumes correctly). Never combine `--last` with an ID.
- No maintainer comment or fix PR on any of them.

**Workaround(s):**
1. Resume by explicit thread UUID (taken from the `thread.started` JSONL event), which errors when unknown; avoid names and `--last` in the Runner.
2. Verify that the `thread_id` in the first `thread.started` event of a resume equals the requested ID; treat a different ID as a failed resume.

**Notes:**
Taken together with CX-060, session-index reliability under parallel exec is weak; rollout files under `sessions/` remain the source of truth.


---

### CX-100: `apply_patch` silently applies a hunk whose final blank context line is not in the file

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/60876 |
| **Reported** | 2026-11-11 |
| **Last Activity** | 2026-11-11 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.0-alpha.3.2 (Windows, binary tested); source unchanged on main at filing; not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, tool-calls |

**Summary:**
`compute_replacements` in `codex-rs/apply-patch/src/file_update.rs` retries a failed match without the last empty pattern line (meant as a trailing-newline sentinel). A real blank context line is also an empty string, so a hunk ending in a blank context line is applied (exit 0, file changed) even when the file's next line is not blank. An unmatched non-blank context line correctly fails (exit 1). A second commenter verified the root cause from source and proposed restricting the retry to `is_end_of_file`. No maintainer response, no fix PR.

**Impact on Orchestration:**
A model-generated patch with a wrong trailing blank context line is accepted instead of rejected, so the edit may land in a place the model did not intend with no error to trigger a retry. Rare (needs a mis-specified trailing blank context line) but silent.

**Evidence:**
- Single reporter with a self-contained repro (three cases, exit codes and file bytes) on a Windows binary; independent source-level confirmation in a comment.
- No maintainer comment, 0 reactions; included as Unverified because apply_patch is the core file-edit tool and the repro is concrete on stock infrastructure.

**Workaround(s):**
1. None from the thread. Review diffs after steps (orchestration layer / git diff) rather than trusting a successful apply_patch exit status.

**Notes:**
Distinct from the display-only streaming-preview `\r\r\n` mismatch #62327 (behind default-off `apply_patch_streaming_events`; skipped as non-material).

---

### CX-102: Windows hooks report bare "Failed": `bash` resolves to WSL shim, hook stdout JSON with CRLF or unknown fields is rejected

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/38295 |
| **Reported** | 2026-08-13 |
| **Last Activity** | 2026-09-02 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.144.1 through 0.152.1 (three independent reporters); no activity for 10+ weeks and not re-tested on 0.170.0 (needs re-verification) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, exec, CLI, hooks |

**Summary:**
On Windows, hook commands run via `cmd.exe /C` with no interpreter handling. Three distinct failure causes were found in the thread: (1) a hook command `bash <script>` resolves via PATH to the WindowsApps WSL shim, which mangles Windows script paths (exit 127); (2) hook stdout JSON containing a carriage return (`{}\r\n`, what Python text mode writes on Windows) fails the hook while `{}\n` completes; (3) any unrecognised top-level field in the hook's JSON reply (including Claude's `async` marker or a `metrics` field) fails it. All of these surface only as `hook: <Phase> Failed` with no exit code or stderr, although the runner already captures them. A hook whose command-string hash is not trusted is skipped with no output at all. No maintainer response.

**Impact on Orchestration:**
If MOSAIC Runner steps rely on Codex hooks (guards, logging, SessionStart/Stop handlers) on Windows, hooks written for Claude Code can silently fail or be skipped, and the only diagnostic is the word "Failed". Hook output formatting differences (CRLF, extra fields) between harnesses matter for any shared hook scripts.

**Evidence:**
- Original reporter reproduction (exit-127 capture, PATH fix), a source-level confirmation of `build_command`/`default_shell_command` in `codex-rs/hooks/src/engine/command_runner.rs`, and a third user's byte-level CRLF pair test plus unknown-field test on 0.152.1 (PATH already preferred Git Bash there).
- No maintainer comment, 0 reactions; related #18887 (opaque Stop-hook JSON error for unsupported fields).

**Workaround(s):**
1. Put `C:\Program Files\Git\bin` (Git Bash) ahead of WindowsApps on PATH, or call hooks via an explicit interpreter path (single reporter, fixes cause 1 only).
2. Emit hook JSON with LF-only newlines (`sys.stdout.reconfigure(newline="\n")` in Python) and only fields Codex recognises (third reporter, local patch).
3. Use `codex exec --json` plus external logging instead of hooks where diagnostics matter.

**Notes:**
Related: CX-042 (untrusted/modified hooks reported as bare "Failed", symlink drops plugin hooks). Verify on 0.170.0 before relying on hooks in the Windows Runner.

---

### CX-103: `apply_patch` deletes the file when `*** Move to:` resolves to the same file as the source (including case-only renames on case-insensitive disks)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/34210 (earlier closed report #6358 not read) |
| **Reported** | 2026-07-19 |
| **Last Activity** | 2026-11-14 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | main at 2026-07-19 (macOS source build); commenter says still present on 0.169.0 (macOS case-only rename, `codex exec`); Windows not tested; not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, tool-calls |

**Summary:**
In `apply_patch`, the move branch writes the new contents to the destination and then unconditionally removes the source. If source and destination are the same file, the just-updated file is deleted while the tool reports success (`M foo.txt`, exit 0). A November comment shows the same on a case-insensitive filesystem for a case-only rename (`readme.md` to `README.md`): `codex exec` exits 0, the model is told `M README.md`, and the file is gone. Canonical-path comparison would not catch the case-only variant (device/inode check needed). The reporter has a prepared fix (not opened as PR); no maintainer response.

**Impact on Orchestration:**
A step that renames a file by changing only its case, or moves a file onto itself, silently loses the file. Windows NTFS is case-insensitive by default, so the case-only variant is plausible there (not confirmed on Windows). Needs a rare patch shape, hence MEDIUM despite data loss.

**Evidence:**
- Direct CLI reproduction with before/after evidence, a second user's reproduction of the case-only variant on 0.169.0, source reference (`codex-rs/apply-patch/src/lib.rs` move branch). No maintainer comment.

**Workaround(s):**
1. For case-only renames, do the rename in two steps (rename to a temporary name, then to the target) or via shell `git mv`, rather than a single `Update File` + `Move to`.
2. Keep work under git so a deleted file can be restored (`git checkout -- <file>`); have the orchestration layer verify expected files exist after a step.

**Notes:**
Workaround 1 is inferred from the mechanism, not reported working. Windows behavior should be tested before assuming it applies.

---

### CX-104: Short shell command returns empty output with exit 0 when its bytes arrive after the 50 ms post-exit cap (loaded host)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/57413 (related: #57191 merged fix that narrows the window, #37207 lagged receivers) |
| **Reported** | 2026-10-27 |
| **Last Activity** | 2026-10-27 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.160.0 and 0.164.0 source (Linux container `--cpus=2` on a loaded host, `codex app-server`); #57191 (merged, unreleased at filing) may reduce it; not re-tested on 0.170.0; no activity for 3+ weeks |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, tool-calls |

**Summary:**
In the unified exec path (`codex-rs/core/src/unified_exec/`), a command that exits within the 150 ms early-exit grace cancels output collection, and `collect_output_until_deadline` then waits at most `POST_EXIT_CLOSE_WAIT_CAP = 50 ms` for the buffered bytes. Any scheduler pause over 50 ms (CPU throttling, busy host, VM steal) lets the timer win, and the tool returns `Original token count: 0`, empty `Output:`, exit code 0. The bytes are also missing from `item/completed` and the rollout. `unified_exec = false` does not avoid it. The reporter proposes raising the cap to 2 s and gives a deterministic unit-test shape; no maintainer response.

**Impact on Orchestration:**
A parallel Runner fan-out of `codex exec` steps on a busy or CPU-limited machine can see a shell/test/git command report success with no output, so the model decides on missing evidence (for example "no test output") with no error signal.

**Evidence:**
- Single reporter, source-level root cause with line references and a deterministic unit-test recipe; observed intermittently on a loaded CI host with a scripted Responses endpoint. Included as Unverified because it threatens tool execution reliability in automated, parallel workflows on stock infrastructure.
- One "please assign me" comment; no maintainer acknowledgement.

**Workaround(s):**
1. None from the thread. Reduce CPU contention (lower parallelism, avoid tight CPU quotas) and have prompts verify command results (for example re-run a command that returns empty output).

**Notes:**
Reported on Linux; whether the timing window is reachable on Windows is untested. Re-check on 0.170.0 after #57191.

---

### CX-105: Large tool/command output is middle-truncated by design (errors in the middle are lost; no spill-to-file)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/9502 (related: #9504, #59622 not read) |
| **Reported** | 2026-01-19 |
| **Last Activity** | 2026-11-05 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Reported on 0.87.0 (Windows); a Nov 2026 comment still discusses it as current; exact truncation budget on 0.170.0 not verified |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, agent |

**Summary:**
When shell, build, test or MCP tool output exceeds the model-facing token/byte budget, Codex keeps a prefix and suffix and drops the middle, which is where compiler or test failures often sit. An OpenAI contributor (etraut-openai) said this is "as designed" and any change needs a design discussion; the issue is effectively an enhancement request. Later commenters note the model must re-run the command with redirection or grep to recover the lost part, and that large MCP tool responses are truncated too (a user had to batch requests of five elements). Competitors spill full output to a temp file; Codex does not.

**Impact on Orchestration:**
A Runner step running a large build or test suite may see only the head and tail of the output and miss the failure, then re-run or misjudge success. Long-output MCP tools lose data the same way.

**Evidence:**
- Maintainer-side statement of by-design behavior (2026-01-19); several independent users reporting the same pain through November 2026; no fix PR.

**Workaround(s):**
1. Have agents redirect long command output to a file and read or grep the relevant part (user-suggested; also what the model does after truncation).
2. Use quiet or summary flags of build/test tools so failures are in the tail (inferred, not from the thread).
3. For MCP tools, page or batch results server-side (reporter workaround).

**Notes:**
By design: MOSAIC must adapt permanently (agent instructions to filter or redirect noisy commands). The configurable truncation settings mentioned in the issue were not verified here. Related distinct entry: CX-104 (output lost entirely on short commands under load).

---

### CX-106: `apply_patch` `Delete File` fails ("Failed to delete file") on writable paths where Add/Update succeed (Linux and Windows MXC)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/18534 (upstream comparison: https://github.com/microsoft/mxc/issues/1676) |
| **Reported** | 2026-04-19 |
| **Last Activity** | 2026-11-05 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.121.0 (Linux, aarch64 container); Windows Desktop bundled 0.164.0-alpha.11.1 with elevated sandbox plus `prefer_mxc` (Oct 2026); not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, tool-calls |

**Summary:**
On two platforms, `apply_patch` successfully adds and updates a disposable file but the matching `*** Delete File:` patch fails with `Failed to delete file <path>` while the file still exists and plain shell deletion works. The Linux report also had a bwrap namespace problem in the same session. The Windows reproduction (MXC backend) shows the short-lived `--__codex-windows-mxc` helper exiting without any DELETE request on the file in Process Monitor; root cause unknown, and a commenter points to the Microsoft MXC issue about delete/rename denied inside `readwritePaths` (see CX-025). No maintainer response.

**Impact on Orchestration:**
An agent that must delete or move files (cleanup, refactor, rename via delete + add) cannot do it with the built-in edit tool and may loop or fall back to shell deletion, which may itself be limited by the sandbox.

**Evidence:**
- Original Linux reproduction with minimal Add/Update/Delete patches and shell control; a detailed Windows reproduction with logs, Process Monitor capture and an OpenCode comparison (delete passes there). Two independent platforms, no maintainer comment.

**Workaround(s):**
1. Delete through the shell (`rm` / `Remove-Item`) instead of `apply_patch` Delete File (the control in both reports succeeded on Linux; not tested under Windows MXC).
2. Non-MXC Windows sandbox backend or no sandbox (inferred from the MXC link; not confirmed).

**Notes:**
Related: CX-025 (MXC rename/delete denied), CX-029 (unelevated sandbox Update/Delete failures). The two platform reports may have different root causes.


---

### CX-120: `codex exec` MCP tools unreachable when deferred behind `tool_search` (>100 MCP tools, or `unsupported call: tool_searchtool_search_tool`)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/24536 (empty-success variant); https://github.com/openai/codex/issues/55512 (dispatch error variant) |
| **Reported** | 2026-05-26 (#24536); 2026-10-20 (#55512) |
| **Last Activity** | 2026-10-21 (#55512); 2026-10-02 (#24536) |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | #24536: 0.133.0 (source analysis); #55512: 0.160.1 and 0.163.0 (including bundled 0.163.0-alpha.10.2) with model `gpt-6-astra`; neither re-tested on 0.170.0 (needs re-verification) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, mcp, exec, CLI, tool-calls |

**Summary:**
Two related failure modes make configured MCP tools unreachable in headless `codex exec`. (1) #24536: when the combined MCP tool count crosses `DIRECT_MCP_TOOL_EXPOSURE_THRESHOLD = 100` (e.g. ChatGPT-auth apps connector plus bundled plugin MCPs), every MCP tool, including the user's own configured server, is hidden behind `tool_search`; the model may emit only reasoning, the turn completes with `last_agent_message = null`, and `exec` finishes "successfully" with empty output. (2) #55512: with a single one-tool stdio MCP server on the default OpenAI provider and `gpt-6-astra`, the model's `tool_search` call is rejected by Codex's router with `unsupported call: tool_searchtool_search_tool`, so no MCP tool is ever called; three independent reproductions (macOS CLI, macOS Desktop, Windows Desktop). No maintainer response on either; an unreviewed bot-style code suggestion in #24536 is not a fix.

**Impact on Orchestration:**
A Runner step that must call an MCP tool (e.g. `contact-user` for HITL) can silently not call it: the step exits 0 with an empty or error-reporting final message, and the JSONL stream shows no MCP call. This looks like a model failure rather than a tool-exposure problem.

**Evidence:**
- #24536: request-shape capture showing 17 top-level tools and no direct MCP namespaces when over the threshold; removing the bundled plugin made the call succeed. A later Windows Desktop automation user reported the same discovery failure (July 2026).
- #55512: five of five runs failing with a minimal probe server, two further independent reproductions, one with a source-level hypothesis (tool-search call arriving as a `FunctionCall` instead of `ToolSearchCall`). Working fallback: discovery through `functions.exec`/`ALL_TOOLS`.
- No maintainer comment on either issue; both open.

**Workaround(s):**
1. Keep the total MCP tool count under 100 for exec runs: `-c features.apps=false`, disable unneeded plugins and apps (see CX-040), so the configured server stays directly exposed.
2. Name the MCP server and tool explicitly in the step prompt and require a final message; treat an empty final message or exit 0 without the expected tool call as failure (check server-side call logs).
3. For `unsupported call: tool_searchtool_search_tool`: reporters found code-mode discovery (`functions.exec` / `ALL_TOOLS`) works; a different model may avoid the path (not confirmed; the report is specific to `gpt-6-astra`).

**Notes:**
Related: CX-004 (nested MCP calls missing from `--json`), CX-040 (apps tool catalog overhead; same cause of the >100 threshold), CX-001 (approval refusal). Pinned/always-direct exposure per server was requested in #24536 but does not exist.

---

### CX-121: Stdio MCP replies over 16 MiB are silently discarded; the call hangs until the tool timeout (regression since ~0.167.0)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/60383 |
| **Reported** | 2026-11-09 |
| **Last Activity** | 2026-11-10 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.167.0 (CLI repro in comment) and bundled 0.167.0-alpha.3.2; 0.163.x handled ~94 MB replies; not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, mcp, app |

**Summary:**
After the rmcp bump (#54653) Codex's local stdio MCP transport enforces rmcp's 16 MiB default line limit and never calls `with_max_line_length()`, so a stdio server reply (`tools/call` or `resources/read`) larger than 16 MiB is dropped with only a log line (`Incoming message exceeded the maximum line length, discarding it`). The pending request is not failed; it waits until the tool timeout (300 s in the app). The opt-in newer stdio transport (#57210) caps at 8 MiB. A commenter reproduced it with `codex exec` on 0.167.0 using a synthetic server (1 MiB reply fine, 17 MiB hangs until the configured tool timeout) and built a candidate patch that fails fast; no maintainer response, limit not configurable.

**Impact on Orchestration:**
An MCP tool returning a large payload (file snapshot, database dump, base64 blob) makes a `codex exec` step stall for the full tool timeout and then fail, with no clear error. MOSAIC's own MCP servers return small payloads, so the risk applies only to third-party MCP tools.

**Evidence:**
- Reporter logs and root-cause trace; independent CLI/`exec` reproduction with timing in comment (2026-11-10); no maintainer comment, no fix PR (a community patch is offered).

**Workaround(s):**
1. Keep MCP replies well under the limit: chunk/paginate (reporter reads in chunks under 4 MiB).
2. Set a tight `tool_timeout_sec` on the server so a dropped reply fails fast instead of stalling (implied by the exec repro: reply returned at the configured tool timeout).
3. A third-party stdio proxy that truncates oversized results (mcp-diet) was suggested by a commenter; unreviewed, single source.

**Notes:**
Distinct from model-visible output truncation. If MOSAIC adds MCP servers returning bulk data, enforce size caps server-side.

---

### CX-122: `--output-schema` / strict-schema requests fail model-dependently: whitespace-only stream stalls or instant `response.incomplete: max_output_tokens` with zero usage (`gpt-6.1-sol`)

| Field | Value |
|-------|-------|
| **Classification** | Quirk |
| **Source** | https://github.com/openai/codex/issues/54285 |
| **Reported** | 2026-10-15 |
| **Last Activity** | 2026-10-29 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.163.0-alpha.11 and 0.165.0 (macOS) with `gpt-6.1-sol`, `gpt-6-sol` (one repetitive run); not re-tested on 0.170.0 and no activity for 3 weeks (needs re-verification) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, model-behavior, exec, CLI |

**Summary:**
With `codex exec --json --output-schema`, large or constrained schemas can make a model fail structured generation. Variant A (first report): the stream stays open but emits only whitespace after an array opening for 25+ minutes (102k deltas), ending in a generic `server_error`; the CLI's idle timeout never fires because deltas keep arriving, and the automatic retry stalls the same way. Variant B (cross-model tests, two more users): `gpt-6.1-sol` returns `response.incomplete` with reason `max_output_tokens` and zero usage within 6-7 s, with no assistant item, even though no output limit was set; removing eight `maxLength`/`minLength`/pattern constraints, or (with function tools) disabling strict mode or shrinking total strict-schema size, makes it succeed. `gpt-5.6-sol` succeeded in all trials. Labeled `model-behavior`; no maintainer response and the root cause (model, server-side constrained decoding, or CLI instructions) is not isolated.

**Impact on Orchestration:**
A Runner step that uses `--output-schema` for structured results can hang for many minutes (whitespace loop) or fail instantly with a misleading `max_output_tokens`, depending on model and schema details. Switching model versions can change this silently.

**Evidence:**
- Three independent reporters (including a later `exec` run on 0.165.0 with ~29 KB schema and an MCP source tool); an 18-run cross-model table; a pinned-snapshot regression in the Responses API with strict tools (a commenter).
- No maintainer comment; intermediate deltas are not exposed in `--json` (see CX-004), so the stall is invisible to a JSONL consumer.

**Workaround(s):**
1. Simplify the schema: remove `minLength`/`maxLength`/`pattern` constraints and keep the schema small (validate lengths in your own code); reported to make `gpt-6.1-sol` runs complete.
2. Use a model that does not exhibit it (`gpt-5.6-sol` passed 3/3 in the cross-model test; model-specific and may change).
3. Add an external wall-clock/no-progress timeout around the exec process and treat `max_output_tokens`/zero-usage as a schema problem rather than a budget problem.

**Notes:**
Related: CX-005 (schema applies to every assistant message), CX-080 (silent stream stall timeout not configurable). Model-dependent, so classified Quirk rather than Bug.

---

### CX-123: `exec --json` error events carry only a message: retry notices ("Reconnecting... n/5") look identical to fatal errors, no typed error info (auth, quota, rate limit)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/22570 (closed duplicate #36562 not read) |
| **Reported** | 2026-05-13 |
| **Last Activity** | 2026-10-13 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.144.1 to 0.162.1 (empty-home repro on 0.159.3 and 0.162.1; source on main at the time); not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | enhancement, exec |

**Summary:**
In `codex exec --json`, `ThreadErrorEvent` has a single `message` string. Reconnect/retry notices and the terminal failure are emitted as the same top-level `{"type":"error","message":...}` event (the only difference is an English prefix like `Reconnecting... 2/5`), then `turn.failed` with just a message, and the process exits 1. The structured `CodexErrorInfo` (`unauthorized`, `usageLimitExceeded`, `serverOverloaded`, HTTP status) and `willRetry` that `codex app-server` exposes are dropped by exec. Example: with no credentials, five WebSocket retries, a fallback to HTTPS, and five more retries are all printed as `error` events (a missing credential is typed `httpConnectionFailed 401`, not `unauthorized`, in app-server). Open enhancement request with a concrete proposal; no maintainer response.

**Impact on Orchestration:**
A Runner that treats any `error` event as a fatal failure will mark healthy-but-retrying steps as failed (see also CX-004: warnings also arrive as `error` items); a Runner that ignores them cannot distinguish auth failure, usage-limit/rate-limit exhaustion, and transient network errors except by parsing English text. Usage-limit and 401 failures surface only as message text plus exit code 1.

**Evidence:**
- Two independent commenters with verbatim JSONL (Windows 0.144.1 `turn.failed` with message only; Linux no-credential run on 0.159.3/0.162.1 with side-by-side app-server output) and source references to where exec drops the fields. Open since May with no maintainer reply.

**Workaround(s):**
1. Decide success from the process exit code plus `turn.completed` / a non-empty `-o` file (see CX-007), not from the presence of `error` events; treat only `turn.failed` or a final `error` followed by exit != 0 as fatal.
2. Classify failures by message substring (e.g. `401`, `usage limit`, `Reconnecting`) as a stopgap, accepting the fragility.
3. Use `codex app-server` instead of `exec --json` if typed errors are required (heavier integration).

**Notes:**
Limitation in the sense that exec's JSONL contract is currently message-only; a change is requested but not planned. Related: CX-004, CX-080 (silent stream stalls), CX-007.

---

### CX-124: `codex exec resume` rejects `-s/--sandbox` and does not inherit the session's sandbox/network policy; resumed turns can run broader than the first turn

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/40149 |
| **Reported** | 2026-08-22 |
| **Last Activity** | 2026-11-10 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.147.0, 0.149.0, 0.154.0 (network variant), 0.167.0 (re-confirmed on macOS 2026-11-10); not re-tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, sandbox, exec, CLI, session |

**Summary:**
`codex exec resume` has no `-s/--sandbox` option (`error: unexpected argument '-s' found`) although `exec` does, and it does not restore the policy the session was launched with; the effective sandbox is re-resolved from the current environment. A session opened with `exec -s read-only` wrote a file when resumed without extra flags, because the directory had `trust_level = "trusted"` (trust granted by an earlier `workspace-write` run) and trust wins when no `-s` can override it. The same applies to network access enabled via `-c`/`-p` at exec time: it is dropped on resume (curl fails to resolve until `-c sandbox_workspace_write.network_access=true` is re-passed), and a more permissive first turn loses permissions when resumed. The rollout file's `turn_context.sandbox_policy` does record the launch policy but resume does not use it. Measured by filesystem and network effect, not model self-report. No maintainer comment, but four independent users reproduced across three versions and two OSes, with a source reading confirming `ResumeArgs` lacks the field.

**Impact on Orchestration:**
A Runner that uses `exec resume` for follow-up rounds (reviewer loops, human-answered continuations) silently changes sandbox and network policy between turns unless every resume re-passes the policy: a step meant to be read-only can write, or a step that needed network/full access can lose it mid-conversation.

**Evidence:**
- Four independent reports with a reproducible script and a measurement table (file on disk / HTTP status); latest 2026-11-10 on 0.167.0 with rollout-file evidence.
- Observed precedence: `config.toml sandbox_mode` > directory `trust_level` > default; `-c sandbox_mode=...` re-applied on each resume works.
- Open since August with no maintainer response.

**Workaround(s):**
1. Re-pass the intended policy on every resume with `-c sandbox_mode="..."` (and `-c sandbox_workspace_write.network_access=true` when needed); confirmed by multiple users on several versions.
2. Set `sandbox_mode` (and `network_access`) in `config.toml` / `CODEX_HOME`, which does reach resumed sessions (confirmed 0.149.0 and 0.154.0).
3. One user's wrapper reads `sandbox_policy.type` from the first `turn_context` of the session rollout and passes it back through `-c` on resume (depends on the rollout format).
4. Note `--dangerously-bypass-approvals-and-sandbox` is accepted by `resume`, so only the permissive direction has a flag.

**Notes:**
Related: CX-082 (resume selector problems), CX-027. MOSAIC's Runner should always pass the sandbox policy explicitly on resumed invocations; verify on 0.170.0 whether `-s` is accepted now.

---

### CX-125: Explicit `$skill` invocation of skills with `allow_implicit_invocation: false` is reported "not available in this session"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/23454 (related #43727 uppercase display-name variant, not read; #19695 cited) |
| **Reported** | 2026-05-19 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.131.0 through 0.166.0 (CLI TUI, Desktop; plugin skill on 0.166.0 on 2026-11-04); not tested on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, skills |

**Summary:**
Skills whose `agents/openai.yaml` sets `policy.allow_implicit_invocation: false` are left out of the model-visible "Available skills" list, and an explicit `$skill` mention does not always inject them: the model replies the skill is unavailable (sometimes only on the first try, or when sent as a steer while a turn is running). Confirmed independently by 10+ users across CLI, App and Windows; 10 thumbs-up. Contributing causes found in comments: the mention parser accepts only ASCII letters/digits and `_-:` (a skill name containing a dot, such as `release.deploy`, is cut short, behavior appears intentional); the `$` picker can submit the display name with different case than the canonical name (#43727); plugin-delivered explicit-only skills were not injected on 0.166.0. No maintainer comment on the issue.

**Impact on Orchestration:**
MOSAIC delegates skills through the shared `.agents/skills` root. If a step relies on a skill being explicitly named in the prompt and that skill is explicit-only, the model may proceed without it (or report it as unavailable) with no error; the run still exits 0.

**Evidence:**
- Many independent reports including a `codex exec` validation (fresh exec sessions resolved a renamed, dot-free skill correctly), rollout-level diagnostics showing the skill enabled yet absent from the catalog; no maintainer response, no fix PR.

**Workaround(s):**
1. Leave `allow_implicit_invocation` at its default (true) for skills the Runner must reach (reported fix: set to true and restart).
2. Keep skill names lowercase with letters, digits, `-`, `_` only (no dots); mention them by canonical name.
3. Repeat the explicit mention (reported to work on the second attempt) or paste the SKILL.md path/payload into the prompt (reported working for plugin skills).

**Notes:**
Relevance depends on whether MOSAIC skills ever set `allow_implicit_invocation: false`; verify with a `codex exec` smoke test on 0.170.0. Related: CX-042 (hooks/plugins in exec).

---

### CX-126: AGENTS.md chain silently truncated at `project_doc_max_bytes` (32 KiB default), deepest file first, no user-visible warning

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/43075 (symptom); https://github.com/openai/codex/issues/37956 (source analysis, docs request) |
| **Reported** | 2026-09-05 (#43075); 2026-08-11 (#37956) |
| **Last Activity** | 2026-10-01 (#43075); 2026-08-18 (#37956) |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.153.4 (symptom: first 37,089 bytes read, 4,321 lost); source verified on main in Aug 2026 (default 32 KiB); not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, app, config (#43075); documentation, CLI, config (#37956) |

**Summary:**
Codex concatenates the project AGENTS.md chain (root first, then nested files down to the cwd) and stops at `project_doc_max_bytes` (default 32 KiB). When the budget runs out the tail is cut with `data.truncate(remaining)`, which hits the deepest, most specific file first (or the end of a single long file, even mid-word), and only a `tracing::warn!` is emitted; no UI or JSONL warning. The global `~/.codex/AGENTS.md` is exempt from the budget. In multi-environment turns one budget is shared across environments, so an earlier chain can starve a later one. Both issues are open with no maintainer response; the behavior is a coded default and a docs gap rather than a defect acknowledged by OpenAI.

**Impact on Orchestration:**
Instructions placed at the end of a large AGENTS.md (or in nested files) can be absent from a `codex exec` run without any signal, which looks like the model ignoring rules. MOSAIC injects project context via AGENTS.md; a large project doc can silently lose its tail.

**Evidence:**
- Reporter measured the cut-off and had the model quote the last sentence (truncated mid-fragment); #37956 cites exact source lines and a repo whose own root AGENTS.md uses 70% of the budget; a second user re-verified on main.
- No maintainer comment on either issue.

**Workaround(s):**
1. Raise the limit: `project_doc_max_bytes = 65536` in config.toml (or `-c project_doc_max_bytes=...` per exec call).
2. Keep the AGENTS.md chain under 32 KiB (`wc -c` on root plus nested files on the cwd path); put the most important rules at the top and split rarely needed rules into nested files.
3. Put always-required rules in the global `~/.codex/AGENTS.md` (not subject to the cap) or in the agent TOML `developer_instructions`.
4. Quick check: ask the agent to quote the last lines of its AGENTS.md instructions.

**Notes:**
Limitation by design (budget is intentional, silent truncation is the gap). Related request: explicit `agents_files` config list (#38680) would share the same budget. Verify the default and whether a warning now exists on 0.170.0.

---

### CX-160: Windows elevated sandbox: `codex exec` cannot run any shell command after the desktop app provisions the sandbox (setup marker `proxy_ports` mismatch); exec still exits 0

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/62397 (related #54441 repeated UAC prompts, #61268 elevated helper exits 1; not read in full) |
| **Reported** | 2026-11-19 |
| **Last Activity** | 2026-11-20 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | codex-cli 0.163.0, 0.169.0, 0.170.0 with Codex desktop app 26.1111.2228.0 sharing one CODEX_HOME, `[windows] sandbox = "elevated"` |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, exec, CLI |

**Summary:**
If the Codex desktop app (or a managed-proxy client) has provisioned the elevated sandbox, `.codex\.sandbox\setup_marker.json` stores `proxy_ports: [3128, 8081]` and `allow_local_binding: true`. A plain `codex exec` computes desired `[]/false` from its own environment, the exact-equality marker check demands re-provisioning, and the elevated setup helper exits 1 (with or without a UAC prompt). Every shell call then fails with `Failed to create unified exec process: orchestrator_helper_exit_nonzero`. A second reporter confirms `codex exec` still exits 0 with `turn.completed`; only the `command_execution` item has `exit_code: -1`, `status: "failed"`.

**Impact on Orchestration:**
A Runner step on a machine where the desktop app also runs can burn a full model run (about 250k input tokens reported) without executing any command, and look successful by exit code. Steps depending on shell commands produce nothing; a UAC prompt in a headless run would hang or fail. No maintainer response yet.

**Evidence:**
- Two independent reporters; a third commenter traced it in source at rust-v0.170.0 (`offline_proxy_settings_from_env`, `SetupMarker::request_mismatch_reason` in windows-sandbox-rs/src/setup.rs). No maintainer comment; Codex Action bot flagged #61268 as possible duplicate (not verified).
- Tried without effect: `--enable network_proxy`, `--enable windows_sandbox_service`, `CODEX_NETWORK_ALLOW_LOCAL_BINDING=1`, upgrading to 0.170.0, app-bundled codex.exe. `codex sandbox` keeps working in this state, so it is not a health check.

**Workaround(s):**
1. Read `%CODEX_HOME%\.sandbox\setup_marker.json` before starting `codex exec`; do not start while `proxy_ports` is non-empty or `allow_local_binding` is true (reporter's pre-check; not confirmed reliable).
2. One reporter recovered by quitting/restarting the desktop app, `sc start "CodexSandboxService.OpenAI.Codex"`, letting a desktop command re-provision to `[]/false` (once, unconfirmed).
3. Do not run the desktop app and `codex exec` against the same CODEX_HOME; give the Runner a dedicated CODEX_HOME.
4. Detect the failure from `--json` item `exit_code: -1`/`status: failed` rather than the process exit code.

**Notes:**
Part of the Windows sandbox cluster (CX-020, CX-021, CX-029); matters only if the desktop app is installed and used on the MOSAIC machine. Backend default is now established (run 2): `unelevated` when no `[windows] sandbox` key is set; this entry applies when the shared CODEX_HOME config says `elevated`.

---

### CX-161: Turn ends (`task_complete`) right after a commentary/progress message that promises a next action; no tool call follows, work stalls until a manual "continue"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/27352 (related #32389 is a different client, Pi agent, not tracked; #36506 closed as duplicate, not read) |
| **Reported** | 2026-06-10 |
| **Last Activity** | 2026-11-16 |
| **Confidence** | Likely |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.139.0, 0.144.1, 0.145.0, 0.163.0 (TUI, ACP, Desktop, VS Code); not re-verified on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, CLI, agent, subagent |

**Summary:**
The model emits a `phase=commentary` message stating it will continue (or a reasoning-only response), no tool call is attached, and Codex records `task_complete` immediately. Internal log shows `model_needs_follow_up=false` / `needs_follow_up=false` for the reasoning-only/commentary-only shape, so the finalizer accepts it as terminal. Reports span models (gpt-5.5, gpt-5.6 Terra, gpt-6.1-sol); one A/B shows GPT-5.6 Sol completing the same task continuously, and one trace links it to context near the window limit (about 240k of 258k tokens). A stronger variant: the turn completed two seconds after `spawn_agent`, abandoning the child.

**Impact on Orchestration:**
A headless `codex exec` step ends with exit 0 and a final `agent_message` that is only a progress note ("I'll now run the tests..."), so the Runner records the step as done without the work being performed. Long multi-stage steps are most exposed. Combined with `--output-schema` (CX-005) the progress text may even be schema-valid.

**Evidence:**
- 9 comments, at least 6 independent reporters across CLI/ACP/Desktop/VS Code with rollout-level evidence; no maintainer comment.
- Most recent comment (2026-11-16) reproduces on 0.163.0 with gpt-6.1-sol max effort.

**Workaround(s):**
1. Runner-side check: after the step, verify the expected artifacts or a required completion marker in the final message; if absent, `codex exec resume` with a "continue the task" prompt (resume caveats in CX-082 and CX-124).
2. Instruct the agent to end only with a defined completion token/final report and not to emit progress-only messages (model-dependent, unreliable).
3. Keep steps small so context stays far from the window limit; switch model (Sol variant reported to run continuously).
4. A watchdog that resumes an idle session (reporter on Desktop; for exec this means a Runner re-dispatch).

**Notes:**
Partly model behavior (model-dependent in the A/B), partly a finalizer gap that no maintainer has addressed; treat as Bug with Quirk characteristics. Related: CX-005, CX-041, CX-061.

---

### CX-162: `workspace-write` / `read-only` sandbox has network off by default: commands see dead proxy `127.0.0.1:9` and `CODEX_SANDBOX_NETWORK_DISABLED=1`; on Windows even loopback TCP connect fails (WinError 10013)

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/47671 (closed, explained by a community comment) and https://github.com/openai/codex/issues/60449 (open; related #53278, #54675, #57671, #60058 not read) |
| **Reported** | 2026-09-23 |
| **Last Activity** | 2026-11-18 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.153.4 (#47671, VS Code extension); Desktop/Work 26.1103.4239.0 and CLI 0.167.0 comparison (#60449); not re-checked on 0.170.0 |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, sandbox, extension / app, connectivity |

**Summary:**
Without network access enabled, sandboxed commands run with `ALL_PROXY`/`HTTP(S)_PROXY`/`GIT_HTTP(S)_PROXY` pointing at `http://127.0.0.1:9` (a deliberate black hole, per a community comment on #47671; not a maintainer statement) and `CODEX_SANDBOX_NETWORK_DISABLED=1`, so `git fetch/push`, package installs and HTTP calls fail with confusing proxy errors. On Windows (Desktop/Work executor, Low Integrity) TCP bind/listen on loopback works but connect fails with WinError 10013, so tests against a local server/DB fail; a resumed conversation with full-access profile could connect while a new conversation could not. The reporter of #47671 reports the symptom later stopped on its own.

**Impact on Orchestration:**
A Runner step sandboxed as `workspace-write` that needs network (dependency install, git remote, web fetch) or loopback services (integration tests, local mock servers) fails with opaque errors; the agent may loop trying alternatives. The cause is not visible in tool output unless the agent inspects the environment.

**Evidence:**
- #47671: one reporter plus one explanatory community comment (no maintainer); closed by the reporter. #60449: three reproductions on two Windows machines, including a Node.js minimal repro; no maintainer response (bot suggested duplicate #60058).
- #60449 notes the standalone CLI 0.167.0 with elevated sandbox could bind/listen/connect on loopback, so the loopback block appears specific to the Desktop/Work executor; not established for plain `codex exec`.

**Workaround(s):**
1. Enable network for the sandbox per call: `-c sandbox_workspace_write.network_access=true` (or the config key in `config.toml`); on resumed turns re-pass it (CX-124).
2. For steps that truly need unrestricted network/loopback, run with `danger-full-access` (removes sandbox protections).
3. Keep network-dependent work (installs, remote git) outside the sandboxed agent step in the Runner.

**Notes:**
Network-off default is the documented sandbox design (Limitation); the unclear part is the Windows loopback case and missing diagnostics of the effective policy. The Windows MXC/elevated backend in use by the Runner is still unestablished. Related: CX-027, CX-124, CX-160.

---

### CX-180: Windows stdio MCP servers fail to initialize since 0.168.0-alpha.7 (IOCP named-pipe read error 536 `ERROR_PIPE_LISTENING`): "connection closed: initialize response"

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/62671 (introduced by merged PR https://github.com/openai/codex/pull/59726) |
| **Reported** | 2026-11-20 |
| **Last Activity** | 2026-11-20 |
| **Confidence** | Unverified |
| **Orchestration Impact** | HIGH |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Windows x64: first failing 0.168.0-alpha.7 (alpha.6 works); stock 0.169.1, 0.170.0, 0.171.0 reported failing in local probes; single machine (Windows 11 Insider 26H2 build 26340) |
| **Latest Platform Version** | 0.170.0 (0.171.0 also released) |
| **Labels** | bug, windows-os, mcp, app |

**Summary:**
PR #59726 (merged 2026-11-05) moved Windows local stdio MCP I/O to IOCP-backed named pipes. The reporter shows the first read on the parent pipe endpoint is issued before the child end is opened and completes with `STATUS_PIPE_LISTENING` (Win32 536), so the child is launched but the handshake closes ("connection closed: initialize response") and the server never reaches ready. Reproduced with a minimal Node JSON-lines probe through `codex app-server` and with the standalone CLI on 0.169.1/0.170.0/0.171.0; all six local stdio servers in Desktop failed. A reporter-made patch (open the client end before wrapping the server handle) makes the probe pass 3/3; no maintainer response, no fix PR yet.

**Impact on Orchestration:**
MOSAIC attaches stdio MCP servers (e.g. `contact-user`) to Windows `codex exec` steps. If this reproduces on the MOSAIC machine, every such server fails to initialize: its tools are missing and HITL via `contact-user` is unavailable, while the step itself may keep running.

**Evidence:**
- Single reporter (one machine, Insider OS build) with a self-contained repro, adjacent-build bisect (alpha.6 vs alpha.7), root-cause trace and a working patch. No independent confirmation, no maintainer comment at capture time. Included as Unverified because it threatens a core pattern (MCP tool availability in Windows `codex exec`) on stock OpenAI infrastructure with concrete repro steps.
- Other Windows stdio MCP failures exist (#49820 remote MCP drops SystemRoot; #61877 codex_apps startup failure) but have different causes; not verified as the same.

**Workaround(s):**
1. Pin to 0.168.0-alpha.6 or earlier (reporter's bisect; impractical for the stable channel).
2. Reporter's source patch (unreleased). Nothing maintainer-confirmed.

**Notes:**
Smoke test on the MOSAIC Windows machine: run `codex exec` with the `contact-user` MCP server on 0.170.0 and confirm the tool list/call works. The reporter's machine runs a pre-release Windows build, so it may be environment-sensitive; downgrade confidence if nobody else reproduces it.

---

### CX-140: Windows `apply_patch` writes LF into CRLF files (mixed line endings), drops trailing newline / BOM, and can fail to match mixed-EOL spans

| Field | Value |
|-------|-------|
| **Classification** | Limitation |
| **Source** | https://github.com/openai/codex/issues/25048 (also https://github.com/openai/codex/issues/9914, https://github.com/openai/codex/issues/35789, https://github.com/openai/codex/issues/7640) |
| **Reported** | 2026-01-26 (#9914); 2026-05-29 (#25048) |
| **Last Activity** | 2026-09-20 (#25048); 2026-08-17 (#9914) |
| **Confidence** | Confirmed |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.88.0 (#9914); Codex App 26.521 to 26.715 and VS Code extension (#25048); no re-test on 0.170.0 and no fix PR seen; needs re-verification |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, tool-calls, windows-os (#9914: bug, windows-os, CLI, tool-calls) |

**Summary:**
`apply_patch` writes inserted/replaced lines with LF only and does not preserve the target file's line-ending style. On a Windows repo whose files are CRLF, every patched file ends up with mixed CRLF/LF (confirmed independently with `git ls-files --eol` showing `w/mixed` on 10 Java/C files; new files are `w/lf`). Commenters also report that the patch always strips the file's trailing newline, that BOM and EOL get broken, and that a later patch touching a span with mixed CRLF/LF sometimes fails to apply, after which the model falls back to slow, token-heavy Python edits. A textual `\r` or a raw `0D` byte in the patch payload does not produce CRLF. No maintainer comment; feature requests for a line-ending setting or `preserve` mode (#9914 proposal, #35789) are open. Classified Limitation because the tool has no EOL control and nobody from OpenAI has responded.

**Impact on Orchestration:**
A Runner step editing existing CRLF files on Windows leaves mixed line endings that show up as whole-file diffs, editor warnings, and failed or noisy follow-up patches; steps that verify by diff or checksum can mis-report. Repeated patch failure burns tokens through shell/Python fallback edits. Edits themselves succeed, so it is silent rather than a hard failure.

**Evidence:**
- #25048: 23 thumbs-up, 7 comments, two independent users with eol evidence (App 26.715, VS Code extension); #9914: 12 thumbs-up, 8 comments, CLI 0.88.0 mixed-CRLF failure and trailing-newline loss.
- One user says the only working mitigation is a system prompt telling Codex to convert files to LF before editing.

**Workaround(s):**
1. Keep files LF in the working tree (`.gitattributes` with `* text=auto eol=lf`, `core.autocrlf=false`) so patch output matches (users' reports; not tested by the reporters as a fix).
2. Instruct the agent (AGENTS.md / step prompt) to normalize line endings and restore the trailing newline after patching, or to convert to LF before editing (reported by a #25048 commenter).
3. Add a post-step normalization (`git add --renormalize` / unix2dos) in the workflow.

**Notes:**
Not re-verified on 0.170.0; last activity over 2 months ago, so flag as needs re-verification. Related: CX-100 (blank-context-line patch quirk), CX-103 (Move to), CX-106 (Delete File). #62327 (open, 2026-11-18) reports the streaming preview showing a different old line than the applied patch for `\r\r\n` endings; preview only, not tracked. Source check (run 2): `codex-rs/apply-patch/src/lib.rs` at rust-v0.170.0 contains no CRLF/BOM handling, so the behavior very likely persists on 0.170.0.

---

### CX-141: `apply_patch` fails "Failed to find expected lines" on the first line of UTF-8-BOM files (Windows editors); misleading error causes retry loops

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/27784 |
| **Reported** | 2026-06-12 |
| **Last Activity** | 2026-06-26 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | Codex App/CLI around June 2026; a commenter reproduced against `main` with the standalone `apply_patch` binary; no BOM handling found in `apply-patch/src/lib.rs` at rust-v0.170.0 (source check, not a runtime test) |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, tool-calls |

**Summary:**
When the target file starts with a UTF-8 BOM (EF BB BF, the default for Windows Notepad and some tool-generated .js/.json files), `apply_patch` Update File cannot match context that includes the first line: the file text is split on `\n` with the BOM still attached to line 1, so the first line never equals the patch context. The error ("Failed to find expected lines/context") does not mention the BOM. A commenter reproduced it with a minimal printf-created BOM file and the standalone binary, and identified `derive_new_contents_from_chunks` as the place; another commenter offers a BOM-detection patch. No maintainer comment, no fix PR seen. Only patches whose context or edit touches the first line are affected.

**Impact on Orchestration:**
A Runner step that edits the first line of a BOM-prefixed file (e.g. a PowerShell-generated JSON/JS/config file) fails repeatedly with a misleading error; the model retries or falls back to shell edits, burning tokens, and may rewrite the file without the BOM. Failure is loud, not silent.

**Evidence:**
- Independent reproduction by a second user with exact command and result; source-level cause given (BOM kept in line 1).
- Original report says the agent retried endlessly; only 2 comments, 0 reactions, no maintainer engagement (weak popularity).

**Workaround(s):**
1. Strip the BOM before the step (or make the agent do so via shell) and re-add it if required (inference from the cause; not reported as tested).
2. Avoid patch context that includes line 1, e.g. edit only later lines (inference).
3. Ensure workspace files are written as UTF-8 without BOM (cf. CX-006 for schema files).

**Notes:**
Needs re-verification on 0.170.0 (last activity June). Related: CX-140 (EOL handling in the same tool), CX-006 (BOM in `--output-schema`).

---

### CX-142: Windows `apply_patch` fails for large patches (CreateProcessAsUserW 206 / os error 206): patch body is passed through argv via `apply_patch.bat`

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/15003 (also https://github.com/openai/codex/issues/17107, https://github.com/openai/codex/issues/13574) |
| **Reported** | 2026-03-18 (#15003); 2026-04-08 (#17107) |
| **Last Activity** | 2026-04-14 (#15003); 2026-04-08 (#17107) |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | CLI 0.115 and 0.117.0 reported (Windows); no report on 0.170.0. Source check: `codex-rs/arg0/src/lib.rs` at rust-v0.170.0 still generates `apply_patch.bat` that runs `"{exe}" --codex-run-as-apply-patch %*`, so the argv transport still exists. Needs re-verification |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, windows-os, tool-calls (#17107 also sandbox) |

**Summary:**
On Windows, when `apply_patch` runs through the generated `apply_patch.bat` shim (shell-invoked `apply_patch`, heredoc style), the whole patch body travels as a command-line argument, so a large patch (reported at about 1000+ lines or about 100 KB) fails before any change is made with `Io(Os { code: 206, ... filename or extension too long })` or `windows sandbox: runner error: CreateProcessAsUserW failed: 206`. No file is written. Splitting into several smaller patches works, and models do that on their own, which multiplies token cost. No maintainer comment; #15003 has 16 thumbs-up plus 4 hearts and a source-level root cause. Whether the model's native apply_patch tool path (as opposed to the `.bat` shim) avoids this on current releases was not established: the later reports of the same error class (#31511 in a custom deny profile, different cause) show the 206 symptom still exists in 0.14x.

**Impact on Orchestration:**
A Runner step on Windows that asks for a whole-file generation or large rewrite in one patch can fail without any file change and then retry in fragments or via shell, consuming tokens; the loud failure may be recovered by the model or end the step.

**Evidence:**
- #15003: reproduction script (1200-line Add File via PowerShell `apply_patch $patch`) and code references; #17107: independent report at about 100 KB; #13574 cited as the related sandbox case.
- Reports are 7+ months old on 0.115/0.117; nothing on recent versions, so reproduction status on 0.170.0 is unknown.

**Workaround(s):**
1. Instruct the agent to keep each `apply_patch` call small (for example a few hundred lines) and split large files across several Add/Update calls (reported behavior of the model after failure).
2. Write large files through shell/PowerShell instead of `apply_patch` (inference).

**Notes:**
Needs re-verification on 0.170.0 with a 1200-line `apply_patch` call in `codex exec` on Windows (repro in #15003). Related: CX-140, CX-141, CX-029. #31511 (open; false `os error 206` from `apply_patch`/`view_image` only under a custom permission profile with `:root = "deny"` plus a nested non-glob `deny_read` folder; single reporter, narrow configuration) was reviewed and not tracked separately.

---

### CX-181: Optional stdio MCP server that becomes ready before turn 1 is still missing from `tool_search` on the first turn of `codex exec`; lazy stdio startup is now default

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/56102 (related: merged PR https://github.com/openai/codex/pull/62366 lazy stdio startup default; closed #62428) |
| **Reported** | 2026-10-22 |
| **Last Activity** | 2026-11-15 |
| **Confidence** | Unverified |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.159.3 (reporter); not re-tested on 0.170.0; PR #62366 (merged 2026-11-18) changes startup timing again, so current behavior is unknown |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, mcp, exec, CLI, app-server |

**Summary:**
When an optional stdio MCP server finishes `tools/list` before the first turn starts, `tool_search` still does not see its tools on turn 1 (the searchable catalog is not refreshed); they appear from turn 2. In `codex exec` the first-turn lookup missed in 6 of 8 runs (scripted model, server wrapped with `sleep 2`). Marking the server `required = true` avoided the miss (5/5) but makes startup fail if the server is down. Separately, PR #62366 (2026-11-18) made lazy stdio startup the default when a tool catalog is cached: optional stdio servers expose cached tools and start on the first call; required servers still connect at startup. No maintainer response on #56102.

**Impact on Orchestration:**
A Runner step whose first (often only) turn needs a tool from an optional MCP server (e.g. `contact-user`, when reached via `tool_search`/deferred tools) can see "no such tool" and the model may then stop searching, even though the server is up. Single-turn `exec` steps have no turn 2 to recover on. A commenter's synthetic replay measured that a model which says "tool not available" once often does not search again.

**Evidence:**
- Reporter repro with 8 `exec` runs and 7 app-server runs plus `rmcp=trace` capture; one commenter (2026-11-15) adds model-behavior data on the cost of a first-turn miss (third-party study). No maintainer comment, no fix PR.
- Unverified: single reporter with a scripted-model harness, stock OpenAI infrastructure; included because MCP tool availability in `exec` is a core MOSAIC pattern. Related to CX-120 (different mechanism: >100 deferred tools).

**Workaround(s):**
1. Mark critical MCP servers `required = true` (reporter: 5/5 no miss; startup fails if the server is unavailable, which is arguably the desired fail-closed behavior for `contact-user`).
2. Avoid depending on `tool_search` discovery for critical tools: keep the total MCP tool count low enough that tools are exposed directly (see CX-120).

**Notes:**
Verify on 0.170.0 with the real `contact-user` server: does the first-turn `exec` call see the tool? With lazy startup default (PR #62366), also confirm that an optional server whose catalog is not yet cached still works on the very first run (fresh CODEX_HOME per call would mean no cached catalog).

---

### CX-182: Premature auto-compaction: client adds an estimate of older encrypted reasoning on top of server-reported usage (token count used for compaction exceeds reported usage by 50-300k tokens)

| Field | Value |
|-------|-------|
| **Classification** | Bug |
| **Source** | https://github.com/openai/codex/issues/49961 (related per reporters: #39767, #49026, #45074; not read) |
| **Reported** | 2026-10-01 |
| **Last Activity** | 2026-11-19 |
| **Confidence** | Likely |
| **Orchestration Impact** | MEDIUM |
| **Reproduced at MOSAIC** | No |
| **MOSAIC Response** | Unevaluated |
| **Version(s) Affected** | 0.159.2 through 0.170.0 (11 comments, latest 2026-11-19 on core 0.170.0 via a custom Responses gateway); GPT-6.1 Sol and GPT-6 Astra on ChatGPT sign-in; CLI, Windows/macOS Desktop and WSL2 all reported |
| **Latest Platform Version** | 0.170.0 |
| **Labels** | bug, rate-limits, CLI, context |

**Summary:**
When the response does not carry the "reasoning included" signal (`server_reasoning_included == false`), the history accounting in `core/src/context_manager/history.rs` adds a client-side estimate of earlier encrypted reasoning items plus new tool output to the last server-reported total. The sum crosses `model_auto_compact_token_limit` / the effective window while reported usage is far below it (e.g. reported 193k vs internal 266k; 496k vs 811k; 287k vs 452k). Result: a mid-turn `run_auto_compact{reason=ContextLimit phase=MidTurn}` that takes 2-5 minutes and a large extra model request, while the displayed context percentage shows plenty of headroom. About ten independent reports with exact arithmetic reconstructions; whether the backend already counts historical reasoning (true double-counting) is not established. No maintainer response, no fix PR.

**Impact on Orchestration:**
A long-running `codex exec` step (many tool calls, retained reasoning) can compact earlier than expected, burning minutes and quota, and the compacted history loses detail mid-task. Percentage-based handoff rules inside prompts cannot be trusted because displayed and enforced budgets differ. Distinct from CX-040 (fixed tool-catalog overhead) and CX-041 (compaction that does not shrink the request), but the symptoms stack.

**Evidence:**
- Roughly ten independent reproductions across CLI 0.159.2/0.160.0/0.162.1, Desktop core 0.162.0-alpha.17.2 and 0.170.0, with log fields (`total_usage_tokens`, `token_limit_reached`) and exact formula matches to the source. 9 thumbs-up. One reporter observed the `X-Reasoning-Included` header absent on ChatGPT-backend responses.
- No maintainer comment; the issue's own bot flagged #49026 as a possible duplicate (not verified; reporter says it supplies a current repro of #39767/#49026).
- The 2026-11-19 comment uses a custom gateway, but the accounting path is client-side and matches the stock reports.

**Workaround(s):**
1. Raise `model_auto_compact_token_limit` (and `model_context_window` where valid) so the inflated count stays below the threshold; reporters call this a compensation, not a fix, and it shifts the real overflow point.
2. Keep exec steps short/scoped so they finish before reaching about 60% of the displayed window.
3. Community client patch (unofficial, single author): https://github.com/coinman-dev/codex-context-accounting-fix (reported compaction at 734k moved to 922k); not suitable for MOSAIC without review.

**Notes:**
`model_auto_compact_token_limit_scope = "body_after_prefix"` did not prevent it (the full-context guard still fired). Verify on 0.170.0 stock whether `X-Reasoning-Included` is now sent. The reporters detected compaction from the session rollout (`compacted` records) and logs; whether `exec --json` surfaces it was not established here.

---

