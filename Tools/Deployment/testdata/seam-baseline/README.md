# Seam Baseline

This directory holds the pre-seam regression baseline for all four existing harnesses
(claude-code, ghcp-cli, opencode, vscode-ghcp), captured before any pipeline code
changes.

## What it is

The seam stage inserts a decode step into the app layer's read funnel. After decode
is inserted, every deployed file read for an update goes through decode before the
transform sees it. For the four Markdown harnesses the decode step is the identity, so
the output must be byte-identical to what the unmodified pipeline produced. This
directory holds that "unmodified pipeline" record.

## What it contains

For each harness, two path types are captured:

- `create/` - output for an agent with no prior deployed file (nil Deployed in the
  transform request). One file per fixture agent.
- `update/` - output for an agent that does have a prior deployed file. Each update
  case stores two files:
    - `<agent>.output.md` - the transform output produced when Deployed = prior bytes
    - `<agent>.prior.md` - the prior deployed bytes used as input (Deployed field)

The update-path fixture demonstrates the full "read prior -> transform" cycle that
the seam stage inserts decode into. Byte-identity of `.output.md` after the seam
proves that decode is the identity for Markdown content.

## How it is used

The committed driver at `Tools/Deployment/internal/app/seam_baseline_test.go` both
writes this directory (with -update flag) and replays it (default mode). The seam
stage runs the driver in replay mode after inserting decode; a passing run proves
byte-identity.

## This directory is not regenerated to make a comparison pass

Once committed, the files in this directory are the pre-seam record. Regenerating them
to make a failing comparison pass defeats the purpose.

The driver's -update flag may be used only for the initial capture, for any extension
that adds new fixture cases, and for the single justified exception below. It must
never be used to "fix" a comparison whose diff is not fully explained by a recorded
intentional output change.

## Justified re-baselines

A re-baseline is allowed only when every hunk of the diff is traced to an intentional
output change and the cause is recorded here. Re-baseline per harness and file, after
classifying the diff.

### Frozen CommunicationProtocol rewrite (integration merge)

- Files: all 24 files under `claude-code/`, `ghcp-cli/`, `opencode/` and `vscode-ghcp/`
  (`create/{contracts-review,orchestrator,planner-tdd-soft,test-runner}.md`,
  `update/test-runner.output.md`, `update/test-runner.prior.md`).
- Diff kind: only the content of the deployed `<CommunicationProtocol type="managed">`
  region changed: the version attribute `1.11` -> `1.12` and the region body. Outside
  that region every file is byte-identical to the previous baseline (no tools, frontmatter,
  injection region, line ending or BOM change). The subagent fixtures carry the frozen
  catalog's Subagent section and the orchestrator fixture carries its Orchestrator
  section, verified line for line against `testdata/frozen-catalog/Catalog/CommunicationProtocol.md`,
  and the region is identical across all four harnesses.
- Integration cause: `testdata/frozen-catalog/Catalog/CommunicationProtocol.md` was moved
  from `Development/Designs/` to `Catalog/` and rewritten (frozen protocol version now
  `1.12`), commit `b5fcaf88` ("Move deployed CommunicatonProtocol to Catalog ..."). The
  `.prior.md` files are the create output of the same fixture and change with it.
- Requirements: R-1, R-7 (intentional integration output change).
