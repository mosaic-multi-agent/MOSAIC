# Harness Knowledge

Knowledge about the AI coding harnesses (platforms) that MOSAIC has been deployed on. The orchestration system itself is **harness-agnostic** — this folder documents the harnesses we explicitly support with tooling, captures, and issue tracking.

## Tracked Harnesses

| Harness | Folder Name | GitHub Repository | ID Prefix |
|---------|-------------|-------------------|-----------|
| **Claude Code** | `ClaudeCode` | `anthropics/claude-code` | CC- |
| **GitHub Copilot CLI** | `GHCP-CLI` | `github/copilot-cli` | GC- |
| **OpenCode 1.x** | `OpenCode` (captures) / `OpenCodeV1` (known issues) | `anomalyco/opencode` | OC- |
| **OpenCode 2.x** | `OpenCodeV2` | `anomalyco/opencode` | O2- |
| **VS Code GitHub Copilot** | `VsCodeGHCP` | `microsoft/vscode` | VC- |
| **Codex CLI** | `Codex` | `openai/codex` | CX- |

The **Folder Name** column is the canonical identifier used in all subdirectories under `HarnessKnowledge/`. Use these exact names when creating harness-specific folders here.

> **Note on OpenCode versions:** In `KnownIssues/`, the OpenCode folder is `OpenCodeV1` and covers **OpenCode 1.x only**. OpenCode 2.x is substantially different and is tracked as a separate harness in `KnownIssues/OpenCodeV2` (ID prefix `O2-`), never mixed into the v1 entries. Both lines share one GitHub repo and tracker, so check the reported version on every issue. `SystemPromptCapture/OpenCode` is v1 only; v2 captures live in `SystemPromptCapture/OpenCodeV2`.

> **Note on OpenCode:** Previously published as `sst/opencode` and sometimes referenced as `opencode-ai/opencode`. Only `anomalyco/opencode` is current.

## Directory Layout

```
HarnessKnowledge/
├── README.md                          # This file — canonical harness list
├── SystemPromptCapture/               # Harness system prompt documentation
│   ├── CaptureGuide.md                # Agent-facing specs and prompts
│   ├── CrossHarnessComparison.md      # Side-by-side comparison across all harnesses
│   ├── TestArtifacts/                 # Shared test files for captures
│   └── {Harness}/                     # Per-harness captures (Subagent + PrimaryAgent)
└── KnownIssues/                       # Harness issue knowledge base (bugs, limitations, quirks)
    ├── CaptureGuide.md                # Agent-facing format specs
    ├── CrossHarnessComparison.md      # Side-by-side comparison across all harnesses (mode-split rankings)
    ├── GhcpCrossHarness.md            # GHCP CLI, VS Code Copilot and VS Code Local in one workspace
    └── {Harness}/                     # Per-harness issue tracking
        ├── index.md                   # Quick-reference index
        ├── active-issues.md           # Currently active issues
        └── resolved-issues.md         # Issues that were active and later resolved
```

## Who Uses This

- **Harness Injection authors** — the primary consumer. Issue knowledge and system prompt captures inform what goes into `Catalog/HarnessInjections/{Harness}/` — the harness-specific constraints and workarounds injected into deployed agents
- **Harness Issue Hunter** (`Catalog/UtilityAgents/harness-issue-hunter.md`) — maintains KnownIssues
- **System Prompt Capture agents** — populate SystemPromptCapture
