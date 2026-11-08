# Codex Deploy-Flow Update Limitation

**Created:** 2026-09-21
**Status:** Open -- accepted as a known limitation for now, to be resolved separately.

## Problem

The Codex encoder requires prior deployed file bytes when performing an update
(OpUpdate). The Deploy workspace and Deploy agents flows do not supply prior file
bytes -- they always regenerate from source with no deployed reader.

For existing Markdown harnesses this is harmless (Markdown ignores the Op field).
For Codex, this means a redeploy via the Deploy flow after a source version bump
(where items are classified as ActionUpdate) will fail with ErrMissingPriorBytes.

## What Works

- First deploy (all items ActionCreate): works after the Op derivation fix.
- Idempotent second deploy (all items ActionUnchanged): works -- executor skips
  content rendering entirely, Codex encoder is never called.
- Update flow (update.go, workflow_update.go): works -- these flows thread prior
  bytes via a deployedReader closure.

## What Fails

- Deploy workspace or Deploy agents flow when items are classified as ActionUpdate
  (e.g. after a source version bump): Codex encoder rejects OpUpdate with nil
  prior bytes (ErrMissingPriorBytes).

## Options Considered

(a) Accept the limitation (chosen for now) -- first deploy and idempotent redeploy
    work; redeploy-after-bump must use the Update flow.

(b) Thread prior bytes into Deploy flows for all harnesses -- fixes Codex but
    changes Markdown behavior (starts preserving user-edited injection regions on
    Deploy rewrites, which current Deploy flows do not do). May be the right long-
    term fix, but needs careful consideration of whether Deploy should preserve
    user edits (current behavior is arguably strange).

(c) Thread prior bytes only for Codex -- harness-specific branching, invents
    behavior not present for other harnesses.

(d) Refuse Deploy when a prior deployment already exists -- force users to use the
    Update flow for subsequent deployments. Simplest semantically but changes the
    Deploy UX.

## Context

- Codex encoder rules: agentformat/codextoml/encode.go:56-73
- Deploy flow buildContent call: app/deploy.go:312, app/deployagents_service.go:357
- Update flow deployedReader: app/update.go:444-456
- Transform Op contract: transform/types.go:103-115
