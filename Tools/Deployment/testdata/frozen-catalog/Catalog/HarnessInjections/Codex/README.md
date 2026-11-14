# Frozen Codex Injection Content

This directory exists so tests can construct a usable Codex harness module over the
frozen-catalog root. Without it, every test that calls `codex.New(opts)` with an
`opts.ContentDir` resolver pointing at the frozen catalog would fail with "failed to
initialise built-in" because the shared injection-file loader requires both files to be
present.

## What this is

Frozen fixture content for the deployment tool's test suite. It is NOT the live
catalog content for the Codex harness. This directory is a stable, minimal snapshot that
gives the injection-file loader something valid to read.

## What this is not

This is not the authoritative Codex harness content. Do NOT update these files in step
with the live `Catalog/HarnessInjections/Codex/` tree. Their purpose is stability.

## Relationship to other test fixtures

- `internal/harness/builtin/codex/testdata/` -- module-local fixtures for the codex
  package's own unit tests. These have specific content matched to constants in
  `codex_test.go`. Do not consolidate them with this directory.
- `testdata/frozen-catalog/Catalog/HarnessInjections/*/` (other harnesses) -- sibling
  directories for the other built-in harnesses, used by their own regression tests.
