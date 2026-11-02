# mosaic-deploy CLI Reference

The `mosaic-deploy` tool provides two modes: an interactive TUI (when no subcommand is given and a terminal is attached) and a non-interactive CLI (when a subcommand is provided). This document covers the CLI.

## Design decision: multi-valued selections via a file

Multi-valued selections — workflows, utility agents, hooks, and per-tier model mappings — are expressed via a **YAML selections file** supplied with `--selections <path>` rather than as repeated flags.

**Rationale:** Repeated flags grow unmanageable for batch multi-workspace scripting. A selections file is stable, shareable, and version-controllable: a team can commit one canonical `selections.yaml` per project and pass it to every invocation. Repeated flags, by contrast, produce long and fragile shell one-liners. Single-valued flags (`--harness`, `--workspace`) remain as flags for ergonomics on one-off invocations.

---

## Global flags

These flags are accepted by all subcommands.

| Flag | Type | Description |
|------|------|-------------|
| `--verbose` | bool | Enable verbose progress output |
| `--mosaic-root <path>` | string | Override the MOSAIC root directory |
| `--allow-external` | bool | Enable external harness modules (opt-in for security) |
| `--catalog-folder <dir>` | string (repeatable) | Add a user catalog as a stack layer. The first occurrence is the highest layer. See [Catalog stack flags](#catalog-stack-flags) |
| `--include-default-catalog` | bool | Add the default catalog (`<mosaic-root>/Catalog`) as the bottom layer. Alone, it selects the default catalog alone |

---

## Catalog stack flags

The two stack flags are persistent: every subcommand (`deploy`, `update`, `workflows`, `agents`, `hooks`, `render`, `promote`, `transform`, `check-index`) accepts them, and so does the TUI launch. The stack is decided from the raw arguments before any subcommand runs.

| Input | Stack |
|-------|-------|
| `--catalog-folder A --catalog-folder B` | A over B; the default catalog is **not** included |
| `--catalog-folder A --include-default-catalog` | A over the default catalog |
| `--include-default-catalog` alone | the default catalog alone |
| `--include-default-catalog=false` without `--catalog-folder` | empty stack: error |
| no stack flag | the workspace's persisted stack, else the default catalog alone |

Forms: `--catalog-folder <dir>` and `--catalog-folder=<dir>`; values are unquoted and made absolute against the working directory. `--include-default-catalog` is accepted bare (true) or with `=<bool>`; the last valid occurrence wins and the next argument is never taken as its value.

A stack flag (any occurrence of either) is *stack input*: the resulting stack is used as given and never merged with the stack persisted in the workspace. The write subcommands (`deploy`, `update`, `workflows`, `agents`, `hooks`) store it in the workspace manifest. Without stack input, a subcommand reuses the persisted stack of its workspace:

| Subcommand | Workspace used for reuse |
|------------|-------------------------|
| `deploy`, `update`, `workflows`, `agents`, `hooks`, `render` | `--workspace` (`render --dest` alone has none) |
| `promote` | nearest ancestor of `--file` holding `.mosaic/manifest.yaml` |
| `transform` | `--workspace` |
| `check-index` | none: never reuses |

`promote`, `transform` and `render` reuse a persisted stack but never persist one. To return a workspace to the default catalog alone, run a write subcommand with `--include-default-catalog` and no `--catalog-folder`.

Stack errors (empty stack, missing or invalid catalog identity, duplicate catalog id, incomplete orchestrator pair, numeric-id collision, a persisted layer that no longer resolves or a malformed persisted stack) are printed to stderr and the process exits with code 1 before anything is written. In `--output json` mode stdout stays empty.

### Stack in effect

Every catalog-loading subcommand reports the stack it used.

- Human output: a leading block `Catalog stack (<origin>):` with one line per layer, highest first, as `  <n>. <id> <name> <version> <root>[ (default)]`. It is written only when `<origin>` is `stack-input` or `persisted`; a default-only run keeps its output unchanged.
- JSON output (all three origins `stack-input`, `persisted`, `default`): one extra top-level key in the subcommand's own document, merged into its existing object: `StackInEffect` in the run summary of `deploy`, `update`, `workflows`, `agents`, `hooks`; `stackInEffect` in the documents of `render`, `promote`, `transform`, `check-index`. Value: `{"layers":[{"identity":{"id","name","version"},"root":"...","default":false}],"origin":"stack-input"}`. Existing keys are unchanged. Error envelopes carry no such key.

### Conflict report

When a layer shadows an item of a lower layer, the run reports it:

- Human output: one line per entry before the result, `<severity>: <kind>[:<key>] from <id> <version> shadows <id> <version>[, ...][; affects workflows: a, b]`. `info` means the winning catalog declared the override in `catalog-identity.yaml`; `warning` means it did not. The orchestrator pair is the kind `orchestrator-pair` (declared once with `orchestrator_pair: true`).
- JSON output: the report is inside the document only: `ConflictReport` (`{"lines":[...]}`, always present, `{}` when empty) in the run summary, `conflictReport` in the `render` document (absent when nothing is shadowed). Each line has `item`, `winner`, `shadowed` (highest first), `severity` and `affectedWorkflows`.
- Harness injections (kind `harness-injections`) are reported only for the harness the run deploys to; shadows of another harness's injection directory are left out of the lines, the warnings and the TODO entries. A descriptor-only or external harness reads no stack injections, so its runs report none.
- Under `--auto-confirm` (or `--yes`) each `warning` line also adds a "Manual steps" entry to the TODO file.
- The report never changes the outcome or the exit code. In JSON mode stdout is exactly one JSON document.

---

## `deploy` subcommand

Deploy MOSAIC agents and skills into a new workspace, or re-deploy into an existing one.

```
mosaic-deploy deploy [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID to deploy (e.g. `claude-code`) |
| `--workspace <path>` | string | — | Absolute path to the target workspace |
| `--conflict <skip\|overwrite\|backup>` | string | — | How to handle files the tool cannot confirm as its own |
| `--selections <path>` | string | — | Path to a YAML selections file |
| `--workflows <ids>` | string | — | Comma-separated workflow IDs; absent = use selections file; empty string = explicitly none |
| `--infrastructure <ids>` | string | — | Comma-separated infrastructure agent IDs; absent = ask interactively; empty string = explicitly none |
| `--output <json>` | string | — | Machine-readable output format |
| `--dry-run` | bool | false | Plan without writing any files |
| `--auto-confirm` | bool | false | Skip plan review and proceed automatically |
| `--strict-models` | bool | false | Fail the run (exit 1) when any agent would be deployed without a resolved model, instead of recording a TODO gap. Applies to real and `--dry-run` runs; the check precedes review and any write |

### Selections file format

```yaml
# selections.yaml
workflows:
  - quick-fix
  - build
utility_agents:
  - code-reviewer
hooks:
  - pre-commit
tier_models:
  HIGH: claude-opus-4
  LOW: claude-haiku-3
```

Unknown keys in the selections file are silently ignored (forward-compatible).

For `ghcp-cli` and `vscode-ghcp`, deploy refuses a workspace that already holds the other GHCP target and points to `transform`; both write agents as `X.agent.md` and migrate agents recorded as `X.md` (see "One GHCP target per workspace" and "GHCP agent file names" under `transform`).

Re-running `deploy` on an existing workspace rewrites stale agents and keeps their project and custom regions and tool choices. A model the run supplies replaces the deployed one; otherwise the deployed model is kept. The selected workflows replace the deployed set. With no infrastructure selection, existing declarations are kept.

---

## `update` subcommand

Update an existing workspace deployment to the latest agent versions, including already-deployed hook bundles.

```
mosaic-deploy update [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID |
| `--workspace <path>` | string | — | Absolute path to the existing workspace |
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle files the tool cannot confirm as its own |
| `--output <json>` | string | — | Machine-readable output format |
| `--dry-run` | bool | false | Plan without writing any files |
| `--auto-confirm` | bool | false | Skip plan review |

`--workspace` must exist; a missing path is an error and nothing is written.

The `--conflict` flag controls what happens when the tool cannot confirm a file as its own: it cannot be parsed, or it has no manifest record:
- `skip` (default): leave the file unchanged and record a TODO item
- `overwrite`: replace the file with the latest version
- `backup`: copy the file to `.mosaic/backups/<path>.<timestamp>.bak`, then overwrite

**What an update keeps.** Only project and custom regions, models and tool choices survive an update. Edits elsewhere in an agent or skill file are replaced the next time the file is rewritten, and `--conflict` does not protect them. Keep your own content in project and custom regions.

### Orchestrator declaration refresh

Update also refreshes catalog-backed infrastructure agent declarations in the orchestrator (`orchestrator.md`, `orchestrator-script.md`) when their version, class, triggers or on-failure changed in the catalog. It never adds new declarations and never touches hand-added ones. An orchestrator with current declarations is left unchanged.

### Hook refresh

Update also refreshes hook bundles that are already deployed in the workspace (files of the harness's hook variant found on disk) when the Catalog version is newer than the version recorded in the manifest. Up-to-date hooks are reported unchanged and not rewritten. It never adds a hook that is not deployed (use `deploy` or the deploy-hooks mode for that).

- Refreshed hook files are overwritten with the Catalog version (local edits are lost); files the new version no longer has are left in place.
- An existing registration target (e.g. `.claude/settings.json`, `.github/hooks/mosaic-logger.json`) is never modified; a TODO with the registration fragment is reported for every refreshed hook whose target exists, whether or not the fragment changed. A missing target is created. The TODO is emitted when the update is applied, so the plan review and `--dry-run` show none. Up-to-date hooks and hooks skipped in a conflict produce no registration TODO.
- A deployed hook with no manifest record is treated as a conflict: `--conflict skip` (default) leaves it as is; `overwrite` and `backup` refresh it.
- Both GHCP harnesses keep hooks in `.github/hooks/`, and `claude-code` in `.claude/hooks/`; Update refreshes the hook folder of the harness it runs for.

### GHCP workspaces

A workspace holds one GHCP target at a time (decided by the manifest `harness_id`); Update for the other one is refused and points to `transform`. Update also migrates agents the manifest records as `X.md` to `X.agent.md`, with backups under `.mosaic/backups/` (see "GHCP agent file names" under `transform`).

---

## `transform` subcommand

Move the whole deployment of a workspace from one harness to another.

```
mosaic-deploy transform [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | - | Source harness ID (required) |
| `--target-harness <id>` | string | - | Target harness ID (required) |
| `--workspace <path>` | string | - | Workspace root whose deployment is moved (required) |
| `--target-model <id>` | string | - | Default target model for source models not covered by `--model-map`; absent means those agents' model field is left empty |
| `--model-map <src=tgt>` | string (repeatable) | - | Map a source model to a target model as `sourceModel=targetModel`; repeat once per source model. Use an empty source (`=targetModel`) for agents with no model set. Unmapped source models fall back to `--target-model` |
| `--overwrite` | bool | false | Overwrite an existing destination file |
| `--dry-run` | bool | false | Show the plan and outcomes without writing any file |
| `--yes` | bool | false | Confirm the transform plan without prompting |
| `--output <json>` | string | - | Machine-readable output format |

`--workspace` is quote-stripped via the shared path-input helper, so shell-quoted paths (e.g. from drag-and-drop) work without manual editing.

### Input and enumeration

The input is the workspace root, as for `deploy`. A single agent file or a loose folder of agent files is not accepted. Transform plans the whole deployment found in the workspace (agents, skills and hook bundles), shows the plan, and writes the target harness's copies only after the plan is confirmed (`--yes` confirms without prompting; `--dry-run` shows the plan and writes nothing).

Agents are read from the source harness's agents folder and written to the target harness's agents folder (e.g. `.claude/agents/foo.md` becomes `.opencode/agents/foo.opencode.md`). The summary names the agents folder it looked in.

### What is kept and what comes from the target

Transform never upgrades the deployment. From the source it keeps unchanged: agent bodies, stamps, protocol and bundle regions, workflow tables, infrastructure declarations, and project and custom regions and skills. From the target it takes tool names, the frontmatter shape, role-dependent fields and the regions the harness fills in.

Hooks are deployed from the target harness's catalog variant for the bundles present in the source. A bundle is not carried when the target cannot carry it. Because of this, injections and hooks in the target may be newer than the source. `update` is the explicit way to upgrade a deployment.

For every pair except the GHCP pair below, source files and source manifest entries are never changed. The target deployment is recorded in the manifest. Agents written into `vscode-ghcp` are named `X.agent.md`, like the `ghcp-cli` agents.

### GHCP pair

`ghcp-cli` ("GHCP CLI / VS Code GHCP Copilot") and `vscode-ghcp` ("VS Code GHCP Local") share `.github/agents` and `.github/hooks`, and a workspace holds one of them at a time. Transform between the two (either direction) therefore replaces the source deployment instead of adding the target beside it:

- Agents recorded in the manifest are rewritten in place as `.github/agents/X.agent.md` with the target's content. An agent recorded as a legacy `X.md` is migrated: the target version is written as `X.agent.md`, the old file is removed with its manifest entry, and an existing `X.agent.md` is replaced. Files in the folder that the manifest does not record are left alone.
- Target hooks are written; source hook files and registrations the target does not write are removed, and target hook files and registrations that already exist are replaced.
- Skills are left as they are.
- Every removed or replaced file is first backed up under `.mosaic/backups/`.
- `--dry-run` and the confirmation plan list the removals and replacements.
- The run is atomic on failure: if a write fails, the workspace is left as it was.

A workspace that holds both GHCP targets is refused by Transform and by every deploy flow (see below); clean it up by hand first.

### One GHCP target per workspace

The manifest `harness_id` decides which GHCP target a workspace holds. `deploy`, `update` and the other deploy flows for `ghcp-cli` or `vscode-ghcp` refuse to run in a workspace that holds the other one, change nothing, and point to `mosaic-deploy transform --harness <held> --target-harness <requested> --workspace <path>`. A workspace that holds both must have one of them removed by hand (its agent files in `.github/agents` and its hook files in `.github/hooks`).

### GHCP agent file names

Both GHCP targets write agents as `.github/agents/X.agent.md`, with content specific to each target. A workspace deployed earlier with `X.md` agents is migrated by the next full `deploy`, `agents` deploy, `update` or workflow update for its target: each old file is backed up under `.mosaic/backups/` and replaced by `X.agent.md`, and an existing `X.agent.md` for the same agent is backed up and replaced too. Only agents recorded as `X.md` in the manifest are migrated. `render` for `vscode-ghcp` also uses `X.agent.md`.

### Models

Each source model is mapped through `--model-map`, else `--target-model`, else asked interactively. A model that stays unresolved is omitted: the agent is written without a `model` key and is named in the summary and in the TODO file.

### Per-item outcomes

Each planned item (agent, skill or hook bundle) is processed independently and listed with one of three statuses:

| Status | Meaning |
|--------|---------|
| `transformed` | Written at `destinationPath` (or computed, under `--dry-run`) |
| `skipped` | Not written; the reason says why (not a MOSAIC agent, destination already exists, hook bundle not carried, and so on) |
| `failed` | The item could not be transformed or written |

An existing destination is skipped and kept unless `--overwrite` is given. A failed item does not stop the others. Between `ghcp-cli` and `vscode-ghcp` the plan always replaces the existing destination whatever `--overwrite` says, and a failure leaves the workspace unchanged (see "GHCP pair" above).

Esc at a question cancels the run: nothing is written.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Everything transformed; nothing to follow up |
| 1 | An item failed, or an unrecoverable error (unresolvable harness, unreadable workspace, cancellation, plan not confirmed) |
| 2 | Completed, but something needs follow-up: agents without a model, tools without a target equivalent, or hook bundles not carried. Removals and replacements of the GHCP pair are reported in the plan and do not by themselves give exit code 2 |
| 3 | Invalid arguments (missing required flag or malformed `--model-map`) |

### JSON output

With `--output json`, a single document is written to stdout containing `sourceHarnessId`, `targetHarnessId`, `workspace`, `sourceAgentsDir`, `dryRun`, `cancelled`, `targetModel`, `appliedModelMap`, `unresolvedModels`, the per-item `files` array, `notes`, `todoPath`, and the counts `transformed`, `skipped` and `failed`, plus `stackInEffect`. A document is also written when the run was cancelled.

For the GHCP pair, the document also carries two arrays, each absent when empty: `removals` (files the run removes, or would remove under `--dry-run`) and `replacements` (existing files the run overwrites after backing them up). Each entry has `kind` (`agent` or `hook`), `key` (the agent or hook key, when there is one), `path` (workspace-relative, forward slashes), `reason`, and `backupPath` (the absolute backup file; set only after a real run that backed the file up). The human output lists the same entries under `removals:` and `replacements:` and ends the summary line with `removed: N` and `replaced: N`.

### Example

```sh
# Dry-run to preview the plan: which items would be written and where.
mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --workspace /path/to/my-project \
  --dry-run

# Move the deployment, confirming the plan and replacing existing destination files.
mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --workspace /path/to/my-project \
  --yes \
  --overwrite \
  --output json
```

---

## `agents` subcommand

Deploy a chosen mix of subagents, utility agents, infrastructure agents, and standalone agents plus the skills they require. No workflow or hook work is performed.

```
mosaic-deploy agents [flags]
```

### When to use this subcommand

Use `agents` when you want to add or refresh individual agents in an existing workspace without touching workflow or hook configuration. It writes only the files the selected agents require. Infrastructure-agent declarations injected into an existing orchestrator leave its deployed workflow tables unchanged; the workflow `Row` column is added only when workflows are rebuilt (a full deploy or `mosaic-deploy workflows`).

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID (required) |
| `--workspace <path>` | string | — | Absolute path to the target workspace (required) |
| `--subagents <ids>` | string | — | Comma-separated ordinary subagent IDs; absent = ask; empty = none |
| `--utility <ids>` | string | — | Comma-separated utility agent IDs; absent = ask; empty = none |
| `--infra <ids>` | string | — | Comma-separated infrastructure agent IDs; absent = ask; empty = none |
| `--standalone <ids>` | string | — | Comma-separated standalone agent IDs; absent = ask; empty = none |
| `--conflict <skip\|overwrite\|backup>` | string | — | Default decision for files the tool cannot confirm as its own |
| `--output <json>` | string | — | Machine-readable output format |
| `--dry-run` | bool | false | Compute and report without writing any file |
| `--yes` | bool | false | Auto-confirm the deployment plan without prompting |

`--workspace` is quote-stripped via the shared path-input helper, so shell-quoted paths (e.g. from drag-and-drop) work without manual editing.

### Per-class pre-answer convention

If any of the four per-class flags (`--subagents`, `--utility`, `--infra`, `--standalone`) is given, the interactive agent selection is skipped entirely and the classes not given are treated as explicitly empty. If none is given, the tool presents a single browsable list and asks.

### Orchestrator declarations

If an orchestrator file (`orchestrator.md`, `orchestrator-script.md`) is already present in the workspace, declarations for the newly deployed infrastructure agents are added to it. Existing declarations, including hand-added ones, are kept, and a re-deployed agent is refreshed in place, keeping its project and custom regions, models and tool choices. No extra question is asked. An orchestrator the tool cannot confirm as its own follows `--conflict`; when it is skipped, a TODO entry explains the manual step. No orchestrator is created when none is present, and the workflows in the orchestrator are not changed.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | All selected agents deployed successfully |
| 1 | An unrecoverable error occurred |
| 2 | Run completed but some items were skipped; see the TODO file |
| 3 | Invalid arguments (missing required flag or unknown flag value) |

### JSON output

With `--output json`, a single `RunSummary` JSON document is written to stdout containing at least `Mode`, `Outcome`, and `WorkspacePath`. The exit code is unchanged. A document is written even when `Outcome` is `"failed"`.

### Examples

```sh
# Interactive: ask which agents to deploy.
mosaic-deploy agents \
  --harness claude-code \
  --workspace /path/to/my-project

# Non-interactive: pre-answer the selection; unspecified classes are empty.
mosaic-deploy agents \
  --harness claude-code \
  --workspace /path/to/my-project \
  --utility "code-reviewer,test-runner" \
  --infra "docker-manager" \
  --yes

# Dry run - compute the plan without writing anything.
mosaic-deploy agents \
  --harness claude-code \
  --workspace /path/to/my-project \
  --subagents "code-reviewer" \
  --conflict backup \
  --dry-run \
  --output json
```

---

## `promote` subcommand

Turn a harness-only agent into a generic catalog source. Flags: `--file`, `--harness`, `--category`, `--overwrite`, `--dry-run`, `--output` (see the README for their meaning), plus:

| Flag | Type | Description |
|------|------|-------------|
| `--target-catalog <catalog id>` | string | Catalog id of the stack layer that receives a **new** agent. Required when promoting a new agent non-interactively into a stack of two or more layers |

Layer targeting:

| Case | Target layer |
|------|--------------|
| The agent key already resolves in the stack | the layer that supplies it; `--target-catalog` is not needed (an id naming no layer is rejected) |
| New agent, stack of two or more layers, `--target-catalog` given | the layer with that id (the default layer included); an unknown id is an error |
| New agent, stack of two or more layers, flag absent | the TUI asks; the CLI fails and writes nothing |
| New agent, one layer | that layer; the flag may be absent or that layer's id |

The category comes from the target layer's `Subagents/` folder. The new numeric agent id is one greater than the largest id across all agents of the stack. `--output json` adds `targetCatalogId` to the result (and the usual `stackInEffect` key). `DestinationPath` is relative to the MOSAIC root when the target layer lies inside it, otherwise absolute.

---

## `check-index` subcommand

Report drift between each stack layer's workflow `Index.md` and the workflow files on disk.

```
mosaic-deploy check-index [--output json] [stack flags]
```

`check-index` never reuses a persisted stack: it checks the stack its flags describe, or the default catalog alone.

Human output has one block per catalog in layer order, headed `catalog <id> (<root>)` (`catalog unknown (<root>)` for a root without identity), listing only that catalog's findings (`no index`, `clean`, `file-orphans`, `index-orphans`). JSON output keeps the existing top-level fields as an aggregate over all layers (counts summed, orphan lists concatenated highest layer first, `clean` true only when every layer is clean) and adds `catalogs`, one entry per layer with `catalog` (`id`, `name`, `version`), `catalogRoot`, `indexPath`, `indexPresent`, `fileOrphans`, `indexOrphans`, `diskCount`, `indexCount` and `clean`, plus `stackInEffect`.

Exit code: `0` when every layer is clean (a missing index counts as clean), `2` when any layer has drift, `1` on a service error.

---

## Exit codes

| Code | Name | Meaning |
|------|------|---------|
| 0 | success | All items deployed successfully |
| 1 | failure | An unrecoverable error occurred |
| 2 | completed-with-skips | Run completed but some items were skipped; see TODO file |
| 3 | usage | Invalid arguments or unrecognised subcommand; the error is printed to stderr |

With `deploy --strict-models`, an agent left without a resolved model fails the run with exit 1 and the stderr line `error: strict models: N agent(s) have no resolved model: <key> (tier <tier>), ...` (`<none>` when the agent declares no tier). Under `--output json` stdout then holds one failure document instead of a `RunSummary`:

```json
{
  "error": true,
  "reason": "unresolved-models",
  "message": "strict models: 1 agent(s) have no resolved model: orchestrator (tier TEST_SUBJECT)",
  "detail": "orchestrator (tier TEST_SUBJECT)",
  "exitCode": 1
}
```

Without the flag the gap goes to the TODO list as before.

---

## Machine-readable output (`--output json`)

When `--output json` is set, `mosaic-deploy` writes a single JSON document to stdout containing the full `RunSummary`. The exit code is unchanged. With `deploy --strict-models` and an unresolved model, a failure document (see Exit codes) replaces the `RunSummary`. Scripting consumers should read from stdout and parse JSON; diagnostic messages go to stderr.

```sh
result=$(mosaic-deploy deploy --harness claude-code --workspace /path --auto-confirm --output json)
echo "$result" | jq '.Outcome'
```

The JSON object always contains at least `Mode`, `Outcome`, and `WorkspacePath`. A JSON document is written even when `Outcome` is `"failed"`, so consumers can inspect the failure reason programmatically.

Two keys are additive: `ConflictReport` (the [conflict report](#conflict-report)) and `StackInEffect` (the [stack in effect](#stack-in-effect)). Both appear inside the one document; stdout never carries anything else.

---

## Unresolvable questions

When a question cannot be answered from the provided flags or the selections file, the CLI **skips the item and records a TODO entry** rather than blocking or applying a silent default. The run completes with exit code 2 (`completed-with-skips`). The TODO file written to the workspace root lists every item that needs manual attention.

This skip behavior is specific to the non-interactive CLI. The CLI never cancels a run mid-selection — it has no cancel keypress and never produces the abort-the-whole-run outcome that pressing Esc on a selection screen produces in the interactive TUI. A TUI user who wants to proceed with nothing selected for a given category presses Skip on that screen; a CLI user achieves the same outcome by omitting the selection from their `--selections` file. Both paths let the run continue with an empty selection and a TODO gap.

---

## Batch update example

The following shell script updates all workspaces listed in a file, using a shared selections file. It collects the JSON output from each run for post-processing.

```sh
#!/usr/bin/env bash
# batch-update.sh: update every workspace in workspaces.txt
# Usage: ./batch-update.sh workspaces.txt selections.yaml

set -euo pipefail

WORKSPACES_FILE="${1:?usage: batch-update.sh <workspaces.txt> <selections.yaml>}"
SELECTIONS="${2:?usage: batch-update.sh <workspaces.txt> <selections.yaml>}"
RESULTS_DIR="update-results-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RESULTS_DIR"

while IFS= read -r workspace; do
  [[ -z "$workspace" || "$workspace" == \#* ]] && continue

  slug="${workspace//\//_}"
  echo "Updating: $workspace"

  mosaic-deploy update \
    --harness claude-code \
    --workspace "$workspace" \
    --conflict skip \
    --auto-confirm \
    --output json \
    > "$RESULTS_DIR/${slug}.json" || true

  outcome=$(jq -r '.Outcome' "$RESULTS_DIR/${slug}.json" 2>/dev/null || echo "unknown")
  echo "  outcome: $outcome"

done < "$WORKSPACES_FILE"

echo ""
echo "Results written to: $RESULTS_DIR/"
echo "Failures:"
grep -l '"failed"' "$RESULTS_DIR"/*.json 2>/dev/null || echo "  (none)"
```

Run it with:

```sh
chmod +x batch-update.sh
./batch-update.sh workspaces.txt selections.yaml
```

Where `workspaces.txt` contains one workspace path per line (blank lines and `#` comments ignored), and `selections.yaml` contains the shared workflow and model selections.
