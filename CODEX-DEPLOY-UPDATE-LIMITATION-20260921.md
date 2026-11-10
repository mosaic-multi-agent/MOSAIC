# Codex Deploy-Flow Update Limitation

**Created:** 2026-09-21
**Status:** Resolved.

## Original problem

The Codex encoder requires the prior deployed file bytes when performing an update
(OpUpdate). The Deploy workspace and Deploy agents flows did not supply prior bytes,
so a redeploy after a source version bump (items classified as ActionUpdate) failed
with ErrMissingPriorBytes.

## Resolution

Every flow now obtains the deployed bytes of a plan item through one rule,
`deployedBytes.funnelReader` in `Tools/Deployment/internal/app/deployed_bytes.go`:

- Deploy (`deploy.go`), Deploy agents (`deployagents_service.go`), Update (`update.go`)
  and Update workflows (`workflow_update.go`) all build their content callback from
  `funnelReader`.
- For each plan item it reads the existing file through the decode funnel
  (`deployed_read.go`) and derives the operation from the plan item action. The
  encoder receives the canonical bytes plus the raw on-disk bytes, so user-owned
  keys in a Codex TOML file survive an update.
- A missing file is a create and needs no prior bytes.

Covered by `codex_deploy_existing_test.go` (version-bump update over an existing Codex
file in Deploy and Deploy agents, user key preserved) and `codex_deploy_op_test.go`
(create, unchanged and hooks cases).

## Behaviour note

Because prior bytes now reach the content step for all harnesses, Markdown harnesses
also keep user-edited regions when Deploy rewrites an existing file.
