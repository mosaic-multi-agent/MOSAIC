# Deployment Guide

How to deploy, update, and manage MOSAIC agent workspaces using `mosaic-deploy`.

This guide is for **project authors** who want to get MOSAIC running in their AI coding tool workspace. It covers the main operations, essential configuration, and common tasks. For exhaustive flag-by-flag detail, see `Tools/Deployment/docs/cli.md`.

---

## At a Glance

| Operation | What it does | Command |
|-----------|-------------|---------|
| **Deploy** | Create a new workspace from scratch | `mosaic-deploy deploy` |
| **Update** | Bring an existing workspace to latest versions | `mosaic-deploy update` |
| **Change workflows** | Replace the workflow set; deploys missing agents and skills the new workflows need, leaves other deployed agents, skills and hooks unchanged | `mosaic-deploy workflows` |
| **Deploy agents** | Deploy a chosen mix of agents without touching workflows or hooks | `mosaic-deploy agents` |
| **Deploy hooks** | Deploy hook bundles alone | `mosaic-deploy hooks` |
| **Transform** | Move a workspace's whole deployment to another harness | `mosaic-deploy transform` |
| **Promote** | Turn a harness-only agent into a reusable generic source | `mosaic-deploy promote` |

**Two ways to run:** Launch without a subcommand for the interactive TUI, or pass a subcommand for scriptable CLI mode. In the TUI, any key on a final screen returns to the mode screen with the previous mode and harness highlighted (after a declined plan, with choices cleared; scroll keys scroll a scrollable summary); Esc on the mode screen exits, and Ctrl+C exits at once.

**Built-in harnesses:** `claude-code` (Claude Code), `opencode` (OpenCode), `ghcp-cli` ("GHCP CLI / VS Code GHCP Copilot"), `vscode-ghcp` ("VS Code GHCP Local"), `codex` (experimental, see [Codex (experimental)](#codex-experimental)), `opencode-v2` (OpenCode V2, experimental, see [OpenCode V1 vs V2](#opencode-v1-vs-v2-opencode-v2-is-experimental)).

### Choosing a GHCP target

- **GHCP CLI / VS Code GHCP Copilot** (`ghcp-cli`): use it for the GitHub Copilot terminal CLI and for VS Code's default chat harness (the Copilot harness).
- **VS Code GHCP Local** (`vscode-ghcp`): use it only for VS Code's Local harness. Local is being retired; no date has been announced.
- To tell which harness a VS Code chat uses, look below the prompt input in the chat window: the `Set session target` control on the left switches between Local and Copilot, next to the agent mode and permissions controls.
- Subagents on the GHCP CLI / VS Code GHCP Copilot target have no built-in ask tool, as with Claude Code background subagents. Human-in-the-loop for subagents there therefore needs an MCP ask tool that you supply. This setup is untested.
- A workspace holds one GHCP target at a time, decided by the harness recorded on the manifest entries (file names never decide it). Deploy and the other deploy flows refuse the other target and point to Transform; a workspace that holds both must be cleaned up by hand.

### Codex (experimental)

The `codex` harness is experimental. The picker marks it "Experimental", plan review shows a warning, and every writing command prints the same warning once.

- **What works:** agents deploy as standalone TOML files in `.codex/agents/*.toml`, with skills in the shared `.agents/skills` root. Update and Transform work too.
- **How workflows run:** through `mosaic-run` only (see the [Runner Guide](RunnerGuide.md#codex)). Codex's native multi-agent dispatch is not usable for MOSAIC, so native orchestration inside Codex is not supported. Codex runs produce no MOSAIC log, and `mosaic-agent-test` does not support Codex.
- **Capabilities:** a `web_search` tool becomes web search `live` (otherwise `disabled`). `web_fetch` becomes network access for write-capable agents; a read-only agent cannot get it and the plan reports the withheld `web_fetch`. `terminal` alone grants no network access.
- **Minimum Codex version:** 0.169.0.

Codex and OpenCode V2 are the experimental harnesses. The agent-format layer behind them is stable.

### OpenCode V1 vs V2 (`opencode-v2` is experimental)

OpenCode V1 (`opencode`) and OpenCode V2 (`opencode-v2`) are two separate deploy targets. The `opencode-v2` harness is experimental: everything is supported, but the harness is not yet tested in real use. The picker marks it "Experimental", plan review shows a warning, and every writing command prints the same warning once.

- **One OpenCode target per workspace.** Both targets write to the same places under `.opencode/` (`agents`, `skills`, `plugins`), so a workspace holds one of them at a time. The manifest entries, not the paths, decide which one it is. Deploy and the other deploy flows refuse to add the other target before anything is asked or written, and the refusal names the Transform command to use. A workspace that holds both must be cleaned up by hand (remove the files of one target from `.opencode/` and run again).
- **Transform works both ways.** `mosaic-deploy transform --harness opencode --target-harness opencode-v2 --workspace <path>` moves a V1 workspace to V2, and the same command with the two ids swapped moves it back. As with the GHCP targets, the target replaces the source in place: a recorded agent is overwritten, skills are carried over, and source plugin files the target does not write are removed (every replaced or removed file is backed up under `.mosaic/backups/` first). Only files the source's manifest entries record are touched; everything else is left alone. See [Transform between the GHCP targets](#transform-between-the-ghcp-targets) for how removals and replacements show in the plan.
- **No `opencode.json`.** Deploy does not write or change any OpenCode configuration file for V2. Agents are `.opencode/agents/*.md` files with `mode: primary` or `mode: subagent`. To start a workflow in the V2 TUI, you select the orchestrator agent yourself.
- **Permissions are deny by default.** The agent's `permissions` frontmatter is an ordered rule list that starts by denying everything, then allows exactly the tools the agent declares. Declaring file write or file edit grants all three V2 file-writing tools, `write`, `edit` and `patch`, because V2 treats them as one edit action and the model picks the tool (GPT models use `patch`). Access outside the workspace is not denied but asks. A tool an agent does not declare is denied.
- **Hooks.** The mosaic-logger bundle deploys its `opencode-v2` variant to `.opencode/plugins`. See the [logger README](../Catalog/Hooks/mosaic-logger/README.md) and the variant's [`LIMITATIONS.md`](../Catalog/Hooks/mosaic-logger/opencode-v2/LIMITATIONS.md) for what it logs and what it cannot.
- **Human-in-the-loop for headless runs needs your own MCP server.** V2's `question` tool is auto-cancelled in headless (`mosaic-run`) runs, so it is no way to reach you there. Headless HITL on `opencode-v2` needs an ask-the-user MCP server that you supply and configure in OpenCode. Because Deploy denies every tool an agent does not declare, you also add a custom tool mapping so that agents declare the ask tool and it is allowed. Without the mapping the tool is denied. In `MosaicDeploy/config/tool-config.yaml` (or `user-config.yaml`), for an MCP server named `contact-user`:

  ```yaml
  tool_destinations:
    opencode-v2:
      - generic: ask_user
        destinations:
          - to: main
            names: ["contact-user_*"]
  ```

  Then list `ask_user` in the tools of the agents that may ask, and redeploy or update. See [Configuration](#configuration) and `Tools/Deployment/docs/configuration.md` for the mapping syntax. In native mode (the V2 TUI) the built-in `question` tool works for the orchestrator and subagents.

Before you install OpenCode V2, read the warnings in [Getting Started](GettingStarted.md#opencode-v2-warnings): one OpenCode version per machine, installation through npm, and where a subagent's question appears.

### GHCP agent file names

Both GHCP targets write agents as `.github/agents/X.agent.md`, with content specific to each target. A workspace deployed earlier with `X.md` agents is migrated by the next full deploy, deploy agents, Update or workflow update for its target, for the agents that run writes: each old file is backed up under `.mosaic/backups/` and replaced by `X.agent.md`, and an existing `X.agent.md` for the same agent is backed up and replaced too. A migrated agent keeps its project and custom regions, model, tool choices and declarations. A run that writes only some agents leaves the rest as `X.md`, and the next Update migrates them. Only agents recorded as `X.md` in the manifest are migrated. The rename never forces a write by itself, a file in conflict is kept as it is, and a transform to a GHCP target leaves no `X.md` entry in the manifest.

**Catalog stacks:** every subcommand and the TUI can deploy from several catalogs layered on top of each other (`--catalog-folder`, `--include-default-catalog` on the CLI; the stack screen in the TUI). See [Catalog Stacks](#catalog-stacks).

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
| Back up files the tool cannot confirm as its own before replacing them | `--conflict backup` |
| Move a deployment to a different harness | `mosaic-deploy transform --harness X --target-harness Y --workspace /path` |
| Turn a one-off agent into a reusable generic | `mosaic-deploy promote --file /path/to/agent.md --category Category` |
| Deploy from my catalog on top of the default one | Add `--catalog-folder /my/catalog --include-default-catalog` |
| Layer several catalogs | Repeat `--catalog-folder`; the first is the highest layer |
| Return a workspace to the default catalog alone | `mosaic-deploy update --include-default-catalog --harness X --workspace /path` |
| See which catalogs a run used | Read the "Catalog stack" block, or `StackInEffect` / `stackInEffect` in `--output json` |
| Check every layer's workflow index | `mosaic-deploy check-index` (add the stack flags) |
| Promote a new agent into a chosen layer | `mosaic-deploy promote ... --target-catalog <catalog id>` |
| Promote several harness-only agents at once | TUI only: run `mosaic-deploy`, choose Promote, tick the agents |
| Use the interactive wizard | Just run `mosaic-deploy` with no subcommand |
| Script deployments for CI | Use `--auto-confirm --output json` |
| Control which utility agents are offered | Edit `utility_agent_allow_list` in `tool-config.yaml` |
| Check what happened in the last run | Read `MosaicDeploy/logs/latest.log` |

---

## Deploy — New Workspace

Creates a workspace from scratch: selects harness, workflows, utility agents, and model assignments, then writes all files.

**Interactive:**

```sh
./mosaic-deploy
```

The TUI takes no flags. It starts at the mode screen and walks you through every step. Deploy workspace, Deploy agents, Deploy hooks and Promote ask the harness, then the path, then the stack. Update workspace, Change workflows and Transform harness ask the path first, then the harness, then the stack; the harness screen offers only the harnesses recorded in the manifest at that path (a single usable one is selected automatically, and a missing, empty, corrupt or newer-schema manifest shows the full list; unusable recorded harnesses are disabled with their reason). Transform asks the target harness right after the source harness. Esc on a later entry screen goes back one screen keeping your choices.

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
  - brownfield-tdd
  - build
utility_agents:
  - code-reviewer
tier_models:
  HIGH: claude-opus-4-5
  LOW: claude-haiku-3-5
```

Add `--dry-run` to preview the plan without writing anything.

Re-running Deploy on an existing workspace rewrites stale agents and keeps their `project` and `custom` regions and tool choices. A model the run supplies replaces the deployed one; otherwise the deployed model is kept. The selected workflows replace the deployed set. With no infrastructure selection, existing declarations are kept.

---

## Update — Existing Workspace

Compares deployed versions against the catalog, shows what's stale, and applies updates — preserving your `project` and `custom` region content. For a GHCP workspace, Update refuses the other GHCP target (pointing to Transform) and migrates `X.md` agents to `X.agent.md` (see [Choosing a GHCP target](#choosing-a-ghcp-target)).

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

When the tool cannot confirm a file as its own, because it cannot be parsed or has no manifest record:

| `--conflict` value | What happens |
|-------------------|-------------|
| `skip` (default) | File untouched; added to TODO checklist |
| `overwrite` | Replaced with latest |
| `backup` | Copied to `.mosaic/backups/<path>.<timestamp>.bak`, then overwritten |

**What an update keeps.** Project and custom regions, the frontmatter, models and tool choices belong to you and survive an update. In the tools field, built-in tools follow the catalog (the plan lists those an update adds or removes), tools you added are kept, and custom tools the run produces are added.

**Edits to Deploy-owned content.** The agent body outside the project and custom regions, and whole skill and hook files, are Deploy-owned. When one differs from what Deploy recorded at its last write, the plan shows an update with the reason "Deploy-owned content was edited locally" and the current file is backed up under `.mosaic/backups/` before it is replaced. Files written by an earlier Deploy version carry no such record and get no edit check until their next write. The plan lists every reason for an item, not only the first.

### The manifest

`.mosaic/manifest.yaml` (schema version 2) holds several harnesses side by side. Every entry names its harness; each harness has a section with `protocol_source`, `bundle_source` and `updated_at`; `tool_version` is recorded once at the top level; there is no top-level `harness_id`. A hook bundle is recorded file by file, one entry per deployed hook file. Older Deploy builds refuse a schema-2 manifest.

Before every action the manifest is reconciled with the disk: an entry whose file is gone is dropped, and a hook bundle with some files missing is completed by the next update. The plan review and the dry run list the dropped entries.

A manifest from an earlier Deploy version is converted on its first read. Entries under `.claude/` and `.opencode/` are attributed to Claude Code and OpenCode (V1) by their path (OpenCode V2 did not exist then; from schema 2 on, the entry's harness decides between `opencode` and `opencode-v2`, never the path); other entries (including `.github/`) take the old `harness_id` when that harness deploys to the path; entries that cannot be attributed are dropped and reported, and their files count as unrecorded, so the first run may ask the conflict question once for them.

---

## Change Workflows — Replace the Workflow Set

In the TUI this mode is called **Change workflows**; the CLI command is `workflows`. It rewrites the orchestrator files (`orchestrator.md`, and `orchestrator-script.md` when it is deployed) with a new set of workflows. Other deployed agents, skills and hooks are left unchanged, even if stale. The selected workflow set **fully replaces** the currently deployed set — workflows you don't reselect are removed.

Use this to add, remove, or swap workflows without disturbing the rest of the deployment. If the new workflows require agents that aren't deployed yet, those agents are deployed automatically in the same run.

Each rebuilt workflow routing table gains a first-column `Row` (1..N), added by mosaic-deploy; authors never write it and catalog sources do not contain it. Only a rebuild adds it: injecting infrastructure agents into an existing orchestrator keeps its workflows unchanged, and a version stamp does not trigger it. An orchestrator deployed before this column existed therefore gains it the next time its workflows are redeployed, for example with `mosaic-deploy workflows`. The Runner accepts tables with or without the column, but refuses a table whose `Row` numbers do not match row positions, for example after a hand edit.

**Interactive:**

```sh
./mosaic-deploy
# → Select "Change workflows"
```

The TUI marks currently deployed workflows so you can see what you're keeping. Deployed custom workflows, which the catalog stack does not supply, are listed with the hint `not in catalog; deployed only`. Like deployed catalog workflows they start unchecked: confirming without ticking one removes it, and the run summary names it. Tick the custom workflows you want to keep.

**Kept custom workflows.** A custom workflow you keep stays in the orchestrator exactly as deployed. It is not updated or validated, and the run warns `<id> is not in the catalog stack; kept as deployed, not updated or validated.` `Update workspace` keeps every deployed custom workflow the same way. With `--workflows`, an id the catalog stack does not supply is accepted when it names a deployed custom workflow; any other unsupplied id is an unknown-workflow error. These two flows do not show the removed-layer hint for unknown ids; `deploy` still does.

**Same-line workflow tags.** If a deployed orchestrator file has a line that begins with a workflow tag and carries more content on that line, the update for that file fails and nothing is written. Put the tag on a line of its own.

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
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle an orchestrator file the tool cannot confirm as its own |
| `--dry-run` | bool | false | Plan without writing any files |
| `--auto-confirm` | bool | false | Skip plan review |
| `--output <json>` | string | — | Machine-readable output format |

---

## Deploy Agents — Add Individual Agents

Deploys a chosen mix of subagents, utility agents, infrastructure agents, and standalone agents plus the skills they require. No workflow or hook work is performed.

If an orchestrator file (`orchestrator.md`, `orchestrator-script.md`) is already present, declarations for newly deployed infrastructure agents are added to it. Existing declarations, including hand-added ones, are kept, and a re-deployed agent is refreshed in place, keeping its project and custom regions, models and tool choices. No extra question is asked. An orchestrator the tool cannot confirm as its own goes through the normal conflict choice (`--conflict` in the CLI); if skipped, a TODO entry explains the manual step. No orchestrator is created when none is present, and its workflows are not changed.

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
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle files the tool cannot confirm as its own |
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
| `--conflict <skip\|overwrite\|backup>` | string | `skip` | How to handle files the tool cannot confirm as its own |
| `--dry-run` | bool | false | Plan without writing any files |
| `--yes` | bool | false | Auto-confirm the deployment plan |
| `--output <json>` | string | — | Machine-readable output format |

---

## Transform — Cross-Harness Conversion

Moves the whole deployment of a workspace (agents, skills and hook bundles) from one harness to another. The input is the workspace root, as for deploy.

```sh
# Preview the plan; nothing is written
./mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --workspace /path/to/my-project \
  --dry-run

# Execute the transformation, confirming the plan
./mosaic-deploy transform \
  --harness claude-code \
  --target-harness opencode \
  --workspace /path/to/my-project \
  --yes \
  --overwrite
```

Agents are read from the source harness's agents folder and written to the target's. Agent bodies, stamps, protocol and bundle regions, workflow tables, infrastructure declarations, and project and custom regions and skills are kept unchanged from the source and never upgraded. Tool names, frontmatter shape, role-dependent fields and the regions the harness fills in come from the target. Hooks are deployed from the target's catalog variant for the bundles present in the source, so injections and hooks may be newer than the source. Existing destinations are skipped unless `--overwrite` is given. Except between the two GHCP targets, source files and source manifest entries are never changed; the target deployment is recorded in the manifest, and `update` is the explicit way to upgrade it. Esc at a question cancels the run with nothing written. A model that cannot be resolved is left out of the agent (no `model` key) and named in the summary and the TODO file. Agents written into `vscode-ghcp` are named `X.agent.md`, like those of `ghcp-cli`. Between `opencode` and `opencode-v2` the target replaces the source in place; see [OpenCode V1 vs V2](#opencode-v1-vs-v2-opencode-v2-is-experimental).

MCP tool names are converted from the source harness's naming to the target's; no tool mapping is needed for that. A name that can be split into server and tool in more than one way is settled by the servers declared in the OpenCode configuration when the source is OpenCode; otherwise Transform asks which server provides it, once per name for the whole run. A name left unanswered is not written and is reported.

```sh
# Move an OpenCode V1 workspace to OpenCode V2 (experimental); swap the ids to go back
./mosaic-deploy transform \
  --harness opencode \
  --target-harness opencode-v2 \
  --workspace /path/to/my-project \
  --dry-run
```

### Transform between the GHCP targets

"GHCP CLI / VS Code GHCP Copilot" (`ghcp-cli`) and "VS Code GHCP Local" (`vscode-ghcp`) share `.github/agents` and `.github/hooks`, so Transform between them replaces the source deployment instead of adding the target beside it:

- Agents are rewritten in place as `.github/agents/X.agent.md` with the target's content. Legacy `X.md` agents recorded in the manifest are migrated to `X.agent.md`.
- Target hooks are written; source hook files and registration are replaced or removed.
- Skills are left as they are.
- Every removed or replaced file is backed up under `.mosaic/backups/` first.
- `--dry-run` and the confirmation plan list the removals and replacements; with `--output json` they appear as the `removals` and `replacements` arrays.
- If something fails, the workspace is left as it was (atomic on failure).

See `Tools/Deployment/docs/cli.md` for the JSON fields.

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

The source file is never modified. Every project region of the source is present in the generic with its tags, at any depth including inside managed regions, and its content is kept byte-identical (the default) or emptied (`--empty-project-regions`). Custom regions are removed at any depth and reported by name (`removed custom region: <name>` lines in the human output, `removedCustomRegions` in JSON). Managed regions are emptied, apart from project regions nested in them. The result is ready for the normal deploy pipeline, which accepts a managed region holding a nested project region with content and uses that content as the deploy-once default; content already deployed wins on update. Tier, tier rationale and required skills are optional and left out of the generic when empty; an agent without a tier gets its own model question at deploy time, and skills named in an agent's instructions are shipped only when declared in its required skills.

### Batch promote (TUI)

In the TUI, Promote asks for the harness, then the folder that holds the agents, then the catalog stack, then shows a multi-select list of the promotable agents. Agents whose key already exists in the stack are listed too; hand-written agents that are not promotable are not.

Asked **once per batch**:

- the target layer (stacks with two or more layers)
- whether to keep or empty project region content (Keep is pre-selected; asked only when a selected agent has project content, and the summary states the answer)
- a server name for each unmapped tool name
- the recommended tier, once per distinct source model (an agent without a model is asked individually)

Asked **per agent**:

- the category (mandatory, no skip)
- the tier rationale and the required skills (skip for this agent or for the rest of the batch)
- for an existing key, **replace or skip** (default skip); the question names the layer that holds the existing agent

Replacing keeps the existing numeric id and key, writes the agent into the chosen layer under the chosen category, and removes the old file in that layer when the category differs. Other layers are untouched: the new agent shadows one of the same key in another layer.

Keys: Ctrl+K skips the question for this agent, Ctrl+X skips it for the rest of the batch, Esc on any batch question aborts the batch. Agents already promoted stay promoted. A failing agent does not stop the batch; the summary lists every agent as promoted, skipped, failed or not processed. Source files are never changed.

Batch promote is TUI-only. The CLI `promote` command promotes a single file; it keeps project region content by default, `--empty-project-regions` empties it, and it never asks. `--overwrite` replaces the existing agent, keeps its numeric id and leaves exactly one agent with that key; without `--overwrite`, an existing key in the target layer is refused whatever category is chosen.

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

### Creating your own catalog

A catalog is a plain folder. The deploy tool never searches for catalogs: a folder is used only after you add it to the stack, either with `a` on the TUI stack screen or with `--catalog-folder` (see below). Once a stack is saved with a workspace, later runs for that workspace reuse it.

Minimal layout for a catalog that adds workflows and agents on top of the default catalog:

```
my-catalog/                        <- the folder you add to the stack
  catalog-identity.yaml            <- required
  Workflows/
    MyTeam/                        <- category folder (any name)
      my-workflow.md
  Subagents/
    MyTeam/                        <- category folder (any name)
      my-agent.md
```

```yaml
# catalog-identity.yaml
id: my-team          # lowercase letters, digits, '.', '_', '-'; unique in the stack
name: My Team Catalog
version: "1.0"
```

Everything else is optional. Add only the folders you need, at the catalog's top level:

| Folder or file | Holds |
|----------------|-------|
| `Workflows/<Category>/<id>.md` | Workflows (format: `Development/Designs/WorkflowFormat.md`) |
| `Subagents/<Category>/<key>.md` | Subagents |
| `UtilityAgents/<key>.md` | Utility agents |
| `StandaloneAgents/` | Standalone agents |
| `Skills/<key>/SKILL.md` | Skills, one folder each |
| `Hooks/<bundle>/hook.yaml` | Hook bundles, one folder each |
| `Orchestrator/orchestrator.md` + `orchestrator-script.md` | The orchestrator pair (always both files) |
| `CommunicationProtocol.md`, `DeployedSections.md` | System files |
| `HarnessInjections/<Harness>/` | Harness injections |

Agent and skill files follow `Catalog/CatalogFilesFormat.md`. Copying a file from the default catalog and editing it is the quickest start.

**With or without the default layer.** Keep the default layer in the stack and your catalog needs nothing beyond the items it adds or replaces. Leave it out and your catalogs must supply everything a deployment needs: the orchestrator pair, both system files, the injections of the harness you deploy to, and every agent, skill and hook your workflows use.

**Common reasons an item is not picked up.** None of these produces an error or a warning:

- The folder was never added to the stack, or a stack saved earlier with the workspace is being reused without it. Check the stack screen.
- The folder you added is not the one that holds `catalog-identity.yaml` and the content folders. For example, you added a parent folder, or the content sits one level lower in a nested `Catalog/` folder. A catalog whose content is nested loads with no items.
- A workflow file sits directly in `Workflows/` or two folder levels below it. Only `Workflows/<Category>/<file>.md` is loaded. The same one-level rule applies to `Subagents/<Category>/`.
- The file name starts with `_` or does not end in `.md`. These files are ignored on purpose.
- A higher layer supplies the same workflow id or agent key and shadows your item. The conflict report on the plan review screen shows this.

`Workflows/Index.md` is optional and does not affect loading. To compare it with the workflow files, run `check-index`.

A file in your catalog that cannot be read or parsed (for example an agent without frontmatter) stops the whole stack with a "layer failed to load" error that lists the offending files. See [Stack errors](#stack-errors).

### Setting up a stack in the TUI

In the interactive TUI, a **stack screen** is the last entry screen in every mode (deploy, update, change workflows, deploy agents, deploy hooks, transform, promote), after the path and harness screens. It lists the layers highest first, with the default layer always shown as the bottom row.

| Key | Action |
|-----|--------|
| `a` | Add a catalog folder by path |
| `d` | Remove the user layer under the cursor |
| Shift+Up / Shift+Down | Move the user layer under the cursor up or down in precedence |
| Space (on the default row) | Keep the default layer in the stack, or leave it out |
| Enter | Confirm the stack |
| Esc | Go back to the previous entry screen, keeping your choices |

Enter is refused while the stack would have no layer at all. A problem with the confirmed stack (for example a layer without identity, a duplicate catalog id, or a harness whose injections no layer supplies) is shown on the screen, and you stay there to fix it.

The screen starts from:

1. the stack persisted with the workspace you just entered (see [Persistence](#persistence-on-update-and-redeploy)): in deploy, update, workflows, agents and hooks modes when the path is itself a workspace; in promote mode the workspace that contains the file, and in transform mode the workspace you entered, otherwise
2. the default layer alone.

The TUI launch takes no flags, so add user layers with `a` on this screen.

If the persisted stack can no longer be used (for example a layer folder moved), the screen shows the reason and you must change the stack before confirming.

**Migration notice.** When the TUI starts with a user catalog, the stack screen shows this notice: *the behaviour of custom catalogues has changed. Until now a custom catalogue got no default agents or workflows. Your folder is now a layer above the default catalogue, which is offered as the bottom row. Keep it to include the default agents and workflows (space on the default row); leave it out to use only your own catalogues.* See [Migrating from `--catalog-folder`](#migrating-from---catalog-folder).

### Setting up a stack on the command line

Two global flags, accepted by every subcommand (the TUI launch takes no flags):

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
info: orchestrator-pair from team-catalog 1.2 shadows mosaic-default 0.7.0; affects workflows: feature-dev, brownfield-tdd
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

- **Manifest** (`.mosaic/manifest.yaml`): every entry (agents including both orchestrator files, skills, hooks) carries a `catalog` field with the supplying catalog's id, name and version. Each harness section records `protocol_source` and `bundle_source` (written by deploy and update, kept by the partial runs), and the top level records the persisted `catalog_stack`.
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
| `transform` | `--workspace` |
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

- An agent whose key already resolves in the stack is promoted into the layer that supplies it. In a batch, the replace-or-skip question names that layer.
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

The tool keeps its files in a `MosaicDeploy/` directory next to the binary. A run creates `logs/` there. Config is optional: a missing `user-config.yaml` or `tool-config.yaml` is created by the tool itself from the commented templates (`task install` seeds them too), and existing files are never overwritten. External harnesses, if any, go in `harnesses/`. Two config files can live in `MosaicDeploy/config/`:

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

> **Note:** The tool rewrites this file when it saves your choices. Comments and two-space indentation survive saves.

### Where the catalog stack is kept

Neither config file holds the catalog stack. It is stored per workspace in `.mosaic/manifest.yaml` (`catalog_stack`), written by the run that was given stack input and reused by later runs for that workspace. Each catalog's own settings (id, name, version, declared overrides) live in `catalog-identity.yaml` at the catalog root. See [Catalog Stacks](#catalog-stacks).

---

## The TODO File

After every deploy or update, the tool writes `MOSAIC-DEPLOYMENT-TODO-<timestamp>.md` to the workspace root. It lists:

- **Unfilled project regions** — slots you should fill for best agent performance
- **Skipped files** — files the tool could not confirm as its own, left unchanged
- **Unmapped tools** — generic tools without harness equivalents
- **Hook registration steps** — manual steps to complete

Deploy and Update also print two warnings on the console: orchestrator content with no region to go into, and a region a role may not carry.

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
| "harness not found" | Check the ID (`claude-code`, `opencode`, `ghcp-cli`, `vscode-ghcp`, `codex` (experimental), `opencode-v2` (experimental)). For custom descriptors, pass the full file path. |
| "workspace already holds the OpenCode target ..." | A workspace holds one OpenCode target (`opencode` or `opencode-v2`). Nothing was changed. Use the Transform command the message prints to move the workspace to the other target. See [OpenCode V1 vs V2](#opencode-v1-vs-v2-opencode-v2-is-experimental). |
| "workspace ... holds both OpenCode targets" | Remove the agent, skill and plugin files of one target from `.opencode/` by hand, then run again. |
| An `opencode-v2` agent cannot use my ask-the-user MCP tool | V2 denies every tool an agent does not declare. Add a `tool_destinations` mapping for `opencode-v2` and declare it in the agent's tools; see [OpenCode V1 vs V2](#opencode-v1-vs-v2-opencode-v2-is-experimental). |
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
