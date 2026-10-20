# Deployment Guide

How to deploy, update, and manage MOSAIC agent workspaces using `mosaic-deploy`.

This guide is for **project authors** who want to get MOSAIC running in their AI coding tool workspace. It covers the main operations, essential configuration, and common tasks. For exhaustive flag-by-flag detail, see `Tools/Deployment/docs/cli.md`.

---

## At a Glance

| Operation | What it does | Command |
|-----------|-------------|---------|
| **Deploy** | Create a new workspace from scratch | `mosaic-deploy deploy` |
| **Update** | Bring an existing workspace to latest versions | `mosaic-deploy update` |
| **Update workflows** | Replace the workflow set without touching agents or hooks | `mosaic-deploy workflows` |
| **Deploy agents** | Deploy a chosen mix of agents without touching workflows or hooks | `mosaic-deploy agents` |
| **Deploy hooks** | Deploy hook bundles alone | `mosaic-deploy hooks` |
| **Transform** | Convert deployed agents from one harness to another | `mosaic-deploy transform` |
| **Promote** | Turn a harness-only agent into a reusable generic source | `mosaic-deploy promote` |

**Two ways to run:** Launch without a subcommand for the interactive TUI, or pass a subcommand for scriptable CLI mode.

**Built-in harnesses:** `claude-code`, `opencode`, `ghcp-cli`, `vscode-ghcp`.

**Catalog stacks:** every subcommand and the TUI can deploy from several catalogs layered on top of each other (`--catalog-folder`, `--include-default-catalog`). See [Catalog Stacks](#catalog-stacks).

---

## Deploy — New Workspace

Creates a workspace from scratch: selects harness, workflows, utility agents, and model assignments, then writes all files.

**Interactive:**

```sh
./mosaic-deploy
```

The TUI walks you through every step.

**CLI:**

```sh
./mosaic-deploy deploy \
  --harness claude-code \
  --workspace /path/to/my-project \
  --selections selections.yaml \
  --auto-confirm
```

**Selections file (`selections.yaml`):**

```yaml
workflows:
  - quick-fix
  - build
utility_agents:
  - code-reviewer
tier_models:
  HIGH: claude-opus-4-5
  LOW: claude-haiku-3-5
```

Add `--dry-run` to preview the plan without writing anything.

---

## Update — Existing Workspace

Compares deployed versions against the catalog, shows what's stale, and applies updates — preserving your `project` and `custom` region content.

```sh
./mosaic-deploy update \
  --harness claude-code \
  --workspace /path/to/my-project \
  --auto-confirm
```

### Catalog stack on update

Without stack flags, `update` reuses the catalog stack persisted with the workspace (or the default catalog alone when none is persisted), and the output starts with a "Catalog stack (persisted)" block. Stack flags replace the persisted stack; they are never merged with it. To go back to the default catalog alone, run `update --include-default-catalog`. See [Persistence on update and redeploy](#persistence-on-update-and-redeploy). Update also compares protocol and bundle versions against the file supplied by the stack, not against the default catalog.

### Orchestrator declaration refresh

Update also refreshes catalog-backed infrastructure agent declarations in the orchestrator when their version, class, triggers or on-failure changed in the catalog. It never adds new declarations and never touches hand-added ones. An orchestrator with current declarations is left unchanged.

### Conflict resolution

When a file was locally modified since the last deploy:

| `--conflict` value | What happens |
|-------------------|-------------|
| `skip` (default) | File untouched; added to TODO checklist |
| `overwrite` | Replaced with latest (local edits lost) |
| `backup` | Local file copied to `<name>.backup`, then overwritten |

---

## Update Workflows — Replace the Workflow Set

Rewrites only the orchestrator file with a new set of workflows. No agent, skill, or hook file is touched, even if stale. The selected workflow set **fully replaces** the currently deployed set — workflows you don't reselect are removed.

Use this to add, remove, or swap workflows without disturbing the rest of the deployment. If the new workflows require agents that aren't deployed yet, those agents are deployed automatically in the same run.

Each rebuilt workflow routing table gains a first-column `Row` (1..N), added by mosaic-deploy; authors never write it and catalog sources do not contain it. Only a rebuild adds it: injecting infrastructure agents into an existing orchestrator keeps its workflows unchanged, and a version stamp does not trigger it. An orchestrator deployed before this column existed therefore gains it the next time its workflows are redeployed, for example with `mosaic-deploy workflows`. The Runner accepts tables with or without the column, but refuses a table whose `Row` numbers do not match row positions, for example after a hand edit.

**Interactive:**

```sh
./mosaic-deploy
# → Select "Update workflows"
```

The TUI marks currently deployed workflows so you can see what you're keeping.

**CLI:**

```sh
./mosaic-deploy workflows \
  --harness claude-code \
  --workspace /path/to/my-project \
  --workflows "kb-generation,brownfield-tdd" \
  --auto-confirm
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID |
| `--workspace <path>` | string | — | Absolute path to the existing workspace |
| `--workflows <ids>` | string | — | Comma-separated workflow IDs (replaces the current set); absent = ask interactively |
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle a locally-modified orchestrator file |
| `--dry-run` | bool | false | Plan without writing any files |
| `--auto-confirm` | bool | false | Skip plan review |
| `--output <json>` | string | — | Machine-readable output format |

---

## Deploy Agents — Add Individual Agents

Deploys a chosen mix of subagents, utility agents, infrastructure agents, and standalone agents plus the skills they require. No workflow or hook work is performed.

If an orchestrator file (`orchestrator.md`, `orchestrator-script.md`) is already present, declarations for newly deployed infrastructure agents are added to it. Existing declarations, including hand-added ones, are kept, and a re-deployed agent is refreshed in place. No extra question is asked. A locally modified orchestrator goes through the normal conflict choice (`--conflict` in the CLI); if skipped, a TODO entry explains the manual step. No orchestrator is created when none is present, and its workflows are not changed.

Use this to add or update individual agents across any catalog category without disturbing workflows or hooks.

**Interactive:**

```sh
./mosaic-deploy
# → Select "Deploy agents"
```

The TUI presents a browsable list grouped by catalog category (Subagents, Utility Agents, Standalone Agents, etc.), mirroring the folder structure in the catalog.

> **Not every utility agent belongs in a workspace.** `subagent-creator` and `workflow-creator` write catalog source files and depend on the design documents in the MOSAIC repository, so they are meant to run in the MOSAIC repository root, where they ship ready to use. This list still offers them, but deployed into a project workspace they cannot reach their references. See `Catalog/UtilityAgents/README.md`.

**CLI:**

```sh
./mosaic-deploy agents \
  --harness claude-code \
  --workspace /path/to/my-project \
  --utility "injections-helper,code-reviewer" \
  --yes
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID (required) |
| `--workspace <path>` | string | — | Workspace directory (required) |
| `--subagents <ids>` | string | — | Comma-separated ordinary subagent IDs; absent = ask; empty = none |
| `--utility <ids>` | string | — | Comma-separated utility agent IDs; absent = ask; empty = none |
| `--infra <ids>` | string | — | Comma-separated infrastructure agent IDs; absent = ask; empty = none |
| `--standalone <ids>` | string | — | Comma-separated standalone agent IDs; absent = ask; empty = none |
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle locally-modified files |
| `--dry-run` | bool | false | Plan without writing any files |
| `--yes` | bool | false | Auto-confirm the deployment plan |
| `--output <json>` | string | — | Machine-readable output format |

When any per-class flag (`--subagents`, `--utility`, `--infra`, `--standalone`) is given, the interactive question is skipped entirely and unspecified classes are treated as empty.

---

## Deploy Hooks — Add Hook Bundles

Deploys hook bundles alone. No agent, model, workflow, or orchestrator work is performed.

**Interactive:**

```sh
./mosaic-deploy
# → Select "Deploy hooks"
```

**CLI:**

```sh
./mosaic-deploy hooks \
  --harness claude-code \
  --workspace /path/to/my-project \
  --hooks "pre-commit" \
  --yes
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--harness <id>` | string | — | Harness ID (required) |
| `--workspace <path>` | string | — | Workspace directory (required) |
| `--hooks <ids>` | string | — | Comma-separated hook bundle IDs; absent = ask; empty = none |
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle locally-modified files |
| `--dry-run` | bool | false | Plan without writing any files |
| `--yes` | bool | false | Auto-confirm the deployment plan |
| `--output <json>` | string | — | Machine-readable output format |

---

## Transform — Cross-Harness Conversion

Converts already-deployed agent files from one harness to another.

```sh
# Preview what would happen
./mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --path /path/to/agents/ \
  --dry-run

# Execute the transformation
./mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --path /path/to/agents/ \
  --overwrite
```

Map models between harnesses with `--model-map`:

```sh
--model-map "claude-opus-4-5=gpt-4o" --model-map "claude-haiku-3-5=gpt-4o-mini"
```

---

## Promote — Generic from Harness-Only

Turns a harness-only agent (one with no generic source) into a reusable generic agent in the catalog.

```sh
./mosaic-deploy promote \
  --file /path/to/harness-only-agent.md \
  --category Research
```

The source file is never modified. The generated generic has empty `project` and `managed` regions, ready for the normal deploy pipeline.

With a catalog stack of two or more layers, promote also needs to know which layer receives a **new** agent; see [Promote and layers](#promote-and-layers).

---

## Catalog Stacks

By default `mosaic-deploy` deploys from the MOSAIC default catalog (`<mosaic-root>/Catalog`). A **catalog stack** lets you deploy from several catalogs at once: your own catalogs layered on top of the default one (or on top of each other). Use it to add or replace agents, workflows, skills and hooks for your team without editing the default catalog.

### What a stack is

- A stack is an ordered list of catalog folders, **highest layer first**. A higher layer takes precedence over every layer below it.
- Every layer is a catalog root with a `catalog-identity.yaml` file (see the identity section of `Catalog/CatalogFilesFormat.md`). The identity gives the catalog an id, a name, a version and, optionally, the overrides it declares on purpose.
- The default catalog (`mosaic-default`) is one possible layer. It is always the **bottom** layer when present. It is a layer like any other: it can be left out, and a stack may consist of the default layer alone (this is what you get when you give no stack input and none is persisted).
- Catalog ids must be unique within one stack.

### What each layer contributes

Items are resolved one at a time. The highest layer that has an item supplies it; items with the same key in lower layers are *shadowed*.

| Item | Resolved by | Unit that is replaced |
|------|-------------|-----------------------|
| Agents (every role) | agent key | the whole agent file |
| Workflows | workflow id | the whole workflow |
| Skills, hook bundles | folder key | the whole folder; files are never merged across layers |
| Orchestrator pair (`orchestrator` + `orchestrator-script`) | one item | both files come from the same layer: the highest layer that supplies the pair |
| `CommunicationProtocol.md`, `DeployedSections.md` | file | the whole file from the highest layer that has it |
| Harness injections | harness directory (`HarnessInjections/<Harness>/`) | all injection files of that harness come from the highest layer that has the directory |

There is **no fallback** inside a winning unit. If a higher layer supplies `HarnessInjections/Claude Code/`, every injection file for that harness comes from it, even files it lacks; an empty directory still wins. A layer that supplies only one orchestrator file is a stack error. If no layer supplies the protocol, the bundle, or a built-in harness's injections, the operation fails before anything is written.

Only the four built-in harnesses resolve their injections through the stack. Descriptor-only and external harnesses (under `<mosaic-root>/MosaicDeploy/harnesses/`) keep loading their injections from next to their descriptors, whichever stack is in effect.

There is no longer a merge of the default catalog's skills and hooks into a custom catalog. Skills and hooks come only from the layers of the stack, so a custom catalog that relies on default skills or hooks needs the default layer in its stack.

### Setting up a stack in the TUI

In the interactive TUI, a **stack screen** follows the path screen in every mode (deploy, update, update workflows, deploy agents, deploy hooks, transform, promote). It lists the layers highest first, with the default layer always shown as the bottom row.

| Key | Action |
|-----|--------|
| `a` | Add a catalog folder by path |
| `d` | Remove the user layer under the cursor |
| Shift+Up / Shift+Down | Move the user layer under the cursor up or down in precedence |
| Space (on the default row) | Keep the default layer in the stack, or leave it out |
| Enter | Confirm the stack |
| Esc | Go back to the path screen |

Enter is refused while the stack would have no layer at all. A problem with the confirmed stack (for example a layer without identity, a duplicate catalog id, or a harness whose injections no layer supplies) is shown on the screen, and you stay there to fix it.

The screen starts from:

1. the stack the TUI was launched with, if you passed `--catalog-folder` or `--include-default-catalog` (for example `mosaic-deploy --catalog-folder /my/catalog`), otherwise
2. the stack persisted with the workspace you just entered (see [Persistence](#persistence-on-update-and-redeploy)): in deploy, update, workflows, agents and hooks modes when the path is itself a workspace; in promote and transform modes the workspace that contains the file or folder, otherwise
3. the default layer alone.

If the persisted stack can no longer be used (for example a layer folder moved), the screen shows the reason and you must change the stack before confirming.

**Migration notice.** When the TUI starts with a user catalog, the stack screen shows this notice: *the behaviour of custom catalogues has changed. Until now a custom catalogue got no default agents or workflows. Your folder is now a layer above the default catalogue, which is offered as the bottom row. Keep it to include the default agents and workflows (space on the default row); leave it out to use only your own catalogues.* See [Migrating from `--catalog-folder`](#migrating-from---catalog-folder).

### Setting up a stack on the command line

Two global flags, accepted by every subcommand and by the TUI launch:

| Flag | Meaning |
|------|---------|
| `--catalog-folder <dir>` | Add a user layer. Repeat for several layers; the **first** occurrence is the highest layer. |
| `--include-default-catalog` | Add the default catalog as the bottom layer, below every `--catalog-folder`. Given alone, it means the default catalog alone. `--include-default-catalog=false` is also accepted; the flag never consumes the next argument. |

Two rules matter:

- A CLI run that names a user catalog **leaves the default layer out** unless you also pass `--include-default-catalog`.
- `--include-default-catalog` **alone** selects the default catalog alone.

```sh
# Your catalog on top of the default catalog
mosaic-deploy deploy --catalog-folder /my/catalog --include-default-catalog \
  --harness claude-code --workspace /path/to/my-project

# Two user layers (team over company), no default layer
mosaic-deploy update --catalog-folder /team/catalog --catalog-folder /company/catalog \
  --harness claude-code --workspace /path/to/my-project

# Default catalog alone (see Persistence)
mosaic-deploy update --include-default-catalog \
  --harness claude-code --workspace /path/to/my-project
```

`--include-default-catalog=false` with no `--catalog-folder` would be an empty stack and is an error.

### Declared overrides and the conflict report

A higher layer that supplies an item a lower layer also supplies *overrides* it. That is often intended, so each catalog can declare its overrides in `catalog-identity.yaml` under `overrides:` (agents, workflows, skills, hooks, harness injections, system files, and `orchestrator_pair: true` for the orchestrator pair). The pair is declared once, with that single flag; listing `orchestrator` or `orchestrator-script` under `agents` is rejected.

Every run that writes (deploy, update, workflows, agents, hooks) and `render` builds a **conflict report** with one line per shadowed item:

```
warning: agent:code-reviewer from team-catalog 1.2 shadows mosaic-default 0.7.0; affects workflows: feature-dev
info: orchestrator-pair from team-catalog 1.2 shadows mosaic-default 0.7.0; affects workflows: feature-dev, quick-fix
```

| Severity | Meaning |
|----------|---------|
| `info` | The winning catalog declares this override |
| `warning` | The override is not declared |

For an agent, the line names the workflows that use it among the workflows selected in this run plus those already deployed in the workspace's orchestrator. For the orchestrator pair, it names all of those workflows. Other item kinds name no workflows. Workflows are read from the deployed orchestrator files, not from the manifest.

The report never changes the outcome or the exit code. In the CLI it is printed before the result (human output) or carried in the JSON document (`ConflictReport` in the run summary, `conflictReport` in `render`). In `--auto-confirm` runs each warning also becomes a "Manual steps" entry in the TODO file. In the TUI the plan review screen shows the report.

### Stack errors

A stack that cannot be built stops the run before anything is written (CLI: message on stderr, exit code 1, empty stdout). The error names the layer:

| Error | Cause |
|-------|-------|
| catalog stack has no layers | No user layer and the default layer excluded |
| layer invalid | The default layer is not last, appears twice, or a layer path is empty or not absolute |
| layer failed to load | The folder is missing, is not a directory, or a source file is invalid |
| layer has no valid identity | `catalog-identity.yaml` is missing or has a missing or invalid `id`, `name`, `version`, or override |
| two catalog layers share a catalog id | Duplicate ids in one stack |
| layer supplies only one orchestrator file | `orchestrator.md` without `orchestrator-script.md`, or the reverse |
| numeric agent id used by two different agents | Two *resolved* agents have the same numeric `id` (see below) |

A **numeric-id collision is a composition error**: the numeric agent ids of the resolved agents must be unique across the whole stack. A shadowed agent does not count, because the agent with the same key in a higher layer replaces it. The error names the id, both agent keys and both catalogs. For example, the AgentTest catalog uses ids 1-25 and therefore cannot be stacked with the default catalog.

If no layer supplies the orchestrator pair, the run fails when the plan is built with an error that lists the layers searched.

### Checking the catalogs: `check-index`

`mosaic-deploy check-index` checks each layer's workflow `Index.md` against that layer's workflow files. Output has one block per catalog, headed `catalog <id> (<root>)`, in layer order, each listing only that catalog's findings. The exit code is `0` when every layer is clean and `2` when any layer has drift. `check-index` never reuses a persisted stack; it checks the stack given by its flags (or the default catalog alone).

### Provenance

The tool records which catalog supplied what:

- **Manifest** (`.mosaic/manifest.yaml`): every entry (agents including both orchestrator files, skills, hooks) carries a `catalog` field with the supplying catalog's id, name and version. The top level records `protocol_source` and `bundle_source` (written by deploy and update, kept by the partial runs) and the persisted `catalog_stack`.
- **Workflows have no manifest entries.** They are embedded in the deployed orchestrator file, which has its own entry naming the orchestrator pair's source catalog. Workflow provenance is therefore per resolved item in the stack, not per manifest entry.
- **TUI**: the workflow, agent, utility, standalone, infrastructure and hook selection screens label each option with the id of the catalog it comes from (`unknown` when it cannot be told).

Manifests written before stacks carry no provenance; that is not an error.

### Persistence on update and redeploy

A run that writes (deploy, update, workflows, agents, hooks) and was given stack input stores that stack in the workspace manifest (`catalog_stack`, highest first; the default layer as an entry without a path; user paths absolute). Later runs for the same workspace reuse it:

| Situation | Stack used | Stack-in-effect origin |
|-----------|-----------|------------------------|
| Stack flags given (CLI), or the stack was changed on the stack screen (TUI) | exactly that stack, **never merged** with the persisted one; it replaces the persisted stack on the next write | `stack-input` |
| No stack flags, workspace has a persisted stack | the persisted stack | `persisted` |
| No stack flags, no persisted stack | the default layer alone | `default` |

Where the workspace comes from:

| Subcommand | Workspace used for reuse |
|------------|-------------------------|
| `deploy`, `update`, `workflows`, `agents`, `hooks`, `render` | `--workspace` (`render --dest` without `--workspace` has none) |
| `promote` | the workspace containing `--file` |
| `transform` | the workspace containing `--path` |
| `check-index` | none; it never reuses |

`promote` and `transform` reuse the persisted stack but never persist anything. `render` never persists either.

A persisted stack is per workspace and per machine: user layers are stored by absolute path. If a stored layer's folder moved or its catalog id changed, the run stops with an error naming the stored path before anything is written. Fix it by passing new stack flags, which replace the persisted stack. A manifest whose stack list is malformed is treated the same way and is never repaired automatically.

**Returning to a default-only stack.** Run any write subcommand with `--include-default-catalog` and no `--catalog-folder`:

```sh
mosaic-deploy update --include-default-catalog --harness claude-code --workspace /path/to/my-project
```

This replaces the persisted stack with the default catalog alone. (Giving no stack flags keeps the persisted stack.) In the TUI, remove the user layers on the stack screen and keep the default row.

### Seeing the stack in effect (CLI)

Every catalog-loading subcommand reports the stack it used.

- **Human output**: a block at the start of the output when the stack came from stack input or the persisted stack (nothing for the default alone):

  ```
  Catalog stack (persisted):
    1. team-catalog Team Catalog 1.2 /team/catalog
    2. mosaic-default MOSAIC Default Catalog 0.7.0 /path/to/MOSAIC/Catalog (default)
  ```

  The layers are listed in precedence order, highest first, with the id, name, version and root of each. The block also appears for `--dry-run` and a declined plan.
- **`--output json`**: one extra top-level key in the command's own document, for all three origins: `StackInEffect` for `deploy`, `update`, `workflows`, `agents`, `hooks`; `stackInEffect` for `render`, `promote`, `transform`, `check-index`. Its value is `{"layers":[{"identity":{"id","name","version"},"root","default"}],"origin":"stack-input|persisted|default"}`. Existing keys are unchanged.

The stack in effect never changes an exit code.

### Promote and layers

`promote` writes a new generic agent into one layer:

- An agent whose key already resolves in the stack is promoted into the layer that supplies it.
- A **new** agent in a stack of two or more layers needs a target layer: pass `--target-catalog <catalog id>` on the CLI (without it the command fails and writes nothing), or choose a layer when the TUI asks. The default layer is a valid target.
- With one layer there is nothing to choose.

The category list comes from the target layer's `Subagents/` folder. The new agent's numeric id is one greater than the largest id across *all* agents of the stack, so it never collides with another layer.

```sh
mosaic-deploy promote --file /path/to/agent.md --harness claude-code \
  --category Research --catalog-folder /my/catalog --include-default-catalog \
  --target-catalog my-catalog
```

### Migrating from `--catalog-folder`

Until now `--catalog-folder` replaced the default catalog's agents and workflows, and the default catalog's skills and hooks were merged in. This has changed:

- Custom-catalog users who until now got no default agents or workflows are now **offered the default catalog as the bottom layer**. In the TUI it is the bottom row of the stack screen; keep it with Space to include the default agents and workflows.
- CLI runs that name a user catalog **still leave the default catalog out** unless you pass `--include-default-catalog`, so existing scripts behave as before.
- The skills/hooks merge is gone (see above); the protocol, bundle and harness injections now also come from the stack, so a custom catalog must supply its own copies if the default layer is left out (the shipped test catalogs do).
- The given stack is now persisted with the workspace, so later `update` runs without flags reuse it.
- `--catalog-folder` can be repeated.

---

## Configuration

On first run, the tool creates a `MosaicDeploy/` directory next to the binary with default config files, logs, and an optional external harnesses folder. Two config files live in `MosaicDeploy/config/`:

### tool-config.yaml — Project-level (commit this)

```yaml
schema_version: "1"
utility_agent_allow_list:
  - code-reviewer
  - documentation-writer
allow_external_modules: false
log_retention_runs: 50
```

| Field | What it controls |
|-------|-----------------|
| `utility_agent_allow_list` | Which utility agents are selectable. `[]` = none. File absent = `[injections-helper]` only; the Deploy agents flow still offers every utility agent on disk. |
| `allow_external_modules` | Whether external harness modules in `MosaicDeploy/harnesses/` are allowed |
| `log_retention_runs` | Max history log entries (`0` = keep all) |
| `tool_destinations` | Permanent generic-tool to harness-tool mappings (silences repeated prompts) |

### user-config.yaml — Per-user (do NOT commit)

Auto-written when you confirm selections. Stores your tier-to-model mappings and custom model IDs so repeated runs are faster.

```yaml
schema_version: "1"
tier_models:
  claude-code:
    HIGH: claude-opus-4-5
    LOW: claude-haiku-3-5
custom_model_ids:
  claude-code:
    - claude-sonnet-4-5-20250929
```

> **Warning:** The tool rewrites this file on each run. YAML comments you add will not survive.

### Where the catalog stack is kept

Neither config file holds the catalog stack. It is stored per workspace in `.mosaic/manifest.yaml` (`catalog_stack`), written by the run that was given stack input and reused by later runs for that workspace. Each catalog's own settings (id, name, version, declared overrides) live in `catalog-identity.yaml` at the catalog root. See [Catalog Stacks](#catalog-stacks).

---

## The TODO File

After every deploy or update, the tool writes `MOSAIC-DEPLOYMENT-TODO-<timestamp>.md` to the workspace root. It lists:

- **Unfilled project regions** — slots you should fill for best agent performance
- **Skipped files** — locally modified files left unchanged
- **Unmapped tools** — generic tools without harness equivalents
- **Hook registration steps** — manual steps to complete

Work through this checklist before using the workspace. Filling project regions is the single highest-impact post-deployment action. See the [Agent Customization Guide](AgentCustomizationGuide.md) for how.

---

## Logs

| File | Content |
|------|---------|
| `MosaicDeploy/logs/latest.log` | Full log of the most recent run |
| `MosaicDeploy/logs/history.log` | Summary of all runs, newest first |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Unrecoverable error |
| 2 | Completed with skips (check TODO file) |
| 3 | Invalid arguments |

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| "not a MOSAIC repository" | Run from inside the MOSAIC repo, or pass `--mosaic-root /path/to/MOSAIC` |
| "harness not found" | Check the ID (`claude-code`, `opencode`, `ghcp-cli`, `vscode-ghcp`). For custom descriptors, pass the full file path. |
| "external harness modules require explicit opt-in" | Set `allow_external_modules: true` in `tool-config.yaml`, or pass `--allow-external` |
| Config file not picked up | The tool looks in `MosaicDeploy/config/` under the MOSAIC root. Verify `--mosaic-root` if you moved the binary. |
| "catalog stack has no layers" | You gave `--include-default-catalog=false` without `--catalog-folder`. Add a layer or drop the flag. |
| "catalog layer ... has no valid identity" | The folder given to `--catalog-folder` has no valid `catalog-identity.yaml` at its root. See the identity section of `Catalog/CatalogFilesFormat.md`. |
| "two catalog layers share a catalog id" | Two layers declare the same `id`. Give each catalog a unique id. |
| "numeric agent id used by two different agents" | Two agents that both win in the stack share a numeric `id`. Renumber one, or leave the clashing catalog (for example the AgentTest catalog) out of the stack. |
| "catalog layer supplies only one orchestrator file" | A layer has `orchestrator.md` or `orchestrator-script.md` but not both. Supply both or neither. |
| "no catalog layer supplies ..." | No layer has `CommunicationProtocol.md`, `DeployedSections.md` or the harness's `HarnessInjections/<Harness>/`. A catalog used without the default layer must carry its own copies. |
| "persisted catalog layer no longer resolves" | A layer stored with the workspace moved or changed id. Pass new stack flags (they replace the persisted stack), or `--include-default-catalog` alone to return to the default catalog. |
| My custom catalog's agents are missing the default ones | A CLI run that names a user catalog leaves the default catalog out. Add `--include-default-catalog`. |

---

## Quick Reference

| I want to... | Do this |
|--------------|---------|
| Deploy MOSAIC to a new project | `mosaic-deploy deploy --harness claude-code --workspace /path` |
| Update agents to latest versions | `mosaic-deploy update --harness claude-code --workspace /path` |
| Add or swap workflows in an existing workspace | `mosaic-deploy workflows --harness claude-code --workspace /path` |
| Add specific agents (declarations added to an existing orchestrator) | `mosaic-deploy agents --harness claude-code --workspace /path` |
| Add hook bundles only | `mosaic-deploy hooks --harness claude-code --workspace /path` |
| Preview without writing files | Add `--dry-run` to any command |
| Keep my local edits as backups during update | `--conflict backup` |
| Convert agents to a different harness | `mosaic-deploy transform --harness X --target-harness Y --path /path` |
| Turn a one-off agent into a reusable generic | `mosaic-deploy promote --file /path/to/agent.md --category Category` |
| Deploy from my catalog on top of the default one | Add `--catalog-folder /my/catalog --include-default-catalog` |
| Layer several catalogs | Repeat `--catalog-folder`; the first is the highest layer |
| Return a workspace to the default catalog alone | `mosaic-deploy update --include-default-catalog --harness X --workspace /path` |
| See which catalogs a run used | Read the "Catalog stack" block, or `StackInEffect` / `stackInEffect` in `--output json` |
| Check every layer's workflow index | `mosaic-deploy check-index` (add the stack flags) |
| Promote a new agent into a chosen layer | `mosaic-deploy promote ... --target-catalog <catalog id>` |
| Use the interactive wizard | Just run `mosaic-deploy` with no subcommand |
| Script deployments for CI | Use `--auto-confirm --output json` |
| Control which utility agents are offered | Edit `utility_agent_allow_list` in `tool-config.yaml` |
| Check what happened in the last run | Read `MosaicDeploy/logs/latest.log` |
