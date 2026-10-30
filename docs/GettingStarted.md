# Getting Started

Your first MOSAIC run in about 15 minutes — from cloning to watching agents orchestrate across your codebase.

This guide walks you through the **golden path**: one minimal deployment, one zero-risk workflow run, then guidance on going deeper. It deliberately skips depth — the specialized guides linked at the end cover everything in detail.

---

## Prerequisites

You need an AI coding tool already installed, configured, and working with model access:

- **Claude Code**, **OpenCode**, **VS Code GitHub Copilot**, or **GitHub Copilot CLI**

MOSAIC deploys agents *into* these tools — it doesn't replace them. If you can already have a working conversation with your AI coding tool, you're ready.

---

## 1. Clone and Unpack Tools

Clone the repository:

```sh
git clone https://github.com/tomasgurtler21/MOSAIC.git
cd MOSAIC
```

Download the latest release archive for your platform from [Releases](https://github.com/tomasgurtler21/MOSAIC/releases) and unpack it into the repository root:

| Platform | Archive |
|----------|---------|
| Linux x64 | `mosaic-linux-amd64.tar.gz` |
| Windows x64 | `mosaic-windows-amd64.zip` |

After unpacking you should have `mosaic-deploy`, `mosaic-run`, and `mosaic-log-analyzer` in the repository root (`.exe` on Windows).

---

## 2. Meet mosaic-helper

Before you deploy anything, you already have a guide available. The `mosaic-helper` agent ships with the repository — it's pre-indexed for every harness at clone time (e.g. `.claude/agents/mosaic-helper.md`, `.github/agents/mosaic-helper.md`).

Open it in your AI coding tool and ask it anything:

- "What workflows are available?"
- "Walk me through deployment"
- "What does HITL mean in a workflow table?"

It's a lightweight agent — designed for a small, cheap model — that reads the docs for you and points you to the right resources. You can use it alongside this guide or instead of it.

> **Model configuration:** The pre-indexed `mosaic-helper` ships with a default model ID that works out of the box on some harnesses (e.g. Claude Code). On others (e.g. OpenCode), you may need to edit the model ID in the agent file to match a model available in your setup. The same applies to all deployed agents — `mosaic-deploy` lets you assign models during deployment, but if the defaults don't match your environment, update them.

---

## 3. Deploy to Your Project

Run the deploy tool **from the MOSAIC repository root**:

```sh
.\mosaic-deploy.exe
```

On Linux: `./mosaic-deploy`

The interactive TUI walks you through:

1. **Pick a harness** — which AI coding tool you use (Claude Code, OpenCode, VS Code GHCP, or GHCP CLI)
2. **Pick workflows** — select **only `kb-generation`** for now (you'll add more later)
3. **Skip utility agents** — you don't need them for your first run
4. **Assign models** — the TUI asks you to map each model tier (HIGH, MEDIUM-HIGH, etc.) to a specific model ID available in your setup. If you have one strong model and one cheaper model, use the strong one for the upper tiers and the cheaper one for the lower tiers. If you only have one model available, use it for everything — you can always redeploy with different assignments later.
5. **Set target** — point the tool at **your project workspace** (the codebase you want agents to work on). This is where agent files get written — not the MOSAIC repo itself.
6. **Confirm** — the tool writes all agent files to your project workspace

> For CLI/scriptable usage, see the [Deployment Guide](DeploymentGuide.md).

---

## 4. Run Your First Workflow

Open your AI coding tool **in your project workspace** (where you deployed to) and start the orchestrator agent. Tell it which workflow and what task:

```
Use kb-generation workflow. Task: Generate knowledge base for this codebase.
Checkpoints disabled.
```

The orchestrator creates a run folder (`Orchestration-{run_id}/`), sets up tracking in `Orchestration.md`, and begins dispatching agents through the workflow.

> Add `Checkpoints disabled.` to your first run prompts. You can enable them later — see the [Orchestration Guide](OrchestrationGuide.md#checkpoints).

### Why kb-generation first?

- **Read-only** — it scans your codebase and produces documentation. Zero risk, no code changes.
- **No injections needed** — agents discover the codebase by reading it, so you don't need to teach them anything about your project upfront.
- **Fully autonomous** — all HITL (human-in-the-loop) checkpoints are off by default, so the workflow runs end-to-end without needing you to configure agent-to-user interaction tools.
- **Immediately useful** — the generated `KnowledgeBase.md` makes every subsequent workflow noticeably better, because research agents actively look for an existing KB.

### What success looks like

The orchestrator dispatches agents one by one through the workflow. You'll see each dispatch and response in your AI tool's conversation. Expect a research agent to scan your codebase and produce knowledge base artifacts in the run folder — typically 5-15 minutes depending on codebase size.

When the workflow completes, the orchestrator reports a summary and the run folder contains all artifacts and the full `Orchestration.md` audit trail. Open it to see exactly what happened at each step.

> **Something seem off?** Skim `HarnessKnowledge/KnownIssues/{your-harness}/index.md` — these document real quirks and limitations of each AI platform that can affect orchestration. A two-minute skim can save you from debugging a harness problem you'd otherwise mistake for a MOSAIC problem.

---

## 5. Go Deeper — Injections and Real Work

You've seen the system work. Now make it work *well* for your specific codebase.

### Add a workflow and fill project injections

Now add the `brownfield-tdd` workflow to your existing deployment. Run the deploy tool and choose **Update workflows** — this rewrites only the orchestrator with your new workflow set, without disturbing agents or injections you've already configured:

```sh
.\mosaic-deploy.exe
# → Select "Update workflows"
# → Select both kb-generation and brownfield-tdd
```

If the new workflow requires agents that aren't deployed yet, the tool deploys them automatically in the same run.

Next, add the `injections-helper` utility agent. Run the deploy tool again and choose **Deploy agents**:

```sh
.\mosaic-deploy.exe
# → Select "Deploy agents"
# → Find injections-helper under the Utility Agents category
```

After deployment, you'll find a `MOSAIC-DEPLOYMENT-TODO-<timestamp>.md` in your workspace root. This checklist tells you exactly what needs attention — most importantly, **project injection points** to fill.

Project injections are where you teach the agents about *your* codebase: conventions, folder structure, build commands, test patterns. Filling these is the single highest-impact thing you can do for agent quality.

**Use the `injections-helper` agent** to fill them collaboratively — it walks you through each region, asks about your project, and writes the content. This is much easier than editing injection points by hand. The [Agent Customization Guide](AgentCustomizationGuide.md) covers the full details of what each region type means.

For example, a project injection region starts empty and gets filled with your specifics:

```markdown
<!-- Before -->
<CodebaseContext type="project">
</CodebaseContext>

<!-- After (filled by injections-helper or by hand) -->
<CodebaseContext type="project">
Python 3.12, pytest. Source in src/, tests mirror source tree in tests/.
Build: `py -m pytest`. CI runs on GitHub Actions.
</CodebaseContext>
```

### Run brownfield-tdd

Pick a small, well-scoped feature for your first real run. This is the main workhorse workflow — it takes a requirement, researches the codebase, plans the implementation with TDD, and executes it. With injections filled, agents understand your conventions and produce code that fits your codebase.

Unlike kb-generation, brownfield-tdd uses **approval gates** — HITL (human-in-the-loop) steps where agents pause for your approval at key points (e.g. before starting implementation). These are different from orchestrator checkpoints (which pause between workflow stages). Approval gates require a **user interaction tool** that subagents can call. Not every harness provides this automatically — for example, Claude Code's `ask_user_questions` built-in tool is only available to the primary agent, not to subagents dispatched via the task tool. If you hit this, check `HarnessKnowledge/KnownIssues/{your-harness}/` for harness-specific setup guidance.

For your first brownfield-tdd run, disable orchestrator checkpoints to keep things simple:

```
Use brownfield-tdd workflow. Task: <your feature description>.
Checkpoints disabled.
```

> **Runner mode** (`mosaic-run`) automates the orchestrator's mechanical work and is more cost-efficient for longer workflows. Try it once you're comfortable with how native mode works. See the [Runner Guide](RunnerGuide.md).

---

## Token Usage on Copilot Harnesses

GHCP CLI and VS Code Copilot Chat report token usage through Copilot's OpenTelemetry file export, which the MOSAIC logging hook reads. To get usage and cost in `mosaic-log-analyzer` for these harnesses:

1. Set the environment variable `COPILOT_OTEL_FILE_EXPORTER_PATH` to the path of a `.jsonl` file. This one variable enables both Copilot's export and MOSAIC's ingest. Do not use the VS Code user setting for the export instead: the hooks cannot read it.
2. Set it at user level (Windows user environment variables; macOS/Linux login environment), not in a single shell, and restart VS Code afterwards. VS Code started from the desktop sees only user-level variables.
3. Content capture is optional and only makes the file larger. MOSAIC never reads content and never modifies the file.
4. Cost needs `github-copilot` entries that you maintain in `pricing.yaml`; the shipped file has none.

If usage is missing from a run (the variable was unset, the file was missing, a session crashed, or the hooks did not see the variable), recover it with `mosaic-log-analyzer ingest-otel <otel-file> --run <run-id>`. The [LogAnalyzer Guide](LogAnalyzerGuide.md#token-usage-recovery-ingest-otel) explains the recovery, the views, the pricing and settings files, and the `currency` key. Hook-side details are in the hook README (`Catalog/Hooks/mosaic-logger/README.md`).

---

## Where to Go from Here

You've deployed, run workflows, and seen the orchestration in action. Here's what to explore next:

| Goal | Resource |
|------|----------|
| Understand deployment operations in depth | [Deployment Guide](DeploymentGuide.md) |
| Learn run configuration, checkpoints, parallel runs | [Orchestration Guide](OrchestrationGuide.md) |
| Automate workflows with the Runner | [Runner Guide](RunnerGuide.md) |
| Read token, cost, context and compaction views; configure pricing | [LogAnalyzer Guide](LogAnalyzerGuide.md) |
| Customize agents for your codebase | [Agent Customization Guide](AgentCustomizationGuide.md) |
| Browse available workflows | `Catalog/Workflows/Index.md` |
| Create your own subagents or workflows | Open your harness in the MOSAIC repository root (not your project workspace) and use `subagent-creator` to design agents or `workflow-creator` to author workflows — they ship there ready to use. Then deploy the result with `mosaic-deploy` |
| Understand the full architecture | `README.md` at the repository root |
